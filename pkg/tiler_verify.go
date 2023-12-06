package pkg

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager"
	lidario "github.com/mfbonfigli/gocesiumtiler/third_party/lasread"
	"github.com/mfbonfigli/gocesiumtiler/tools"
)

type TilerVerify struct {
	fileFinder       tools.FileFinder
	algorithmManager algorithm_manager.AlgorithmManager
}

func NewTilerVerify(fileFinder tools.FileFinder, algorithmManager algorithm_manager.AlgorithmManager) ITiler {
	return &TilerVerify{
		fileFinder:       fileFinder,
		algorithmManager: algorithmManager,
	}
}

func (tilerVerify *TilerVerify) RunTiler(opts *tiler.TilerOptions) error {
	if opts.Command == tools.CommandVerifyLas {

		if err := tilerVerify.RunTilerVerifyLas(opts); err != nil {
			log.Println(err)
			return nil
		}
	} else if opts.Command == tools.CommandVerifyLasMerge {
		lasFilePathList := []string{
			// "./las/10009-6-29-20-1.las",
			// "./las/10009-7-58-40-0.las",
			// "./las/10009-8-117-87-2.las",
			"./las/10009-7-57-41-1.las",
			// "tileset-las1/chunk-tileset-10009-6-29-20-1/content.las",
			// "tileset-las1/chunk-tileset-10009-7-58-40-0/content.las",
			// "tileset-las1/chunk-tileset-10009-8-117-87-2/content.las",
			// "tileset-las1/chunk-tileset-10009-7-57-41-1/content.las",
			// "./0/7/7-57-41-1.las",
			// "tileset/0/chunk-tileset-7-57-41-1/content.las",
		}
		mergedLasFilePath, err := tilerVerify.mergeLasFileListCheck(lasFilePathList)
		if err != nil {
			log.Fatal(err)
			return nil
		}
		log.Println("mergedLasFilePath", mergedLasFilePath)
		return nil
	}

	return nil
}

func (tilerVerify *TilerVerify) RunTilerVerifyLas(opts *tiler.TilerOptions) error {
	filePath := opts.Input

	// Create empty octree
	tree := tilerVerify.algorithmManager.GetTreeAlgorithm()
	lasFileLoader, err := tilerVerify.readLasData(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = lasFileLoader.LasFile.Clear()
		_ = lasFileLoader.LasFile.Close()
		// lasFileLoader.LasFile = nil
		// lasFileLoader.Tree = nil
	}()

	tilerVerify.VerifyLasLoader(opts)

	tilerVerify.prepareDataStructure(tree)

	tilerVerify.VerifyLas(lasFileLoader.LasFile, opts)

	tools.LogOutput("> done processing", filepath.Base(filePath))

	return nil
}

func (tilerVerify *TilerVerify) readLasData(filePath string, opts *tiler.TilerOptions, tree *grid_tree.GridTree) (*lidario.LasFileLoader, error) {
	// Reading files
	tools.LogOutput("> reading data from las file...", filepath.Base(filePath))
	lasFileLoader, err := readLas(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader, nil
}

func (tilerVerify *TilerVerify) prepareDataStructure(octree *grid_tree.GridTree) {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")

	if err := octree.Build(); err != nil {
		log.Fatal(err)
	}

	rootNode := octree.GetRootNode()
	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints(), ", points.len:", len(rootNode.GetPoints()))

}

func (tilerVerify *TilerVerify) VerifyLasLoader(opts *tiler.TilerOptions) error {

	return nil
}

func (tilerVerify *TilerVerify) VerifyLas(lasFile *lidario.LasFile, opts *tiler.TilerOptions) error {
	lasHeader := lasFile.Header
	log.Println("las_file num_of_points:", lasHeader.NumberPoints)

	for i := 0; i < int(lasHeader.NumberPoints); i++ {

		pointLas, err := lasFile.LasPoint(i)
		if err != nil {
			log.Println(err)
			log.Fatal(err)
			return err
		}

		X, Y, Z := pointLas.PointData().X, pointLas.PointData().Y, pointLas.PointData().Z
		if !lasFile.CheckPointXYZInvalid(X, Y, Z) {
			log.Printf(" nonono invalid point_pos:[%d] X:[%f] Y:[%f] Z:[%f]", i, X, Y, Z)
			continue
		}

		if i < 10 {
			log.Printf(" okokok valid point_pos:[%d] X:[%f] Y:[%f] Z:[%f]", i, X, Y, Z)
		}

	}

	log.Println("Verify las file success.")

	return nil
}

func (tilerVerify *TilerVerify) mergeLasFileListCheck(lasFilePathList []string) (_mergeLasFilePath string, _err error) {
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
