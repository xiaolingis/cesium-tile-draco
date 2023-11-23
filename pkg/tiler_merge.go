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
	"strings"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/geometry"
	"github.com/mfbonfigli/gocesiumtiler/internal/io"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager/std_algorithm_manager"
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
	if opts.Command == tools.CommandMergeChildren {
		if err := tiler.RunTilerMergeChildren(opts); err != nil {
			log.Println(err)
			return nil
		}
	} else if opts.Command == tools.CommandMergeTree {
		if err := tiler.RunTilerMergeTree(opts); err != nil {
			log.Println(err)
			return nil
		}
	}

	return nil
}

func (tiler *TilerMerge) RunTilerMergeChildren(opts *tiler.TilerOptions) error {
	log.Println("Preparing list of files to process...")

	// Prepare list of files to process
	lasFilePathList := tiler.fileFinder.GetLasFilesToMerge(opts)
	log.Println("las_file list", lasFilePathList)

	if len(lasFilePathList) == 0 {
		err := fmt.Errorf("no children las-file found. input:[%s]", opts.Input)
		log.Fatal(err.Error() + ". " + tools.FmtJSONString(opts))
		return err
	}

	for i, filePath := range lasFilePathList {
		log.Printf("las_file path %d [%s]", i+1, filePath)
	}

	tree := tiler.algorithmManager.GetTreeAlgorithm()
	lasFile, err := tiler.mergeLasFileListToSingleTree(lasFilePathList, opts, tree)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer func() { _ = lasFile.Close() }()

	tiler.exportTreeRootTileset(tree, opts)

	tiler.exportRootNodeLas(tree, opts, lasFile)

	tiler.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()

	tools.LogOutput("> done merging-children", opts.Input)

	return nil
}

