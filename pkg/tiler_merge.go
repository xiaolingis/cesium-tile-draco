package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/geometry"
	"github.com/mfbonfigli/gocesiumtiler/internal/io"
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

func NewTilerMerge(fileFinder tools.FileFinder, algorithmManager algorithm_manager.AlgorithmManager) tiler.ITiler {
	return &TilerMerge{
		fileFinder:       fileFinder,
		algorithmManager: algorithmManager,
	}
}

func (tilerMerge *TilerMerge) RunTiler(opts *tiler.TilerOptions) error {
	if opts.Command == tools.CommandMergeChildren {
		if err := tilerMerge.RunTilerMergeChildren(opts); err != nil {
			log.Println(err)
			return nil
		}
	} else if opts.Command == tools.CommandMergeTree {
		if err := tilerMerge.RunTilerMergeTree(opts); err != nil {
			log.Println(err)
			return nil
		}
	}

	return nil
}

func (tilerMerge *TilerMerge) RunTilerMergeChildren(opts *tiler.TilerOptions) error {
	log.Println("Preparing list of files to process...")

	// Prepare list of files to process
	lasFilePathList := tilerMerge.fileFinder.GetLasFilesToMerge(opts)
	log.Println("las_file list", lasFilePathList)

	if len(lasFilePathList) == 0 {
		err := fmt.Errorf("no children las-file found. input:[%s]", opts.Input)
		log.Fatal(err.Error() + ". " + tools.FmtJSONString(opts))
		return err
	}

	for i, filePath := range lasFilePathList {
		log.Printf("las_file path %d [%s]", i+1, filePath)
	}

	tree := tilerMerge.algorithmManager.GetTreeAlgorithm()
	lasFile, err := tilerMerge.mergeLasFileListToSingleTree(lasFilePathList, opts, tree)
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer func() {
		_ = lasFile.Clear()
		_ = lasFile.Close()
	}()

	tilerMerge.exportTreeRootTileset(tree, opts)

	if err := tilerMerge.repairTilesetMetadata(opts, lasFilePathList); err != nil {
		log.Fatal(err)
		return err
	}

	tilerMerge.exportRootNodeLas(tree, opts, lasFile)

	tilerMerge.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()

	tools.LogOutput("> done merging-children", opts.Input)

	return nil
}

func (tilerMerge *TilerMerge) RunTilerMergeTree(opts *tiler.TilerOptions) error {
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
					pointsFilePath := filepath.Join(path, "/content.las")
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
			tilerMerge.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()
			tilerMerge.algorithmManager = std_algorithm_manager.NewAlgorithmManager(dirOpts)

			tilerMerge.RunTilerMergeChildren(dirOpts)

			if i == 1 {
				scale := 2
				if err := tilerMerge.AdjustRootGeometricError(dirOpts, scale); err != nil {
					log.Fatal(err)
				}
			} else if i == 0 {
				scale := 4
				if err := tilerMerge.AdjustRootGeometricError(dirOpts, scale); err != nil {
					log.Fatal(err)
				}
			}
		}
		cellSize *= 2

	}

	tools.LogOutput("> done merging-tree", opts.Input)

	return nil
}

