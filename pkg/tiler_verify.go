package pkg

import (
	"log"
	"path/filepath"

	"github.com/mfbonfigli/gocesiumtiler/internal/octree"
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

func (tiler *TilerVerify) RunTiler(opts *tiler.TilerOptions) error {
	if opts.Command == tools.CommandVerifyLas {

		if err := tiler.RunTilerVerifyLas(opts); err != nil {
			log.Println(err)
			return nil
		}
	}

	return nil
}

func (tiler *TilerVerify) RunTilerVerifyLas(opts *tiler.TilerOptions) error {
	filePath := opts.Input

	// Create empty octree
	tree := tiler.algorithmManager.GetTreeAlgorithm()
	lasFileLoader, err := tiler.readLasData(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = lasFileLoader.LasFile.Clear()
		_ = lasFileLoader.LasFile.Close()
		// lasFileLoader.LasFile = nil
		// lasFileLoader.Tree = nil
	}()

	tiler.prepareDataStructure(tree)

	tools.LogOutput("> done processing", filepath.Base(filePath))

	return nil
}

func (tiler *TilerVerify) readLasData(filePath string, opts *tiler.TilerOptions, tree octree.ITree) (*lidario.LasFileLoader, error) {
	// Reading files
	tools.LogOutput("> reading data from las file...", filepath.Base(filePath))
	lasFileLoader, err := readLas(filePath, opts, tree)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return lasFileLoader, nil
}

func (tiler *TilerVerify) prepareDataStructure(octree octree.ITree) {
	// Build tree hierarchical structure
	tools.LogOutput("> building data structure...")

	if err := octree.Build(); err != nil {
		log.Fatal(err)
	}

	rootNode := octree.GetRootNode()
	log.Println("las_file root_node num_of_points:", rootNode.NumberOfPoints(), ", points.len:", len(rootNode.GetPoints()))

}

func (tiler *TilerVerify) VerifyLas(lasFile *lidario.LasFile, opts *tiler.TilerOptions) error {
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
			log.Printf("invalid point_pos:[%d] X:[%f] Y:[%f] Z:[%f]", i, X, Y, Z)
			continue
		}

	}

	log.Println("Verify las file success.")

	return nil
}
