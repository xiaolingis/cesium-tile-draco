package pkg

import (
	"log"
	"strconv"

	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/pkg/algorithm_manager"
	"github.com/mfbonfigli/gocesiumtiler/tools"
)

type TilerMerge struct {
	fileFinder       tools.FileFinder
	algorithmManager algorithm_manager.AlgorithmManager
}

func NewTilerMerge(fileFinder tools.FileFinder, algorithmManager algorithm_manager.AlgorithmManager) ITiler {
	return &Tiler{
		fileFinder:       fileFinder,
		algorithmManager: algorithmManager,
	}
}

func (tiler *TilerMerge) RunTiler(opts *tiler.TilerOptions) error {
	tools.LogOutput("Preparing list of files to process...")

	// Prepare list of files to process
	lasFiles := tiler.fileFinder.GetLasFilesToProcess(opts)
	for i, filePath := range lasFiles {
		log.Printf("las_file path %d [%s]", i, filePath)
	}

	// load las points in octree buffer
	for i, filePath := range lasFiles {
		// Define point_loader strategy
		// var tree = tiler.algorithmManager.GetTreeAlgorithm()
		tools.LogOutput("Processing file " + strconv.Itoa(i+1) + "/" + strconv.Itoa(len(lasFiles)) + ", " + filePath)
		// tiler.processLasFile(filePath, opts, tree)
	}
	tiler.algorithmManager.GetCoordinateConverterAlgorithm().Cleanup()

	return nil
}
