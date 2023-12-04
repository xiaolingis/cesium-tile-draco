package algorithm_manager

import (
	"github.com/mfbonfigli/gocesiumtiler/internal/converters"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
)

type AlgorithmManager interface {
	GetElevationCorrectionAlgorithm() converters.ElevationCorrector
	GetTreeAlgorithm() *grid_tree.GridTree
	GetCoordinateConverterAlgorithm() converters.CoordinateConverter
}
