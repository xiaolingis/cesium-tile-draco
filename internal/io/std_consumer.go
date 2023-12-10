package io

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/converters"
	"github.com/mfbonfigli/gocesiumtiler/internal/data"
	"github.com/mfbonfigli/gocesiumtiler/internal/geometry"
	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/ply"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
	"github.com/mfbonfigli/gocesiumtiler/tools"
)

type StandardConsumer struct {
	coordinateConverter converters.CoordinateConverter
	refineMode          tiler.RefineMode
	draco               bool
	dracoEncoderPath    string
}

func NewStandardConsumer(coordinateConverter converters.CoordinateConverter, refineMode tiler.RefineMode, draco bool, dracoEncoderPath string) *StandardConsumer {
	return &StandardConsumer{
		coordinateConverter: coordinateConverter,
		refineMode:          refineMode,
		draco:               draco,
		dracoEncoderPath:    dracoEncoderPath,
	}
}

// struct used to store data in an intermediate format
type intermediateData struct {
	coords          []float64
	colors          []uint8
	intensities     []uint8
	classifications []uint8
	numPoints       int
}

// Continually consumes WorkUnits submitted to a work channel producing corresponding content.pnts files and tileset.json files
// continues working until work channel is closed or if an error is raised. In this last case submits the error to an error
// channel before quitting
func (c *StandardConsumer) Consume(workchan chan *WorkUnit, errchan chan error, waitGroup *sync.WaitGroup) {
	for {
		// get work from channel
		work, ok := <-workchan
		if !ok {
			// channel was closed by producer, quit infinite loop
			break
		}

		// do work
		err := c.doWork(work)

		// if there were errors during work send in error channel and quit
		if err != nil {
			errchan <- err
			fmt.Println("exception in c worker")
			break
		}
	}

	// signal waitgroup finished work
	waitGroup.Done()
}