func (tiler *TilerMerge) RunTilerMergeTree(opts *tiler.TilerOptions) error {
	log.Println("Preparing list of files to process...")

	rootDir := strings.TrimSuffix(filepath.Join(opts.Input, ""), "/")

	levelDirsMap := make(map[int][]string)

	// baseInfo, _ := os.Stat(opts.Input)
	err := filepath.Walk(
		rootDir,
		func(path string, info os.FileInfo, err error) error {
			pathDepth := strings.Count(strings.TrimPrefix(path, rootDir), string("/"))
			// log.Println("walk_path:", path, ", pathDepth:", pathDepth)

			// if os.SameFile(info, baseInfo) {
			// 	levelDirsMap[0] = append(levelDirsMap[0], rootDir)
			// 	return nil // walk_into root_dir
			// }

			if info.IsDir() {
				dirPath := strings.TrimSuffix(filepath.Join(path, ""), "/")
				lasFileDepth := 10

				if pathDepth > lasFileDepth {
					return filepath.SkipDir
				}

				if strings.HasPrefix(filepath.Base(path), tools.ChunkTilesetFilePrefix) {
					pointsFilePath := filepath.Join(path, "/content.pnts")
					if _, err := os.Stat(pointsFilePath); err == nil {
						return filepath.SkipDir
					}
				}

				levelDirsMap[pathDepth] = append(levelDirsMap[pathDepth], dirPath)
			}

			return nil // walk_into only for dir
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	log.Println("merge level-tree. level-num:", len(levelDirsMap))

	for level, dirList := range levelDirsMap {
		log.Println("level-folder", level, tools.FmtJSONString(dirList))
		log.Println("level-folder", level, len(dirList))
	}

	log.Println("opts", tools.FmtJSONString(opts))

	maxLevel := len(levelDirsMap) - 1

	// levelDirsList := make([][]string, 0)
	// for i := 0; i <= maxLevel; i++ {
	// 	levelDirsList = append(levelDirsList, levelDirsMap[i])
	// }

	cellSize := opts.CellMaxSize
	for i := maxLevel; i >= 0; i-- {
		for _, dir := range levelDirsMap[i] {
			dirOpts := opts.Copy()
			dirOpts.Input = dir
			dirOpts.CellMaxSize = cellSize * 2
			dirOpts.CellMinSize = cellSize

			log.Println("dirOpts", tools.FmtJSONString(dirOpts))
			tiler.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()
			tiler.algorithmManager = std_algorithm_manager.NewAlgorithmManager(dirOpts)

			tiler.RunTilerMergeChildren(dirOpts)
		}
		cellSize *= 2

	}

	tools.LogOutput("> done merging-tree", opts.Input)

	return nil
}

func (tiler *TilerMerge) mergeLasFileListToSingleTree(
	lasFilePathList []string, opts *tiler.TilerOptions, tree octree.ITree,
) (lasFile *lidario.LasFile, _err error) {

	// merge multi sub-folder las to single-las
	mergedLasFilePath, err := tiler.mergeLasFileList(lasFilePathList)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	log.Println("mergedLasFilePath", mergedLasFilePath)

	// load merged single-las
	tools.LogOutput("Processing file " + mergedLasFilePath)
	lasFileLoader, err := tiler.readLasData(mergedLasFilePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	tiler.prepareDataStructure(tree)
	log.Println(tree.GetRootNode().NumberOfPoints(), tree.GetRootNode().TotalNumberOfPoints())

	// load sub-folder las points in octree buffer
	lasTreeList := make([]*grid_tree.GridTree, 0)
	for i, filePath := range lasFilePathList {
		// Define point_loader strategy
		lasTree := tiler.algorithmManager.GetTreeAlgorithm()
		tools.LogOutput("Processing file " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(lasFilePathList)) + ", " + filePath)
		tiler.loadLasFileIntoTree(filePath, opts, lasTree)

		lasTreeList = append(lasTreeList, lasTree.(*grid_tree.GridTree))
	}

	/*
		// add point to parent-tree.loader
		for _, tree := range treeList {
			rootNode := tree.GetRootNode()
			rootNodePoints := tree.GetRootNode().GetPoints()
			for _, point := range rootNodePoints {
				parentTree.AddPoint(
					&geometry.Coordinate{X: point.X, Y: point.Y, Z: point.Z},
					point.R, point.G, point.B,
					point.Intensity, point.Classification, rootNode.GetInternalSrid(),
					point.PointExtend)

			}
		}

		// prepare parent-tree hierachy
		tiler.prepareDataStructure(parentTree)
	*/

	if err := tiler.RepairParentTree(tree, lasTreeList); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader.LasFile, nil
}

func (tiler *TilerMerge) loadLasFileIntoTree(filePath string, opts *tiler.TilerOptions, tree octree.ITree) {
	// Create octree from las
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

func (tiler *TilerMerge) prepareDataStructure(octree octree.ITree) error {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")

	if err := octree.Build(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func (tiler *TilerMerge) mergeLasFileList(lasFilePathList []string) (_mergeLasFilePath string, _err error) {
	mergedLasFilePath := "/tmp/merged.las"

	filePath := lasFilePathList[0]
	lf, err := lidario.NewLasFile(filePath, "r")
	if err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}
	defer lf.Close()

	newLf, err := lidario.InitializeUsingFile(mergedLasFilePath, lf)
	if err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}
	defer newLf.Close()

	for i, filePath := range lasFilePathList {
		log.Printf("mergeLasFileList %d/%d %s", i+1, len(lasFilePathList), filePath)
		lf, err := lidario.NewLasFile(filePath, "r")
		if err != nil {
			log.Println(err)
			log.Fatal(err)
			return "", err
		}

		defer lf.Close()

		for i := 0; i < int(lf.Header.NumberPoints); i++ {
			p, err := lf.LasPoint(i)
			if err != nil {
				log.Println(err)
				log.Fatal(err)
				return "", err
			}
			newLf.AddLasPoint(p)
		}
	}

	return mergedLasFilePath, nil
}

func (tiler *TilerMerge) RepairParentTree(octree octree.ITree, treeList []*grid_tree.GridTree) error {
	// Build tree hierarchical structure
	tools.LogOutput("> building parent tree structure...")

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

func (tiler *TilerMerge) exportRootNodeLas(octree octree.ITree, opts *tiler.TilerOptions, lasFile *lidario.LasFile) error {
	parentFolder := opts.Input

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
	defer func() { newLf.Close() }()

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
		progress = int(100.0 * float64(i+1) / float64(numberOfPoints))
		if progress != oldProgress {
			oldProgress = progress
			if progress%50 == 0 {
				fmt.Printf("export root-node progress: %v\n", progress)
			}
		}
	}

	return nil
}