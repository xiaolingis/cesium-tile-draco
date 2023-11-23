package pkg

import (
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/geometry"
	"github.com/mfbonfigli/gocesiumtiler/internal/io"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager"
	lidario "github.com/mfbonfigli/gocesiumtiler/third_party/lasread"
	"github.com/mfbonfigli/gocesiumtiler/tools"
)

type TilerMerge struct {
	fileFinder       tools.FileFinder
	algorithmManager algorithm_manager.AlgorithmManager
}

func NewTilerMerge(fileFinder tools.FileFinder, algorithmManager algorithm_manager.AlgorithmManager) ITiler {
	return &TilerMerge{
		fileFinder:       fileFinder,
		algorithmManager: algorithmManager,
	}
}

func (tiler *TilerMerge) RunTiler(opts *tiler.TilerOptions) error {
	log.Println("Preparing list of files to process...")

	// Prepare list of files to process
	lasFilePathList := tiler.fileFinder.GetLasFilesToMerge(opts)
	log.Println("las_file list", lasFilePathList)
	for i, filePath := range lasFilePathList {
		log.Printf("las_file path %d [%s]", i, filePath)
	}

	tree := tiler.algorithmManager.GetTreeAlgorithm()
	if err := tiler.processLasFileList(lasFilePathList, opts, tree); err != nil {
		log.Fatal(err)
		return err
	}

	tiler.exportTreeRootTileset(tree, opts)

	// tiler.exportRootNodeLas(mergedTree, opts)

	tiler.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()

	tools.LogOutput("> done merging", opts.Input)

	return nil
}

func (tiler *TilerMerge) processLasFileList(lasFilePathList []string, opts *tiler.TilerOptions, tree octree.ITree) error {
	// load las points in octree buffer
	treeList := make([]*grid_tree.GridTree, 0)
	for i, filePath := range lasFilePathList {
		// Define point_loader strategy
		var tree = tiler.algorithmManager.GetTreeAlgorithm()
		tools.LogOutput("Processing file " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(lasFilePathList)) + ", " + filePath)
		tiler.processSingleLasFile(filePath, opts, tree)

		treeList = append(treeList, tree.(*grid_tree.GridTree))
	}

	if err := tiler.BuildParentTree(tree, treeList); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (tiler *TilerMerge) processSingleLasFile(filePath string, opts *tiler.TilerOptions, tree octree.ITree) {
	// Create empty octree
	lasFileLoader, err := tiler.readLasData(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = lasFileLoader.LasFile.Close() }()

	tiler.prepareDataStructure(tree)
	log.Println(tree.GetRootNode().NumberOfPoints(), tree.GetRootNode().TotalNumberOfPoints())

	tools.LogOutput("> done processing", filepath.Base(filePath))
}

func (tiler *TilerMerge) readLasData(filePath string, opts *tiler.TilerOptions, tree octree.ITree) (*lidario.LasFileLoader, error) {
	// Reading files
	tools.LogOutput("> reading data from las file...", filepath.Base(filePath))
	lasFileLoader, err := readLas(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader, nil
}

func (tiler *TilerMerge) prepareDataStructure(octree octree.ITree) {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")
	err := octree.Build()

	if err != nil {
		log.Fatal(err)
	}
}

func (tiler *TilerMerge) BuildParentTree(octree octree.ITree, treeList []*grid_tree.GridTree) error {
	// Build tree hierarchical structure
	tools.LogOutput("> building parent tree structure...")

	// add point to parent-tree.loader
	for _, tree := range treeList {
		rootNode := tree.GetRootNode()
		rootNodePoints := tree.GetRootNode().GetPoints()
		for _, point := range rootNodePoints {
			octree.AddPoint(
				&geometry.Coordinate{X: point.X, Y: point.Y, Z: point.Z},
				point.R, point.G, point.B,
				point.Intensity, point.Classification, rootNode.GetInternalSrid(),
				point.PointExtend)

		}

	}

	// prepare parent-tree hierachy
	tiler.prepareDataStructure(octree)

	// update parent-tree bbox
	bboxList := make([]*geometry.BoundingBox, 0)
	nodeList := make([]*grid_tree.GridNode, 0)
	for _, tree := range treeList {
		rootNode := tree.GetRootNode()
		bboxList = append(bboxList, rootNode.GetBoundingBox())
		nodeList = append(nodeList, rootNode.(*grid_tree.GridNode))
	}

	rootNode := octree.GetRootNode().(*grid_tree.GridNode)
	rootNode.SetSpartialBoundingBoxByMergeBbox(bboxList)
	rootNode.SetChildren(nodeList)

	return nil
}

func (tiler *TilerMerge) exportTreeRootTileset(octree octree.ITree, opts *tiler.TilerOptions) {
	tools.LogOutput("> exporting data...")
	err := tiler.exportRootNodeTileset(opts, octree)
	if err != nil {
		log.Fatal(err)
	}
}

// Exports the data cloud represented by the given built octree into 3D tiles data structure according to the options
// specified in the TilerOptions instance
func (tiler *TilerMerge) exportRootNodeTileset(opts *tiler.TilerOptions, octree octree.ITree) error {
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

	outputDir := opts.Input
	subfolder := ""
	producer := io.NewStandardMergeProducer(outputDir, subfolder, opts)
	go producer.Produce(workChannel, &waitGroup, octree.GetRootNode())

	// add consumers to waitgroup and launch them
	for i := 0; i < numConsumers; i++ {
		waitGroup.Add(1)
		consumer := io.NewStandardConsumer(tiler.algorithmManager.GetCoordinateConverterAlgorithm(), opts.RefineMode)
		go consumer.Consume(workChannel, errorChannel, &waitGroup)
	}

	// wait for producers and consumers to finish
	waitGroup.Wait()

	// close error chan
	close(errorChannel)

	// find if there are errors in the error channel buffer
	withErrors := false
	for err := range errorChannel {
		fmt.Println(err)
		withErrors = true
	}
	if withErrors {
		return errors.New("errors raised during execution. Check console output for details")
	}

	return nil
}

func (tiler *TilerMerge) exportRootNodeLas(octree octree.ITree, opts *tiler.TilerOptions, filePath string, lasFile *lidario.LasFile) error {
	fileName := getFilenameWithoutExtension(filePath)
	subFolder := fileName
	parentFolder := path.Join(opts.TilerIndexOptions.Output, subFolder)

	var err error

	// var lf *lidario.LasFile
	// lf, err = lidario.NewLasFile(filePath, "r")
	// if err != nil {
	// 	fmt.Println(err)
	// 	log.Fatal(err)
	// }
	// defer lf.Close()

	newFileName := path.Join(parentFolder, "content.las")
	newLf, err := lidario.InitializeUsingFile(newFileName, lasFile)
	if err != nil {
		log.Println(err)
		log.Fatal(err)
	}

	progress := 0
	oldProgress := -1

	rootNode := octree.GetRootNode()
	numberOfPoints := rootNode.NumberOfPoints()
	points := rootNode.GetPoints()

	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints())

	for i := 0; i < int(numberOfPoints); i++ {
		point := points[i]

		pointLas, err := lasFile.LasPoint(point.PointExtend.LasPointIndex)
		if err != nil {
			log.Println(err)
			log.Fatal(err)
			return err
		}

		newLf.AddLasPoint(pointLas)

		// print export-progress
		progress = int(100.0 * float64(i) / float64(numberOfPoints))
		if progress != oldProgress {
			oldProgress = progress
			if progress%10 == 0 {
				fmt.Printf("export root_node rogress: %v\n", progress)
			}
		}
	}

	newLf.Close()

	return nil
}
