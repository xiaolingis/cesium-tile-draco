package octree

import (
	"github.com/ecopia-map/cesium_tiler/internal/converters"
	"github.com/ecopia-map/cesium_tiler/internal/data"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
)

type ITree interface {
	Build() error
	GetRootNode() INode
	IsBuilt() bool
	Clear() bool
	// Adds a Point to the Tree
	AddPoint(coordinate *geometry.Coordinate, r uint8, g uint8, b uint8, intensity uint8, classification uint8, srid int, pointExtend *data.PointExtend)
}

type INode interface {
	AddDataPoint(element *data.Point)
	GetInternalSrid() int
	IsRoot() bool
	GetBoundingBoxRegion(converter converters.CoordinateConverter) (*geometry.BoundingBox, error)
	GetChildren() [8]INode
	GetPoints() []*data.Point
	TotalNumberOfPoints() int64
	NumberOfPoints() int32
	IsLeaf() bool
	IsChildrenInitialized() bool
	ComputeGeometricError() float64
	GetParent() INode
	GetBoundingBox() *geometry.BoundingBox
}