// Takes a workunit and writes the corresponding content.pnts and tileset.json files
func (c *StandardConsumer) doWork(workUnit *WorkUnit) error {
	// writes the content.pnts file
	if c.draco {
		err := c.writeBinaryPntsFileWithDraco(*workUnit)
		if err != nil {
			return err
		}
	} else {
		err := c.writeBinaryPntsFile(*workUnit)
		if err != nil {
			return err
		}
	}

	if !workUnit.Node.IsLeaf() || workUnit.Node.IsRoot() {
		// if the node has children also writes the tileset.json file
		err := c.writeTilesetJsonFile(*workUnit)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *StandardConsumer) invokeDracoEncoder(
	programLocation, plyInputFileLocation, outputFileLocation string, compressionLevel int,
) error {
	//startTime := time.Now()
	cmdParams := []string{
		"-point_cloud",
		"-i", plyInputFileLocation,
		"-o", outputFileLocation,
		"-qp", strconv.Itoa(11),
		"-cl", strconv.Itoa(compressionLevel),
	}

	runCmd := exec.Command(programLocation, cmdParams...)
	//log.Println("start run draco_encoder cmd", runCmd.String())

	var cmdStdout, cmdStderr bytes.Buffer
	runCmd.Stdout = &cmdStdout
	runCmd.Stderr = &cmdStderr

	if err := runCmd.Run(); err != nil {
		log.Println("run failed", runCmd.String(), "cmd-stdout", cmdStdout.String(), "cmd-stderr", cmdStderr.String(), err.Error())
		return err
	}
	//log.Println("run draco_encoder cmd success", runCmd.String(), "latency_ms", time.Since(startTime).Milliseconds())
	return nil
}

func (c *StandardConsumer) writeBinaryPntsFileWithDraco(workUnit WorkUnit) error {
	parentFolder := workUnit.BasePath
	node := workUnit.Node

	// Create base folder if it does not exist
	err := tools.CreateDirectoryIfDoesNotExist(parentFolder)
	if err != nil {
		return err
	}

	intermediatePointData, err := c.generateIntermediateDataForPnts(node)
	if err != nil {
		return err
	}

	// Evaluating average X, Y, Z to express coords relative to tile center
	averageXYZ := c.computeAverageXYZ(intermediatePointData)

	// Normalizing coordinates relative to average
	c.subtractXYZFromIntermediateDataCoords(intermediatePointData, averageXYZ)

	// write ply file
	plyFileName := "content.ply"
	plyFilePath := path.Join(parentFolder, plyFileName)
	if err := c.writePlyFile(plyFilePath, intermediatePointData); err != nil {
		log.Println("Wrote PLY failed.", err.Error())
		return err
	}

	// generate Draco Encoder binary
	//programLocation := "/workerdir/gocesiumtiler/draco_encoder"
	programLocation := c.dracoEncoderPath
	plyInputFileLocation := path.Join(parentFolder, plyFileName)
	outputFileName := "content.drc"
	drcFilePath := path.Join(parentFolder, outputFileName)
	compressionLevel := 7
	if err := c.invokeDracoEncoder(programLocation, plyInputFileLocation, drcFilePath, compressionLevel); err != nil {
		log.Println("invokeDracoEncoder failed.", err.Error())
		return err
	}

	dracoContent, err := ioutil.ReadFile(drcFilePath)
	if err != nil {
		fmt.Printf("file error:%s\n", err.Error())
	}
	//fmt.Println("ReadFile success")

	// Feature table
	featureTableStr := c.generateFeatureTableJsonContentWithDraco(
		averageXYZ[0], averageXYZ[1], averageXYZ[2], intermediatePointData.numPoints, 0, len(dracoContent),
	)
	featureTableLen := len(featureTableStr)
	outputByte := c.generatePntsByteArrayWithDraco([]byte(featureTableStr), featureTableLen, []byte{}, 0, dracoContent, len(dracoContent))

	//fmt.Println("generate from generatePntsByteArrayWithDraco")

	// Write binary content to file
	pntsFilePath := path.Join(parentFolder, "content.pnts")
	err = ioutil.WriteFile(pntsFilePath, outputByte, 0777)

	if err != nil {
		return err
	}

	// Delete temporary ply file
	if err := os.Remove(plyFilePath); err != nil {
		log.Println("delete temporary drc file failed.", err.Error())
	}
	// Delete temporary drc file
	if err := os.Remove(drcFilePath); err != nil {
		log.Println("delete temporary drc file failed.", err.Error())
	}

	return nil
}

func (c *StandardConsumer) writePlyFile(filePath string, intermediatePointData *intermediateData) error {
	// generate vertex info
	length := intermediatePointData.numPoints
	verts := make([]ply.Vertex, length)
	for i := 0; i < intermediatePointData.numPoints; i++ {
		verts[i] = ply.Vertex{
			X: float32(intermediatePointData.coords[i*3]),
			Y: float32(intermediatePointData.coords[i*3+1]),
			Z: float32(intermediatePointData.coords[i*3+2]),
			R: intermediatePointData.colors[i*3],
			G: intermediatePointData.colors[i*3+1],
			B: intermediatePointData.colors[i*3+2],
		}
	}

	return ply.WritePlyFile(filePath, verts)
}

// Writes a content.pnts binary files from the given WorkUnit
func (c *StandardConsumer) writeBinaryPntsFile(workUnit WorkUnit) error {
	parentFolder := workUnit.BasePath
	node := workUnit.Node

	// Create base folder if it does not exist
	err := tools.CreateDirectoryIfDoesNotExist(parentFolder)
	if err != nil {
		return err
	}

	intermediatePointData, err := c.generateIntermediateDataForPnts(node)
	if err != nil {
		return err
	}

	// Evaluating average X, Y, Z to express coords relative to tile center
	averageXYZ := c.computeAverageXYZ(intermediatePointData)

	// Normalizing coordinates relative to average
	c.subtractXYZFromIntermediateDataCoords(intermediatePointData, averageXYZ)

	// Coordinate bytes
	positionBytes := tools.ConvertTruncateFloat64ToFloat32ByteArray(intermediatePointData.coords)

	// Feature table
	featureTableBytes, featureTableLen := c.generateFeatureTable(averageXYZ[0], averageXYZ[1], averageXYZ[2], intermediatePointData.numPoints)

	// Batch table
	batchTableBytes, batchTableLen := c.generateBatchTable(intermediatePointData.numPoints)

	// Appending binary content to slice
	outputByte := c.generatePntsByteArray(intermediatePointData, positionBytes, featureTableBytes, featureTableLen, batchTableBytes, batchTableLen)

	// Write binary content to file
	pntsFilePath := path.Join(parentFolder, "content.pnts")
	err = ioutil.WriteFile(pntsFilePath, outputByte, 0777)

	if err != nil {
		return err
	}

	return nil
}

func (c *StandardConsumer) generateIntermediateDataForPnts(node *grid_tree.GridNode) (*intermediateData, error) {
	points := node.GetPoints()

	if c.refineMode == tiler.RefineModeReplace {
		points = appendParentPoints(node, points)
	}

	numPoints := len(points)
	intermediateData := intermediateData{
		coords:          make([]float64, numPoints*3),
		colors:          make([]uint8, numPoints*3),
		intensities:     make([]uint8, numPoints),
		classifications: make([]uint8, numPoints),
		numPoints:       numPoints,
	}

	// Decomposing tile data properties in separate sublists for coords, colors, intensities and classifications
	for i := 0; i < len(points); i++ {
		point := points[i]
		if point == nil {
			fmt.Println("a")
		}
		srcCoord := geometry.Coordinate{
			X: point.X,
			Y: point.Y,
			Z: point.Z,
		}

		// ConvertCoordinateSrid coords according to cesium CRS
		outCrd, err := c.coordinateConverter.ConvertToWGS84Cartesian(srcCoord, node.GetInternalSrid())
		if err != nil {
			log.Println(err)
			return nil, err
		}

		intermediateData.coords[i*3] = outCrd.X
		intermediateData.coords[i*3+1] = outCrd.Y
		intermediateData.coords[i*3+2] = outCrd.Z

		intermediateData.colors[i*3] = point.R
		intermediateData.colors[i*3+1] = point.G
		intermediateData.colors[i*3+2] = point.B

		intermediateData.intensities[i] = point.Intensity
		intermediateData.classifications[i] = point.Classification
	}

	return &intermediateData, nil
}

func appendParentPoints(node *grid_tree.GridNode, points []*data.Point) []*data.Point {
	parent := node.GetParent()
	boundingBox := node.GetBoundingBox()
	isContained := func(point *data.Point) bool {
		if point.X >= boundingBox.Xmin && point.X <= boundingBox.Xmax &&
			point.Y >= boundingBox.Ymin && point.Y <= boundingBox.Ymax &&
			point.Z >= boundingBox.Zmin && point.Z <= boundingBox.Zmax {
			return true
		}
		return false
	}

	for parent != nil {
		for _, point := range parent.GetPoints() {
			if isContained(point) {
				points = append(points, point)
			}
		}
		parent = parent.GetParent()
	}

	return points
}

func (c *StandardConsumer) generateFeatureTable(avgX float64, avgY float64, avgZ float64, numPoints int) ([]byte, int) {
	featureTableStr := c.generateFeatureTableJsonContent(avgX, avgY, avgZ, numPoints, 0)
	featureTableLen := len(featureTableStr)
	return []byte(featureTableStr), featureTableLen
}

func (c *StandardConsumer) generateBatchTable(numPoints int) ([]byte, int) {
	batchTableStr := c.generateBatchTableJsonContent(numPoints, 0)
	batchTableLen := len(batchTableStr)
	return []byte(batchTableStr), batchTableLen
}

func (c *StandardConsumer) generatePntsByteArray(intermediateData *intermediateData, positionBytes []byte, featureTableBytes []byte, featureTableLen int, batchTableBytes []byte, batchTableLen int) []byte {
	outputByte := make([]byte, 0)
	outputByte = append(outputByte, []byte("pnts")...)                 // magic
	outputByte = append(outputByte, tools.ConvertIntToByteArray(1)...) // version number
	byteLength := 28 + featureTableLen + len(positionBytes) + len(intermediateData.colors)
	outputByte = append(outputByte, tools.ConvertIntToByteArray(byteLength)...)
	outputByte = append(outputByte, tools.ConvertIntToByteArray(featureTableLen)...)                                                         // feature table length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(len(positionBytes)+len(intermediateData.colors))...)                         // feature table binary length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(batchTableLen)...)                                                           // batch table length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(len(intermediateData.intensities)+len(intermediateData.classifications))...) // batch table binary length
	outputByte = append(outputByte, featureTableBytes...)                                                                                    // feature table
	outputByte = append(outputByte, positionBytes...)                                                                                        // positions array
	outputByte = append(outputByte, intermediateData.colors...)                                                                              // colors array
	outputByte = append(outputByte, batchTableBytes...)                                                                                      // batch table
	outputByte = append(outputByte, intermediateData.intensities...)                                                                         // intensities array
	outputByte = append(outputByte, intermediateData.classifications...)

	return outputByte
}

func (c *StandardConsumer) generatePntsByteArrayWithDraco(
	featureTableBytes []byte, featureTableLen int, batchTableBytes []byte, batchTableLen int, dracoBytes []byte, dracoByteLength int,
) []byte {
	outputByte := make([]byte, 0)
	outputByte = append(outputByte, []byte("pnts")...)                 // magic
	outputByte = append(outputByte, tools.ConvertIntToByteArray(1)...) // version number
	byteLength := 28 + featureTableLen + dracoByteLength
	outputByte = append(outputByte, tools.ConvertIntToByteArray(byteLength)...)
	outputByte = append(outputByte, tools.ConvertIntToByteArray(featureTableLen)...) // feature table length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(dracoByteLength)...) // feature table binary length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(batchTableLen)...)   // batch table length
	outputByte = append(outputByte, tools.ConvertIntToByteArray(0)...)               // batch table binary length
	outputByte = append(outputByte, featureTableBytes...)                            // feature table
	outputByte = append(outputByte, batchTableBytes...)                              // batch table
	outputByte = append(outputByte, dracoBytes...)                                   // 3DTILES_draco_point_compression

	return outputByte
}

func (c *StandardConsumer) computeAverageXYZ(intermediatePointData *intermediateData) []float64 {
	var avgX, avgY, avgZ float64

	for i := 0; i < intermediatePointData.numPoints; i++ {
		avgX = avgX + intermediatePointData.coords[i*3]
		avgY = avgY + intermediatePointData.coords[i*3+1]
		avgZ = avgZ + intermediatePointData.coords[i*3+2]
	}
	avgX /= float64(intermediatePointData.numPoints)
	avgY /= float64(intermediatePointData.numPoints)
	avgZ /= float64(intermediatePointData.numPoints)

	return []float64{avgX, avgY, avgZ}
}

func (c *StandardConsumer) subtractXYZFromIntermediateDataCoords(intermediatePointData *intermediateData, xyz []float64) {
	for i := 0; i < intermediatePointData.numPoints; i++ {
		intermediatePointData.coords[i*3] -= xyz[0]
		intermediatePointData.coords[i*3+1] -= xyz[1]
		intermediatePointData.coords[i*3+2] -= xyz[2]
	}
}

func (c *StandardConsumer) generateFeatureTableJsonContentWithDraco(x, y, z float64, pointNo int, spaceNo int, dracoByteLength int) string {
	sb := ""
	sb += "{\"POINTS_LENGTH\":" + strconv.Itoa(pointNo) + ","
	sb += "\"RTC_CENTER\":[" + fmt.Sprintf("%f", x) + strings.Repeat("0", spaceNo)
	sb += "," + fmt.Sprintf("%f", y) + "," + fmt.Sprintf("%f", z) + "],"
	sb += "\"POSITION\":" + "{\"byteOffset\":" + "0" + "},"
	sb += "\"RGB\":" + "{\"byteOffset\":" + "0" + "},"
	sb += "\"extensions\":" + "{\"3DTILES_draco_point_compression\":{\"byteLength\":" + strconv.Itoa(dracoByteLength) + ",\"byteOffset\":0,\"properties\":{\"POSITION\":0,\"RGB\":1}}}}"
	headerByteLength := len([]byte(sb))
	paddingSize := headerByteLength % 4
	if paddingSize != 0 {
		return c.generateFeatureTableJsonContentWithDraco(x, y, z, pointNo, 4-paddingSize, dracoByteLength)
	}
	return sb
}

// Generates the json representation of the feature table
func (c *StandardConsumer) generateFeatureTableJsonContent(x, y, z float64, pointNo int, spaceNo int) string {
	sb := ""
	sb += "{\"POINTS_LENGTH\":" + strconv.Itoa(pointNo) + ","
	sb += "\"RTC_CENTER\":[" + fmt.Sprintf("%f", x) + strings.Repeat("0", spaceNo)
	sb += "," + fmt.Sprintf("%f", y) + "," + fmt.Sprintf("%f", z) + "],"
	sb += "\"POSITION\":" + "{\"byteOffset\":" + "0" + "},"
	sb += "\"RGB\":" + "{\"byteOffset\":" + strconv.Itoa(pointNo*12) + "}}"
	headerByteLength := len([]byte(sb))
	paddingSize := headerByteLength % 4
	if paddingSize != 0 {
		return c.generateFeatureTableJsonContent(x, y, z, pointNo, 4-paddingSize)
	}
	return sb
}

// Generates the json representation of the batch table
func (c *StandardConsumer) generateBatchTableJsonContent(pointNumber, spaceNumber int) string {
	sb := ""
	sb += "{\"INTENSITY\":" + "{\"byteOffset\":" + "0" + ", \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},"
	sb += "\"CLASSIFICATION\":" + "{\"byteOffset\":" + strconv.Itoa(pointNumber) + ", \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	sb += strings.Repeat(" ", spaceNumber)
	headerByteLength := len([]byte(sb))
	paddingSize := headerByteLength % 4
	if paddingSize != 0 {
		return c.generateBatchTableJsonContent(pointNumber, 4-paddingSize)
	}
	return sb
}

// Writes the tileset.json file for the given WorkUnit
func (c *StandardConsumer) writeTilesetJsonFile(workUnit WorkUnit) error {
	parentFolder := workUnit.BasePath
	node := workUnit.Node

	// Create base folder if it does not exist
	err := tools.CreateDirectoryIfDoesNotExist(parentFolder)
	if err != nil {
		return err
	}

	// tileset.json file
	file := path.Join(parentFolder, "tileset.json")
	jsonData, err := c.generateTilesetJson(node)
	if err != nil {
		return err
	}

	// Writes the tileset.json binary content to the given file
	err = ioutil.WriteFile(file, jsonData, 0666)
	if err != nil {
		return err
	}

	return nil
}

// Generates the tileset.json content for the given tree node
func (c *StandardConsumer) generateTilesetJson(node *grid_tree.GridNode) ([]byte, error) {
	if !node.IsLeaf() || node.IsRoot() {
		root, err := c.generateTilesetRoot(node)
		if err != nil {
			return nil, err
		}

		tileset := *c.generateTileset(node, root)

		// Outputting a formatted json file
		e, err := json.MarshalIndent(tileset, "", "\t")
		if err != nil {
			return nil, err
		}

		return e, nil
	}

	return nil, errors.New("this node is a leaf, cannot create a tileset json for it")
}

func (c *StandardConsumer) generateTilesetRoot(node *grid_tree.GridNode) (*Root, error) {
	reg, err := node.GetBoundingBoxRegion(c.coordinateConverter)

	if err != nil {
		return nil, err
	}

	children, err := c.generateTilesetChildren(node)
	if err != nil {
		return nil, err
	}

	root := Root{
		Content:        Content{"content.pnts"},
		BoundingVolume: BoundingVolume{reg.GetAsArray()},
		GeometricError: node.ComputeGeometricError(),
		Refine:         c.refineMode.String(),
		Children:       children,
	}

	return &root, nil
}

func (c *StandardConsumer) generateTileset(node *grid_tree.GridNode, root *Root) *Tileset {
	tileset := Tileset{}
	tileset.Asset = Asset{Version: "1.0"}
	tileset.GeometricError = node.ComputeGeometricError()
	tileset.Root = *root

	return &tileset
}

func (c *StandardConsumer) generateTilesetChildren(node *grid_tree.GridNode) ([]Child, error) {
	var children []Child
	for i, child := range node.GetChildren() {
		if c.nodeContainsPoints(child) {
			childJson, err := c.generateTilesetChild(child, i, node)
			if err != nil {
				return nil, err
			}
			children = append(children, *childJson)
		}
	}
	return children, nil
}

func (c *StandardConsumer) nodeContainsPoints(node *grid_tree.GridNode) bool {
	return node != nil && node.TotalNumberOfPoints() > 0
}

func (c *StandardConsumer) generateTilesetChild(child *grid_tree.GridNode, childIndex int, parent *grid_tree.GridNode) (*Child, error) {
	childJson := Child{}
	filename := "tileset.json"
	if child.IsLeaf() {
		filename = "content.pnts"
	}

	childrenPath := parent.GetChildrenPath()
	childPath := childrenPath[childIndex]
	// sort "74520" to "02457" for merge_children case
	if len(childPath) > 1 {
		childList := []byte(childPath)
		sort.Slice(childList, func(i, j int) bool { return childList[i] < childList[j] })
		childPath = string(childList)
	}

	childJson.Content = Content{
		Url: childPath + "/" + filename,
	}
	reg, err := child.GetBoundingBoxRegion(c.coordinateConverter)
	if err != nil {
		return nil, err
	}
	childJson.BoundingVolume = BoundingVolume{
		Region: reg.GetAsArray(),
	}
	childJson.GeometricError = child.ComputeGeometricError()
	childJson.Refine = c.refineMode.String()
	return &childJson, nil
}
