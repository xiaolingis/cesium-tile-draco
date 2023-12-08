package grid_tree

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"

	"github.com/ecopia-map/cesium_tiler/internal/converters"
	"github.com/ecopia-map/cesium_tiler/internal/data"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
	"github.com/ecopia-map/cesium_tiler/internal/point_loader"
	"github.com/ecopia-map/cesium_tiler/tools"
)

// Coordinates are stored in EPSG 3395, which is a cartesian 2D metric reference system
const internalCoordinateEpsgCode = 3395

// Represents an GridTree of points and contains all information needed
// to propagate points in the tree
type GridTree struct {
	rootNode            *GridNode
	built               bool
	maxCellSize         float64
	minCellSize         float64
	coordinateConverter converters.CoordinateConverter
	elevationCorrector  converters.ElevationCorrector
	point_loader.Loader
	sync.RWMutex

	// Extend
	extend *GridTreeExtend
}

type GridTreeExtend struct {
	chunkEdgeX                     float64
	chunkEdgeY                     float64
	chunkEdgeZ                     float64
	useEdgeCalculateGeometricError bool
}

// Builds an empty GridTree initializing its properties to the correct defaults
func NewGridTree(
	coordinateConverter converters.CoordinateConverter,
	elevationCorrector converters.ElevationCorrector,
	maxCellSize float64,
	minCellSize float64,
) *GridTree {
	return &GridTree{
		built:               false,
		maxCellSize:         maxCellSize,
		minCellSize:         minCellSize,
		Loader:              point_loader.NewSequentialLoader(),
		coordinateConverter: coordinateConverter,
		elevationCorrector:  elevationCorrector,
		extend: &GridTreeExtend{
			chunkEdgeX: 0,
			chunkEdgeY: 0,
			chunkEdgeZ: 0,
		},
	}
}

// Builds the hierarchical tree structure
func (tree *GridTree) Build() error {
	if tree.built {
		return errors.New("octree already built")
	}

	tree.init()

	var wg sync.WaitGroup
	tree.launchParallelPointLoaders(&wg)
	wg.Wait()

	tree.Loader.ClearLoader()

	tree.rootNode.BuildPoints()
	tree.built = true

	return nil
}

func (tree *GridTree) GetRootNode() *GridNode {
	return tree.rootNode
}

func (tree *GridTree) IsBuilt() bool {
	return tree.built
}

func (tree *GridTree) Clear() bool {
	tree.clear()
	return true
}

func (tree *GridTree) AddPoint(
	coordinate *geometry.Coordinate,
	r uint8, g uint8, b uint8,
	intensity uint8, classification uint8, srid int,
	pointExtend *data.PointExtend,
) {
	tree.Loader.AddPoint(tree.getPointFromRawData(coordinate, r, g, b, intensity, classification, srid, pointExtend))
}

func (tree *GridTree) getPointFromRawData(
	coordinate *geometry.Coordinate,
	r uint8, g uint8, b uint8,
	intensity uint8, classification uint8, srid int,
	pointExtend *data.PointExtend,
) *data.Point {
	wgs84coords, err := tree.coordinateConverter.ConvertCoordinateSrid(srid, 4326, *coordinate)
	if err != nil {
		log.Println(err)
		log.Fatal(err)
	}

	z := tree.elevationCorrector.CorrectElevation(wgs84coords.X, wgs84coords.Y, wgs84coords.Z)

	worldMercatorCoords, err := tree.coordinateConverter.ConvertCoordinateSrid(
		srid,
		internalCoordinateEpsgCode,
		geometry.Coordinate{
			X: coordinate.X,
			Y: coordinate.Y,
			Z: z,
		},
	)

	if err != nil {
		log.Println(err, fmt.Sprintf("srid:[%d] coordinate:[%s]", srid, tools.FmtJSONString(coordinate)))
		log.Println(err, fmt.Sprintf("coordinate X:[%f] Y:[%f] Z:[%f]", coordinate.X, coordinate.Y, z))
		log.Fatal(err)
	}

	return data.NewPoint(
		worldMercatorCoords.X,
		worldMercatorCoords.Y,
		worldMercatorCoords.Z,
		r, g, b, intensity, classification,
		pointExtend,
	)
}

func (tree *GridTree) GetTreeExtend() *GridTreeExtend {
	return tree.extend
}

func (tree *GridTree) GetBounds() []float64 {
	box := tree.Loader.GetBounds()
	minX, maxX, minY, maxY, minZ, maxZ := box[0], box[1], box[2], box[3], box[4], box[5]

	if tools.IsFloatEqual(minX, maxX) {
		minX -= 0.001
		maxX += 0.001
	}
	if tools.IsFloatEqual(minY, maxY) {
		minY -= 0.001
		maxY += 0.001
	}
	if tools.IsFloatEqual(minZ, maxZ) {
		minZ -= 0.001
		maxZ += 0.001
	}

	return []float64{minX, maxX, minY, maxY, minZ, maxZ}
}

func (tree *GridTree) init() {
	box := tree.GetBounds()

	// box  {eb.minX, eb.maxX, eb.minY, eb.maxY, eb.minZ, eb.maxZ}
	log.Println("tree.box(minX,maxX,minY,maxY,minZ,maxZ):" + tools.FmtJSONString(box))
	log.Println("x-diff:", box[1]-box[0], ", y-diff:", box[3]-box[2], ", z-diff:", box[5]-box[4])

	node := NewGridNode(
		"r",
		tree,
		nil,
		geometry.NewBoundingBox(box[0], box[1], box[2], box[3], box[4], box[5]),
		tree.maxCellSize,
		tree.minCellSize,
		true,
	)

	tree.rootNode = node
	tree.InitializeLoader()
}

func (tree *GridTree) clear() {
	tree.rootNode = nil
	tree.Loader.ClearLoader()
}

func (tree *GridTree) launchParallelPointLoaders(waitGroup *sync.WaitGroup) {
	N := runtime.NumCPU()

	for i := 0; i < N; i++ {
		waitGroup.Add(1)
		go tree.launchPointLoader(waitGroup)
	}
}

func (tree *GridTree) launchPointLoader(waitGroup *sync.WaitGroup) {
	for {
		val, shouldContinue := tree.Loader.GetNext()
		if val != nil {
			tree.rootNode.AddDataPoint(val, true)
		}
		if !shouldContinue {
			break
		}
	}
	waitGroup.Done()
}

func (tree *GridTree) UpdateExtendChunkEdge(chunkEdgeX, chunkEdgeY, chunkEdgeZ float64, useEdgeCalculateGeometricError bool) {
	tree.extend.chunkEdgeX = chunkEdgeX
	tree.extend.chunkEdgeY = chunkEdgeY
	tree.extend.chunkEdgeZ = chunkEdgeZ
	tree.extend.useEdgeCalculateGeometricError = useEdgeCalculateGeometricError
}

func (tree *GridTree) MergeSmallNode(minPointsNum int32) error {
	if !tree.built {
		err := errors.New("octree does not built")
		log.Fatal(err)
		return err
	}

	if err := tree.rootNode.MergeSmallChildren(int64(minPointsNum)); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func (tree *GridTree) SplitBigNode(maxPointsNum int32) error {
	if !tree.built {
		err := errors.New("octree does not built")
		log.Fatal(err)
		return err
	}

	if err := tree.rootNode.SplitBigNode(maxPointsNum); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}
