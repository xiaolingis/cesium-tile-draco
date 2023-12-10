package converters

import (
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
)

type CoordinateConverter interface {
	ConvertCoordinateSrid(sourceSrid int, targetSrid int, coord geometry.Coordinate) (geometry.Coordinate, error)
	Convert2DBoundingboxToWGS84Region(bbox *geometry.BoundingBox, srid int) (*geometry.BoundingBox, error)
	ConvertToWGS84Cartesian(coord geometry.Coordinate, sourceSrid int) (geometry.Coordinate, error)
	Cleanup()
}