func (tilerMerge *TilerMerge) mergeLasFileListToSingleTree(
	lasFilePathList []string, opts *tiler.TilerOptions, tree *grid_tree.GridTree,
) (lasFile *lidario.LasFile, _err error) {

	// merge multi sub-folder las to single-las
	mergedLasFilePath, err := tilerMerge.mergeLasFileList(lasFilePathList)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	log.Println("mergedLasFilePath", mergedLasFilePath)

	// load merged single-las
	tools.LogOutput("Processing file " + mergedLasFilePath)
	lasFileLoader, err := tilerMerge.readLasData(mergedLasFilePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	tilerMerge.prepareDataStructure(tree)
	log.Println(tree.GetRootNode().NumberOfPoints(), tree.GetRootNode().TotalNumberOfPoints())

	// load sub-folder las points in octree buffer
	lasTreeList := make([]*grid_tree.GridTree, 0)
	for i, filePath := range lasFilePathList {
		// Define point_loader strategy
		lasTree := tilerMerge.algorithmManager.GetTreeAlgorithm()
		tools.LogOutput("Processing file " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(lasFilePathList)) + ", " + filePath)
		tilerMerge.loadLasFileIntoTree(filePath, opts, lasTree)

		lasTreeList = append(lasTreeList, lasTree)
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
		tilerMerge.prepareDataStructure(parentTree)
	*/

	if err := tilerMerge.RepairParentTree(tree, lasTreeList); err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader.LasFile, nil
}

func (tilerMerge *TilerMerge) loadLasFileIntoTree(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) {
	// Create octree from las
	lasFileLoader, err := tilerMerge.readLasData(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = lasFileLoader.LasFile.Clear()
		_ = lasFileLoader.LasFile.Close()
	}()

	tilerMerge.prepareDataStructure(tree)
	log.Println(tree.GetRootNode().NumberOfPoints(), tree.GetRootNode().TotalNumberOfPoints())

	tools.LogOutput("> done processing", filepath.Base(filePath))
}

func (tilerMerge *TilerMerge) readLasData(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) (*lidario.LasFileLoader, error) {
	// Reading files
	tools.LogOutput("> reading data from las file...", filepath.Base(filePath))
	lasFileLoader, err := readLas(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader, nil
}

func (tilerMerge *TilerMerge) prepareDataStructure(octree *grid_tree.GridTree) error {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")

	if err := octree.Build(); err != nil {
		log.Fatal(err)
		return err
	}

	rootNode := octree.GetRootNode()
	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints(), ", points.len:", len(rootNode.GetPoints()))

	return nil
}
func (tilerMerge *TilerMerge) mergeLasFileList(lasFilePathList []string) (_mergeLasFilePath string, _err error) {
	mergedLasFilePath := "/tmp/merged.las"

	filePath := lasFilePathList[0]
	lf0, err := lidario.NewLasFile(filePath, "r")
	if err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}
	defer func() {
		if lf0 != nil {
			lf0.Close()
			lf0 = nil
		}
	}()

	if _, err := os.Stat(mergedLasFilePath); err == nil {
		if err := os.Remove(mergedLasFilePath); err != nil {
			log.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}
	newLf, err := lidario.InitializeUsingFile(mergedLasFilePath, lf0)
	if err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}
	defer func() {
		if newLf != nil {
			newLf.Close()
			newLf = nil
		}
	}()

	if err := newLf.CopyHeaderXYZ(lf0.Header); err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}

	lf0.Close()
	lf0 = nil

	for i, filePath := range lasFilePathList {
		log.Printf("mergeLasFileList %d/%d %s", i+1, len(lasFilePathList), filePath)
		lf, err := lidario.NewLasFile(filePath, "r")
		if err != nil {
			log.Println(err)
			log.Fatal(err)
			return "", err
		}
		defer lf.Close()

		if err := newLf.MergeHeaderXYZ(lf.Header); err != nil {
			log.Println(err)
			log.Fatal(err)
			return "", err
		}

		for i := 0; i < int(lf.Header.NumberPoints); i++ {
			// if i >= 5 {
			// 	break
			// }
			p, err := lf.LasPoint(i)
			if err != nil {
				log.Println(err)
				log.Fatal(err)
				return "", err
			}
			newLf.AddLasPoint(p)
		}

		lf.Close()
	}

	newLf.Close()
	newLf = nil

	// Check
	log.Printf("mergedLasFilePath %s", mergedLasFilePath)
	mergedLf, err := lidario.NewLasFile(mergedLasFilePath, "r")
	if err != nil {
		log.Println(err)
		log.Fatal(err)
		return "", err
	}
	defer mergedLf.Close()

	return mergedLasFilePath, nil
}

func (tilerMerge *TilerMerge) RepairParentTree(octree *grid_tree.GridTree, treeList []*grid_tree.GridTree) error {
	// Build tree hierarchical structure
	tools.LogOutput("> building parent tree structure...")

	// update parent-tree bbox
	bboxList := make([]*geometry.BoundingBox, 0)
	nodeList := make([]*grid_tree.GridNode, 0)
	for _, tree := range treeList {
		rootNode := tree.GetRootNode()
		bboxList = append(bboxList, rootNode.GetBoundingBox())
		nodeList = append(nodeList, rootNode)
	}

	rootNode := octree.GetRootNode()
	rootNode.SetSpartialBoundingBoxByMergeBbox(bboxList)
	rootNode.SetChildren(nodeList)

	return nil
}

