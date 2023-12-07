package pkg

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/io"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager"
	lidario "github.com/mfbonfigli/gocesiumtiler/third_party/lasread"
	"github.com/mfbonfigli/gocesiumtiler/tools"
)

type TilerIndex struct {
	fileFinder       tools.FileFinder
	algorithmManager algorithm_manager.AlgorithmManager
}

func NewTiler(fileFinder tools.FileFinder, algorithmManager algorithm_manager.AlgorithmManager) tiler.ITiler {
	return &TilerIndex{
		fileFinder:       fileFinder,
		algorithmManager: algorithmManager,
	}
}

// Starts the tiling process
func (tilerIndex *TilerIndex) RunTiler(opts *tiler.TilerOptions) error {
	log.Println("Preparing list of files to process...")

	// Prepare list of files to process
	lasFiles := tilerIndex.fileFinder.GetLasFilesToProcess(opts)
	log.Println("las_file list", lasFiles)
	for i, filePath := range lasFiles {
		log.Printf("las_file path %d [%s]", i+1, filePath)
	}

	// load las points in octree buffer
	for i, filePath := range lasFiles {
		// Define point_loader strategy
		var tree = tilerIndex.algorithmManager.GetTreeAlgorithm()
		tools.LogOutput("Processing file " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(lasFiles)))
		tilerIndex.processLasFile(filePath, opts, tree)

		// tree.Clear()
	}
	tilerIndex.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()

	return nil
}

func (tilerIndex *TilerIndex) processLasFile(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) {
	// Create empty octree
	lasFileLoader, err := tilerIndex.readLasData(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = lasFileLoader.LasFile.Clear()
		_ = lasFileLoader.LasFile.Close()
		// lasFileLoader.LasFile = nil
		// lasFileLoader.Tree = nil
	}()

	tilerIndex.prepareDataStructure(tree, opts)

	subfolder := fmt.Sprintf("%s%s", tools.ChunkTilesetFilePrefix, getFilenameWithoutExtension(filePath))
	tilerIndex.exportToCesiumTileset(tree, opts, subfolder)

	tilerIndex.exportRootNodeLas(tree, opts, subfolder, lasFileLoader.LasFile)

	tools.LogOutput("> done processing", filepath.Base(filePath))
}

func (tilerIndex *TilerIndex) readLasData(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) (*lidario.LasFileLoader, error) {
	// Reading files
	tools.LogOutput("> reading data from las file...", filepath.Base(filePath))
	lasFileLoader, err := readLas(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	lasFile := lasFileLoader.LasFile

	edgeX := lasFile.Header.MaxX - lasFile.Header.MinX
	edgeY := lasFile.Header.MaxY - lasFile.Header.MinY
	edgeZ := lasFile.Header.MaxZ - lasFile.Header.MinZ
	useEdgeCalculateGeometricError := opts.TilerIndexOptions.UseEdgeCalculateGeometricError

	tree.UpdateExtendChunkEdge(edgeX, edgeY, edgeZ, useEdgeCalculateGeometricError)

	return lasFileLoader, nil
}

func (tilerIndex *TilerIndex) prepareDataStructure(octree *grid_tree.GridTree, opts *tiler.TilerOptions) {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")

	if err := octree.Build(); err != nil {
		log.Fatal(err)
	}

	if opts.MaxNumPointsPerNode < 8*opts.MinNumPointsPerNode {
		err := fmt.Errorf("MaxNumPoints shoud be greater than 8 * MinNumPoints")
		log.Fatal(err)
	}

	log.Println("split-big-node for tree...")
	if err := octree.SplitBigNode(opts.MaxNumPointsPerNode); err != nil {
		log.Fatal(err)
	}
	log.Println("split-big-node for tree finished")

	log.Println("merge-small-node for tree...")
	if err := octree.MergeSmallNode(opts.MinNumPointsPerNode); err != nil {
		log.Fatal(err)
	}
	log.Println("merge-small-node for tree finished")

	rootNode := octree.GetRootNode()
	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints(), ", points.len:", len(rootNode.GetPoints()))

}

func (tilerIndex *TilerIndex) exportToCesiumTileset(octree *grid_tree.GridTree, opts *tiler.TilerOptions, subfolder string) {
	tools.LogOutput("> exporting data...")
	err := tilerIndex.exportTreeAsTileset(opts, octree, subfolder)
	if err != nil {
		log.Fatal(err)
	}
}

func getFilenameWithoutExtension(filePath string) string {
	nameWext := filepath.Base(filePath)
	extension := filepath.Ext(nameWext)
	return nameWext[0 : len(nameWext)-len(extension)]
}

