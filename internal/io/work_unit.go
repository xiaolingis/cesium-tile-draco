package io

import (
	"github.com/ecopia-map/cesium_tiler/internal/octree/grid_tree"
	"github.com/ecopia-map/cesium_tiler/internal/tiler"
)

// Contains the minimal data needed to produce a single 3d tile, i.e. a binary content.pnts file and a tileset.json file
type WorkUnit struct {
	Node     *grid_tree.GridNode
	Opts     *tiler.TilerOptions
	BasePath string
}
