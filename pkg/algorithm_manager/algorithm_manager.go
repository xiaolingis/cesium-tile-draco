package algorithm_manager

import (
	"github.com/ecopia-map/cesium_tiler/internal/converters"
	"github.com/ecopia-map/cesium_tiler/internal/octree/grid_tree"
)

type AlgorithmManager interface {
	GetElevationCorrectionAlgorithm() converters.ElevationCorrector
	GetTreeAlgorithm() *grid_tree.GridTree
	GetCoordinateConverterAlgorithm() converters.CoordinateConverter
}