func (tilerMerge *TilerMerge) repairTilesetMetadata(opts *tiler.TilerOptions, lasFilePathList []string) error {
	// folder hierachy
	/*
		${output}/
			|- tileset.json
			|- content.pnts
			|- content.las
			|
			 ---
			|    \
			|     chunk-tileset-0-xxx/
			|  		|- tileset.json
			|  		|- content.pnts
			|  		|- content.las
			...
			 ---
			|    \
			|     chunk-tileset-2-xxx/
			|  		|- tileset.json
			|  		|- content.pnts
			|  		|- content.las
			...
	*/
	rootDir := filepath.Join(opts.Input, "")

	// read tileset for root
	rootMetadataPath := filepath.Join(rootDir, "tileset.json")
	rootTileset := io.Tileset{}
	rootFile, err := ioutil.ReadFile(rootMetadataPath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	if err := json.Unmarshal([]byte(rootFile), &rootTileset); err != nil {
		log.Fatal(err)
		return err
	}

	// read tileset for children
	metadataPathList := make([]string, 0)
	for _, filePath := range lasFilePathList {
		metadataPath := filepath.Join(filepath.Dir(filePath), "tileset.json")
		metadataPathList = append(metadataPathList, metadataPath)
	}
	childTilesetList := make([]*io.Tileset, 0)
	for _, filePath := range metadataPathList {
		childTileset := io.Tileset{}
		childFile, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatal(err)
			return err
		}
		if err := json.Unmarshal([]byte(childFile), &childTileset); err != nil {
			log.Fatal(err)
			return err
		}
		childTilesetList = append(childTilesetList, &childTileset)
	}

	// merge tileset .children
	children := make([]io.Child, 0)
	childGeometricError := float64(0.0)
	for i, childTileset := range childTilesetList {
		metadataPath := metadataPathList[i]
		relativeMetadataPath := strings.TrimPrefix(strings.TrimPrefix(metadataPath, rootDir), "/")
		child := io.Child{
			Content: io.Content{
				Url: relativeMetadataPath,
			},
			BoundingVolume: childTileset.Root.BoundingVolume,
			GeometricError: childTileset.Root.GeometricError,
			Refine:         "REPLACE",
		}

		if childGeometricError < childTileset.Root.GeometricError {
			childGeometricError = childTileset.Root.GeometricError
		}

		children = append(children, child)
	}
	rootTileset.Root.Children = children
	rootTileset.Root.GeometricError = 2 * childGeometricError

	// merge tileset .boundingVolume
	region := rootTileset.Root.BoundingVolume.Region
	for _, childTileset := range childTilesetList {
		childRegion := childTileset.Root.BoundingVolume.Region

		region[0] = math.Min(float64(childRegion[0]), region[0])
		region[1] = math.Min(float64(childRegion[1]), region[1])
		region[2] = math.Max(float64(childRegion[2]), region[2])
		region[3] = math.Max(float64(childRegion[3]), region[3])
		region[4] = math.Min(float64(childRegion[4]), region[4])
		region[5] = math.Max(float64(childRegion[5]), region[5])
	}
	rootTileset.Root.BoundingVolume.Region = region

	// write root tilset.json
	// Outputting a formatted json file
	rootTilesetJSON, err := json.MarshalIndent(rootTileset, "", "\t")
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Writes the tileset.json binary content to the given file
	if err = ioutil.WriteFile(rootMetadataPath, rootTilesetJSON, 0666); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (tilerMerge *TilerMerge) AdjustRootGeometricError(opts *tiler.TilerOptions, scale int) error {
	// folder hierachy
	/*
		${output}/
			|- tileset.json
			|- content.pnts
			|- content.las

	*/
	rootDir := filepath.Join(opts.Input, "")

	// read tileset for root
	rootMetadataPath := filepath.Join(rootDir, "tileset.json")
	rootTileset := io.Tileset{}
	rootFile, err := ioutil.ReadFile(rootMetadataPath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	if err := json.Unmarshal([]byte(rootFile), &rootTileset); err != nil {
		log.Fatal(err)
		return err
	}

	rootTileset.Root.GeometricError *= float64(scale)

	// write root tilset.json
	// Outputting a formatted json file
	rootTilesetJSON, err := json.MarshalIndent(rootTileset, "", "\t")
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Writes the tileset.json binary content to the given file
	if err = ioutil.WriteFile(rootMetadataPath, rootTilesetJSON, 0666); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (tilerMerge *TilerMerge) exportTreeRootTileset(octree *grid_tree.GridTree, opts *tiler.TilerOptions) {
	tools.LogOutput("> exporting data...")
	err := tilerMerge.exportRootNodeTileset(opts, octree)
	if err != nil {
		log.Fatal(err)
	}
}

// Exports the data cloud represented by the given built octree into 3D tiles data structure according to the options
// specified in the TilerOptions instance
func (tilerMerge *TilerMerge) exportRootNodeTileset(opts *tiler.TilerOptions, tree *grid_tree.GridTree) error {
	// if octree is not built, exit
	if !tree.IsBuilt() {
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
	rootNode := tree.GetRootNode()
	go producer.Produce(workChannel, &waitGroup, rootNode)

	// add consumers to waitgroup and launch them
	for i := 0; i < numConsumers; i++ {
		waitGroup.Add(1)
		consumer := io.NewStandardConsumer(tilerMerge.algorithmManager.GetCoordinateConverterAlgorithm(), opts.RefineMode, opts.Draco, opts.DracoEncoderPath)
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

func (tilerMerge *TilerMerge) exportRootNodeLas(octree *grid_tree.GridTree, opts *tiler.TilerOptions, lasFile *lidario.LasFile) error {
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
				log.Printf("export root-node progress: %v\n", progress)
			}
		}
	}

	newLf.Close()
	newLf = nil

	return nil
}
