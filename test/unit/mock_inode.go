package unit

import "C"
import (
	"github.com/ecopia-map/cesium_tiler/internal/converters"
	"github.com/ecopia-map/cesium_tiler/internal/data"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
	"github.com/ecopia-map/cesium_tiler/internal/octree"
	"github.com/ecopia-map/cesium_tiler/internal/tiler"
	"sync"
)

// mock implementation of the INode interface
type mockNode struct {
	parent              octree.INode
	boundingBox         *geometry.BoundingBox
	children            [8]octree.INode
	points              []*data.Point
	internalSrid        int
	depth               uint8
	globalChildrenCount int64
	localChildrenCount  int32
	opts                *tiler.TilerOptions
	leaf                bool
	initialized         bool
	geometricError      float64
	sync.RWMutex
}

func (mockNode *mockNode) AddDataPoint(element *data.Point) {}

func (mockNode *mockNode) IsRoot() bool {
	return mockNode.parent == nil
}

func (mockNode *mockNode) GetBoundingBoxRegion(converter converters.CoordinateConverter) (*geometry.BoundingBox, error) {
	reg, err := converter.Convert2DBoundingboxToWGS84Region(mockNode.boundingBox, mockNode.GetInternalSrid())

	if err != nil {
		return nil, err
	}

	return reg, nil
}

func (mockNode *mockNode) GetBoundingBox() *geometry.BoundingBox {
	return mockNode.boundingBox
}

func (mockNode *mockNode) GetChildren() [8]octree.INode {
	return mockNode.children
}

func (mockNode *mockNode) GetPoints() []*data.Point {
	return mockNode.points
}

func (mockNode *mockNode) GetInternalSrid() int {
	return mockNode.internalSrid
}

func (mockNode *mockNode) GetDepth() uint8 {
	return mockNode.depth
}

func (mockNode *mockNode) TotalNumberOfPoints() int64 {
	return mockNode.globalChildrenCount
}

func (mockNode *mockNode) NumberOfPoints() int32 {
	return mockNode.localChildrenCount
}

func (mockNode *mockNode) IsLeaf() bool {
	return mockNode.leaf
}

func (mockNode *mockNode) IsInitialized() bool {
	return mockNode.initialized
}

func (mockNode *mockNode) ComputeGeometricError() float64 {
	return mockNode.geometricError
}

func (mockNode *mockNode) GetParent() octree.INode {
	return mockNode.parent
}