// Reads the given las file and preloads data in a list of Point
func readLas(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) (*lidario.LasFileLoader, error) {
	var lasFileLoader = lidario.NewLasFileLoader(tree)
	_, err := lasFileLoader.LoadLasFile(filePath, opts.Srid, opts.EightBitColors)
	if err != nil {
		return nil, err
	}
	// defer func() { _ = lf.Close() }()

	return lasFileLoader, nil
}

// Exports the data cloud represented by the given built octree into 3D tiles data structure according to the options
// specified in the TilerOptions instance
func (tilerIndex *TilerIndex) exportTreeAsTileset(opts *tiler.TilerOptions, octree *grid_tree.GridTree, subfolder string) error {
	// if octree is not built, exit
	if !octree.IsBuilt() {
		return errors.New("octree not built, data structure not initialized")
	}

	// a consumer goroutine per CPU
	numConsumers := runtime.NumCPU()

	// init channel where to submit work with a buffer 5 times greater than the number of consumer
	workChannel := make(chan *io.WorkUnit, numConsumers*5)

	// init channel where consumers can eventually submit errors that prevented them to finish the job
	errorChannel := make(chan error)

	var waitGroup sync.WaitGroup

	// add producer to waitgroup and launch producer goroutine
	waitGroup.Add(1)

	producer := io.NewStandardProducer(opts.TilerIndexOptions.Output, subfolder, opts)
	go producer.Produce(workChannel, &waitGroup, octree.GetRootNode())

	// add consumers to waitgroup and launch them
	for i := 0; i < numConsumers; i++ {
		waitGroup.Add(1)
		consumer := io.NewStandardConsumer(tilerIndex.algorithmManager.GetCoordinateConverterAlgorithm(), opts.RefineMode)
		go consumer.Consume(workChannel, errorChannel, &waitGroup)
	}

	// wait for producers and consumers to finish
	waitGroup.Wait()

	// close error chan
	close(errorChannel)

	// find if there are errors in the error channel buffer
	withErrors := false
	for err := range errorChannel {
		log.Println(err)
		withErrors = true
	}
	if withErrors {
		return errors.New("errors raised during execution. Check console output for details")
	}

	return nil
}

func (tilerIndex *TilerIndex) exportRootNodeLas(octree *grid_tree.GridTree, opts *tiler.TilerOptions, subfolder string, lasFile *lidario.LasFile) error {
	parentFolder := path.Join(opts.TilerIndexOptions.Output, subfolder)

	var err error

	// var lf *lidario.LasFile
	// lf, err = lidario.NewLasFile(filePath, "r")
	// if err != nil {
	// 	log.Println(err)
	// 	log.Fatal(err)
	// }
	// defer lf.Close()

	newFileName := path.Join(parentFolder, "content.las")
	if _, err := os.Stat(newFileName); err == nil {
		if err := os.Remove(newFileName); err != nil {
			log.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	newLf, err := lidario.InitializeUsingFile(newFileName, lasFile)
	if err != nil {
		log.Println(err)
		log.Fatal(err)
	}
	defer func() {
		if newLf != nil {
			newLf.Close()
			newLf = nil
		}
	}()

	if err := newLf.CopyHeaderXYZ(lasFile.Header); err != nil {
		log.Println(err)
		log.Fatal(err)
	}

	progress := 0
	oldProgress := -1

	rootNode := octree.GetRootNode()
	numberOfPoints := rootNode.NumberOfPoints()
	points := rootNode.GetPoints()

	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints(), ", points.len:", len(points))

	for i := 0; i < int(numberOfPoints); i++ {
		point := points[i]

		pointLas, err := lasFile.LasPoint(point.PointExtend.LasPointIndex)
		if err != nil {
			log.Println(err)
			log.Fatal(err)
			return err
		}

		X, Y, Z := pointLas.PointData().X, pointLas.PointData().Y, pointLas.PointData().Z
		if !lasFile.CheckPointXYZInvalid(X, Y, Z) {
			log.Printf(" nonono invalid point_pos:[%d] X:[%f] Y:[%f] Z:[%f]", i, X, Y, Z)
			log.Fatal("invalid point X/Y/Z")
			continue
		}

		newLf.AddLasPoint(pointLas)

		// print export-progress
		progress = int(100.0 * float64(i+1) / float64(numberOfPoints))
		if progress != oldProgress {
			oldProgress = progress
			if progress%50 == 0 {
				log.Printf("export root-node progress: %v\n", progress)
			}
		}
	}

	newLf.Close()
	newLf = nil

	log.Println("Write las file success.", newFileName)

	// // Check
	// log.Printf("check las_file %s", newFileName)
	// mergedLf, err := lidario.NewLasFile(newFileName, "r")
	// if err != nil {
	// 	log.Println(err)
	// 	log.Fatal(err)
	// 	return err
	// }
	// defer mergedLf.Close()

	return nil
}
