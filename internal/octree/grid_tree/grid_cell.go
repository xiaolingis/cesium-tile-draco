package grid_tree

import (
	"math"
	"sync"

	"github.com/ecopia-map/cesium_tiler/internal/data"
)

// Data structure that accepts points and stores just the one closest to its center, or if the side is too small,
// all the points. It assumes that coordinates are expressed in a metric cartesian system.
type gridCell struct {
	index              gridIndex     // unique spatial index of the cell
	size               float64       // length of the side of the cell (cubic cell)
	points             []*data.Point // points stored in the cell
	sizeThreshold      float64       // if size is below sizeThreshold store all points in the cell instead of just the one closest to the center
	distanceFromCenter float64       // distance from center of current point at index 0
	sync.RWMutex
}

// returns the spatial index component associated to a given dimension (e.g. X or Y or Z) coordinate value
func getDimensionIndex(dimensionValue float64, size float64) int {
	return int(math.Floor(dimensionValue / size))
}

// returns the cell center X,Y,Z coordinates from the spatial index of the cell and the cell size
func (gc *gridCell) getCellCenter() (float64, float64, float64) {
	return float64(gc.index.x)*gc.size + gc.size/2,
		float64(gc.index.y)*gc.size + gc.size/2,
		float64(gc.index.z)*gc.size + gc.size/2
}

// submits a point to the cell, eventually returning a pointer to the point pushed out.
func (gc *gridCell) pushPoint(point *data.Point, isFollowSizeThreshold bool) *data.Point {
	gc.Lock()
	if gc.points == nil {
		gc.storeFirstPoint(point)
		gc.Unlock()
		return nil
	}

	if isFollowSizeThreshold && gc.isSizeBelowThreshold() {
		gc.points = append(gc.points, point)
		gc.Unlock()
		return nil
	}

	retPoint := gc.storeClosestPointAndReturnFarthestOne(point)
	gc.Unlock()

	return retPoint
}

// checks if the cell has reached the lower size limit for which it must store all points submitted
func (gc *gridCell) isSizeBelowThreshold() bool {
	return gc.size < gc.sizeThreshold
}

// sets the points slice to a new slice containing the input point and stores its distanceFromCenter
func (gc *gridCell) storeFirstPoint(point *data.Point) {
	gc.points = []*data.Point{point}
	gc.distanceFromCenter = gc.getDistanceFromCenter(point)
}

// takes the input point and compares its distance from the center to the one in the points array,
// storing in the array only the one closest to the center and returning the other, rejected and farthest from the center, one
func (gc *gridCell) storeClosestPointAndReturnFarthestOne(point *data.Point) *data.Point {
	distance := gc.getDistanceFromCenter(point)

	if distance < gc.distanceFromCenter {
		oldPoint := gc.points[0]
		gc.points[0] = point
		gc.distanceFromCenter = distance
		return oldPoint
	}

	return point
}

// computes the cartesian distance of a point from the cell center
func (gc *gridCell) getDistanceFromCenter(point *data.Point) float64 {
	xc, yc, zc := gc.getCellCenter()

	return math.Sqrt(
		math.Pow(point.X-xc, 2) +
			math.Pow(point.Y-yc, 2) +
			math.Pow(point.Z-zc, 2),
	)
}
