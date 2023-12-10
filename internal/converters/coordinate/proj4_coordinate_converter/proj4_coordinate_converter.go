package proj4_coordinate_converter

import (
	"bufio"
	"errors"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/ecopia-map/cesium_tiler/internal/converters"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
	"github.com/ecopia-map/cesium_tiler/tools"
	"github.com/golang/glog"
	proj "github.com/xeonx/proj4"
)

const toRadians = math.Pi / 180
const toDeg = 180 / math.Pi

type proj4CoordinateConverter struct {
	EpsgDatabase map[int]*epsgProjection
}

func NewProj4CoordinateConverter() converters.CoordinateConverter {
	exPath := tools.GetRootFolder()

	// Set path for retrieving projection assets data
	proj.SetFinder([]string{path.Join(exPath, "assets", "share")})

	// Initialization of EPSG Proj4 database
	file := path.Join(exPath, "assets", "epsg_projections.txt")

	return &proj4CoordinateConverter{
		EpsgDatabase: *loadEPSGProjectionDatabase(file),
	}
}

func loadEPSGProjectionDatabase(databasePath string) *map[int]*epsgProjection {
	file := tools.OpenFileOrFail(databasePath)
	defer func() { _ = file.Close() }()

	var epsgDatabase = make(map[int]*epsgProjection)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		record := scanner.Text()
		code, projection := parseEPSGProjectionDatabaseRecord(record)
		epsgDatabase[code] = projection
	}

	if err := scanner.Err(); err != nil {
		glog.Fatal(err)
	}

	return &epsgDatabase
}

func parseEPSGProjectionDatabaseRecord(databaseRecord string) (int, *epsgProjection) {
	tokens := strings.Split(databaseRecord, "\t")
	code, err := strconv.Atoi(strings.Replace(tokens[0], "EPSG:", "", -1))
	if err != nil {
		glog.Fatal("error while parsing the epsg projection file", err)
	}
	desc := tokens[1]
	proj4 := tokens[2]

	return code, &epsgProjection{
		EpsgCode:    code,
		Description: desc,
		Proj4:       proj4,
	}
}

// Converts the given coordinate from the given source Srid to the given target srid.
func (cc *proj4CoordinateConverter) ConvertCoordinateSrid(sourceSrid int, targetSrid int, coord geometry.Coordinate) (geometry.Coordinate, error) {
	if sourceSrid == targetSrid {
		return coord, nil
	}

	src, err := cc.initProjection(sourceSrid)
	if err != nil {
		glog.Infoln(err)
		return coord, err
	}

	dst, err := cc.initProjection(targetSrid)
	if err != nil {
		glog.Infoln(err)
		return coord, err
	}

	var converted, result = executeConversion(&coord, src, dst)

	return *converted, result
}

// Converts the generic bounding box bounds values from the given input srid to a EPSG:4326 srid (in radians)
// and returns a float64 array containing xMin, yMin, xMax, yMax, zMin, zMax. Z values are left unchanged
func (cc *proj4CoordinateConverter) Convert2DBoundingboxToWGS84Region(bbox *geometry.BoundingBox, srid int) (*geometry.BoundingBox, error) {
	z := float64(0)
	projLowCorn := geometry.Coordinate{
		X: bbox.Xmin,
		Y: bbox.Ymin,
		Z: z,
	}
	projUppCorn := geometry.Coordinate{
		X: bbox.Xmax,
		Y: bbox.Ymax,
		Z: z,
	}
	w84lc, err := cc.ConvertCoordinateSrid(srid, 4326, projLowCorn)
	if err != nil {
		glog.Infoln(err)
		return nil, nil
	}
	w84uc, err := cc.ConvertCoordinateSrid(srid, 4326, projUppCorn)
	if err != nil {
		glog.Infoln(err)
		return nil, nil
	}

	return geometry.NewBoundingBox(w84lc.X*toRadians, w84lc.Y*toRadians, w84uc.X*toRadians, w84uc.Y*toRadians, bbox.Zmin, bbox.Zmax), nil
}

// Converts the input coordinate from the given srid to EPSG:4326 srid
func (cc *proj4CoordinateConverter) ConvertToWGS84Cartesian(coord geometry.Coordinate, sourceSrid int) (geometry.Coordinate, error) {
	if sourceSrid == 4978 {
		return coord, nil
	}

	res, err := cc.ConvertCoordinateSrid(sourceSrid, 4326, coord)
	if err != nil {
		glog.Infoln(err)
		return coord, err
	}
	res2, err := cc.ConvertCoordinateSrid(4329, 4978, res)
	return res2, err
}

// Releases all projection objects from memory
func (cc *proj4CoordinateConverter) Cleanup() {
	for _, val := range cc.EpsgDatabase {
		if val.Projection != nil {
			val.Projection.Close()
		}
	}
}

func executeConversion(coord *geometry.Coordinate, sourceProj *proj.Proj, destinationProj *proj.Proj) (*geometry.Coordinate, error) {
	var x, y, z = getCoordinateArraysForConversion(coord, sourceProj)

	var err = proj.TransformRaw(sourceProj, destinationProj, x, y, z)

	var converted = geometry.Coordinate{
		X: getCoordinateFromRadiansToSridFormat(x[0], destinationProj),
		Y: getCoordinateFromRadiansToSridFormat(y[0], destinationProj),
		Z: extractZPointerIfPresent(z),
	}

	return &converted, err
}

// From a input Coordinate object and associated Proj object, return a set of arrays to be used for coordinate coversion
func getCoordinateArraysForConversion(coord *geometry.Coordinate, srid *proj.Proj) ([]float64, []float64, []float64) {
	var x, y, z []float64

	x = []float64{*getCoordinateInRadiansFromSridFormat(coord.X, srid)}
	y = []float64{*getCoordinateInRadiansFromSridFormat(coord.Y, srid)}

	if coord.Z != math.NaN() {
		z = []float64{coord.Z}
	}

	return x, y, z
}

// Returns the input coordinate expressed in the given srid converting it into radians if necessary
func getCoordinateInRadiansFromSridFormat(coord float64, srid *proj.Proj) *float64 {
	var radians = coord

	if srid.IsLatLong() {
		radians = coord * toRadians
	}

	return &radians
}

func extractZPointerIfPresent(zContainer []float64) float64 {
	if zContainer != nil {
		return zContainer[0]
	}

	return math.NaN()
}

// Returns the input coordinate expressed in the given srid converting it into radians if necessary
func getCoordinateFromRadiansToSridFormat(coord float64, srid *proj.Proj) float64 {
	var angle = coord

	if srid.IsLatLong() {
		angle = coord * toDeg
	}

	return angle
}

// Returns the projection corresponding to the given EPSG code, storing it in the relevant EpsgDatabase entry for caching
func (cc *proj4CoordinateConverter) initProjection(code int) (*proj.Proj, error) {
	val, ok := cc.EpsgDatabase[code]
	if !ok {
		return &proj.Proj{}, errors.New("epsg code not found")
	} else if val.Projection == nil {
		projection, err := proj.InitPlus(val.Proj4)
		if err != nil {
			return &proj.Proj{}, errors.New("unable to init projection")
		}
		val.Projection = projection
	}
	return val.Projection, nil
}
