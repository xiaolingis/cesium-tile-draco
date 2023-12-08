package unit

import (
	"encoding/binary"
	"encoding/json"
	"github.com/ecopia-map/cesium_tiler/internal/converters/coordinate/proj4_coordinate_converter"
	"github.com/ecopia-map/cesium_tiler/internal/data"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
	"github.com/ecopia-map/cesium_tiler/internal/io"
	"github.com/ecopia-map/cesium_tiler/internal/octree"
	"github.com/ecopia-map/cesium_tiler/internal/tiler"
	"github.com/ecopia-map/cesium_tiler/tools"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sync"
	"testing"
)

func TestConsumerSinglePointNoChildrenEPSG4326(t *testing.T) {
	// generate mock node with one point and no children
	node := &mockNode{
		boundingBox: geometry.NewBoundingBox(13.7995147, 13.7995147, 42.3306312, 42.3306312, 0, 1),
		points: []*data.Point{
			data.NewPoint(13.7995147, 42.3306312, 1, 1, 2, 3, 4, 5),
		},
		depth:               1,
		internalSrid:        4326,
		globalChildrenCount: 2,
		localChildrenCount:  1,
		opts: &tiler.TilerOptions{
			Srid: 4326,
		},
	}

	// generate a temp dir and defer its deletion
	tempdir, _ := ioutil.TempDir(tools.GetRootFolder(), "temp*")
	defer func() { _ = os.RemoveAll(tempdir) }()

	// generate a mock workunit
	workUnit := io.WorkUnit{
		Node:     node,
		Opts:     node.opts,
		BasePath: tempdir,
	}

	// create workChannel and errorChannel
	workChannel := make(chan *io.WorkUnit, 1)
	errorChannel := make(chan error)

	// create waitGroup and add consumer
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	// start consumer
	consumer := io.NewStandardConsumer(proj4_coordinate_converter.NewProj4CoordinateConverter(), tiler.RefineModeAdd)
	go consumer.Consume(workChannel, errorChannel, &waitGroup)

	// inject work unit in channel
	workChannel <- &workUnit

	// close workchannel
	close(workChannel)

	// wait consumer to finish
	waitGroup.Wait()

	// close error channel
	close(errorChannel)

	for err := range errorChannel {
		t.Errorf("Unexpected error found in error channel: %s", err.Error())
	}

	// read tileset.json and validate its content
	jsonFile, err := os.Open(path.Join(tempdir, "tileset.json"))
	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() { _ = jsonFile.Close() }()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result io.Tileset
	_ = json.Unmarshal([]byte(byteValue), &result)

	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}
	if result.Asset.Version != "1.0" {
		t.Errorf("Expected asset version %s, got %s", "1.0", result.Asset.Version)
	}
	if result.GeometricError != 0 {
		t.Errorf("Expected geometricError %f, got %f", 0.0, result.GeometricError)
	}
	if len(result.Root.Children) != 0 {
		t.Errorf("Expected root children number %d, got %d", 0, len(result.Root.Children))
	}
	if result.Root.Content.Url != "content.pnts" {
		t.Errorf("Expected root content uri %s, got %s", "content.pnts", result.Root.Content.Url)
	}
	if result.Root.BoundingVolume.Region[0] != 0.24084696669235753 {
		t.Errorf("Different region min x coordinate")
	}
	if result.Root.BoundingVolume.Region[1] != 0.7388088888874382 {
		t.Errorf("Different region min y coordinate")
	}
	if result.Root.BoundingVolume.Region[2] != 0.24084696669235753 {
		t.Errorf("Different region max x coordinate")
	}
	if result.Root.BoundingVolume.Region[3] != 0.7388088888874382 {
		t.Errorf("Different region max y coordinate")
	}
	if result.Root.BoundingVolume.Region[4] != 0.0 {
		t.Errorf("Different region min z coordinate")
	}
	if result.Root.BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different region max z coordinate")
	}
	if result.Root.GeometricError != 0.0 {
		t.Errorf("Expected Root GeometricError %f, got %f", 0.0, result.Root.GeometricError)
	}
	if result.Root.Refine != "ADD" {
		t.Errorf("Expected Refine type %s, got %s", "ADD", result.Root.Refine)
	}

	pntsFile, err := os.Open(path.Join(tempdir, "content.pnts"))
	defer func() { _ = pntsFile.Close() }()

	if err != nil {
		t.Errorf("Error opening content.pnts: %s", err.Error())
	}

	var buffer = []byte{0, 0, 0, 0}
	_, err = pntsFile.Read(buffer)
	if err != nil {
		t.Errorf("Error reading magic bytes from content.pnts: %s", err.Error())
	}

	var magicString = string(buffer)
	if magicString != "pnts" {
		t.Errorf("Expected magic value: %s, got: %s", "pnts", magicString)
	}

	_, err = pntsFile.Read(buffer)
	var version = binary.LittleEndian.Uint32(buffer)
	if version != 1 {
		t.Errorf("Expected version value: %d, got: %d", 1, version)
	}

	_, err = pntsFile.Read(buffer)
	var length = binary.LittleEndian.Uint32(buffer)
	if length != 175 {
		t.Errorf("Expected len value: %d, got: %d", 175, length)
	}

	_, err = pntsFile.Read(buffer)
	var featureTableLength = binary.LittleEndian.Uint32(buffer)
	if featureTableLength != 132 {
		t.Errorf("Expected featureTableLength value: %d, got: %d", 132, featureTableLength)
	}

	_, err = pntsFile.Read(buffer)
	var positionPlusColors = binary.LittleEndian.Uint32(buffer)
	if positionPlusColors != 15 {
		t.Errorf("Expected position and color section length value: %d, got: %d", 15, positionPlusColors)
	}

	_, err = pntsFile.Read(buffer)
	var batchTableLen = binary.LittleEndian.Uint32(buffer)
	if batchTableLen != 164 {
		t.Errorf("Expected batch table length: %d, got: %d", 164, batchTableLen)
	}

	_, err = pntsFile.Read(buffer)
	var intensityAndClassificationLen = binary.LittleEndian.Uint32(buffer)
	if intensityAndClassificationLen != 2 {
		t.Errorf("Expected intensity and classification sections length: %d, got: %d", 2, intensityAndClassificationLen)
	}

	buffer = make([]byte, 132)
	_, err = pntsFile.Read(buffer)
	var featureTable = string(buffer)
	var expectedFeatureTable = "{\"POINTS_LENGTH\":1,\"RTC_CENTER\":[4586042.6311360,1126398.922751,4272825.711405],\"POSITION\":{\"byteOffset\":0},\"RGB\":{\"byteOffset\":12}}"
	if featureTable != expectedFeatureTable {
		t.Errorf("Expected feature table: \r\n %s \r\n Got: %s", expectedFeatureTable, featureTable)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != 0.0 {
		t.Errorf("Expected position X: %f  got: %f", 0.0, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != 0.0 {
		t.Errorf("Expected position Y: %f  got: %f", 0.0, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != 0.0 {
		t.Errorf("Expected position Z: %f  got: %f", 0.0, positionZ)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var red = buffer[0]
	if red != 1 {
		t.Errorf("Expected red: %d, got: %d", 1, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var green = buffer[0]
	if green != 2 {
		t.Errorf("Expected green: %d, got: %d", 2, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var blue = buffer[0]
	if blue != 3 {
		t.Errorf("Expected blue: %d, got: %d", 3, blue)
	}

	buffer = make([]byte, 164)
	_, err = pntsFile.Read(buffer)
	var batchTable = string(buffer)
	var expectedBatchTable = "{\"INTENSITY\":{\"byteOffset\":0, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},\"CLASSIFICATION\":{\"byteOffset\":1, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	if batchTable != expectedBatchTable {
		t.Errorf("Expected batch table: \r\n %s \r\n Got: %s", expectedBatchTable, batchTable)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var intensity = buffer[0]
	if intensity != 4 {
		t.Errorf("Expected blue: %d, got: %d", 4, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var classification = buffer[0]
	if classification != 5 {
		t.Errorf("Expected blue: %d, got: %d", 5, intensity)
	}
}

func TestConsumerSinglePointNoChildrenEPSG32633(t *testing.T) {
	// generate mock node with one point and no children
	node := &mockNode{
		boundingBox: geometry.NewBoundingBox(401094.30, 401094.30, 4687184.70, 4687184.70, 0, 1),
		points: []*data.Point{
			data.NewPoint(401094.30, 4687184.70, 1, 1, 2, 3, 4, 5),
		},
		depth:               1,
		internalSrid:        32633,
		globalChildrenCount: 2,
		localChildrenCount:  1,
		opts: &tiler.TilerOptions{
			Srid: 32633,
		},
	}

	// generate a temp dir and defer its deletion
	tempdir, _ := ioutil.TempDir(tools.GetRootFolder(), "temp*")
	defer func() { _ = os.RemoveAll(tempdir) }()

	// generate a mock workunit
	workUnit := io.WorkUnit{
		Node:     node,
		Opts:     node.opts,
		BasePath: tempdir,
	}

	// create workChannel and errorChannel
	workChannel := make(chan *io.WorkUnit, 1)
	errorChannel := make(chan error)

	// create waitGroup and add consumer
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	// start consumer
	consumer := io.NewStandardConsumer(proj4_coordinate_converter.NewProj4CoordinateConverter(), tiler.RefineModeAdd)
	go consumer.Consume(workChannel, errorChannel, &waitGroup)

	// inject work unit in channel
	workChannel <- &workUnit

	// close workchannel
	close(workChannel)

	// wait consumer to finish
	waitGroup.Wait()

	// close error channel
	close(errorChannel)

	for err := range errorChannel {
		t.Errorf("Unexpected error found in error channel: %s", err.Error())
	}

	// read tileset.json and validate its content
	jsonFile, err := os.Open(path.Join(tempdir, "tileset.json"))
	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() { _ = jsonFile.Close() }()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result io.Tileset
	_ = json.Unmarshal([]byte(byteValue), &result)

	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}
	if result.Asset.Version != "1.0" {
		t.Errorf("Expected asset version %s, got %s", "1.0", result.Asset.Version)
	}
	if result.GeometricError != 0 {
		t.Errorf("Expected geometricError %f, got %f", 0.0, result.GeometricError)
	}
	if len(result.Root.Children) != 0 {
		t.Errorf("Expected root children number %d, got %d", 0, len(result.Root.Children))
	}
	if result.Root.Content.Url != "content.pnts" {
		t.Errorf("Expected root content uri %s, got %s", "content.pnts", result.Root.Content.Url)
	}
	if result.Root.BoundingVolume.Region[0] != 0.2408469662404639 {
		t.Errorf("Different region min x coordinate")
	}
	if result.Root.BoundingVolume.Region[1] != 0.7388088889592584 {
		t.Errorf("Different region min y coordinate")
	}
	if result.Root.BoundingVolume.Region[2] != 0.2408469662404639 {
		t.Errorf("Different region max x coordinate")
	}
	if result.Root.BoundingVolume.Region[3] != 0.7388088889592584 {
		t.Errorf("Different region max y coordinate")
	}
	if result.Root.BoundingVolume.Region[4] != 0.0 {
		t.Errorf("Different region min z coordinate")
	}
	if result.Root.BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different region max z coordinate")
	}
	if result.Root.GeometricError != 0.0 {
		t.Errorf("Expected Root GeometricError %f, got %f", 0.0, result.Root.GeometricError)
	}
	if result.Root.Refine != "ADD" {
		t.Errorf("Expected Refine type %s, got %s", "ADD", result.Root.Refine)
	}

	pntsFile, err := os.Open(path.Join(tempdir, "content.pnts"))
	defer func() { _ = pntsFile.Close() }()

	if err != nil {
		t.Errorf("Error opening content.pnts: %s", err.Error())
	}

	var buffer = []byte{0, 0, 0, 0}
	_, err = pntsFile.Read(buffer)
	if err != nil {
		t.Errorf("Error reading magic bytes from content.pnts: %s", err.Error())
	}

	var magicString = string(buffer)
	if magicString != "pnts" {
		t.Errorf("Expected magic value: %s, got: %s", "pnts", magicString)
	}

	_, err = pntsFile.Read(buffer)
	var version = binary.LittleEndian.Uint32(buffer)
	if version != 1 {
		t.Errorf("Expected version value: %d, got: %d", 1, version)
	}

	_, err = pntsFile.Read(buffer)
	var length = binary.LittleEndian.Uint32(buffer)
	if length != 175 {
		t.Errorf("Expected len value: %d, got: %d", 175, length)
	}

	_, err = pntsFile.Read(buffer)
	var featureTableLength = binary.LittleEndian.Uint32(buffer)
	if featureTableLength != 132 {
		t.Errorf("Expected featureTableLength value: %d, got: %d", 132, featureTableLength)
	}

	_, err = pntsFile.Read(buffer)
	var positionPlusColors = binary.LittleEndian.Uint32(buffer)
	if positionPlusColors != 15 {
		t.Errorf("Expected position and color section length value: %d, got: %d", 15, positionPlusColors)
	}

	_, err = pntsFile.Read(buffer)
	var batchTableLen = binary.LittleEndian.Uint32(buffer)
	if batchTableLen != 164 {
		t.Errorf("Expected batch table length: %d, got: %d", 164, batchTableLen)
	}

	_, err = pntsFile.Read(buffer)
	var intensityAndClassificationLen = binary.LittleEndian.Uint32(buffer)
	if intensityAndClassificationLen != 2 {
		t.Errorf("Expected intensity and classification sections length: %d, got: %d", 2, intensityAndClassificationLen)
	}

	buffer = make([]byte, 132)
	_, err = pntsFile.Read(buffer)
	var featureTable = string(buffer)
	var expectedFeatureTable = "{\"POINTS_LENGTH\":1,\"RTC_CENTER\":[4586042.6313460,1126398.920605,4272825.711743],\"POSITION\":{\"byteOffset\":0},\"RGB\":{\"byteOffset\":12}}"
	if featureTable != expectedFeatureTable {
		t.Errorf("Expected feature table: \r\n %s \r\n Got: %s", expectedFeatureTable, featureTable)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != 0.0 {
		t.Errorf("Expected position X: %f  got: %f", 0.0, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != 0.0 {
		t.Errorf("Expected position Y: %f  got: %f", 0.0, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != 0.0 {
		t.Errorf("Expected position Z: %f  got: %f", 0.0, positionZ)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var red = buffer[0]
	if red != 1 {
		t.Errorf("Expected red: %d, got: %d", 1, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var green = buffer[0]
	if green != 2 {
		t.Errorf("Expected green: %d, got: %d", 2, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var blue = buffer[0]
	if blue != 3 {
		t.Errorf("Expected blue: %d, got: %d", 3, blue)
	}

	buffer = make([]byte, 164)
	_, err = pntsFile.Read(buffer)
	var batchTable = string(buffer)
	var expectedBatchTable = "{\"INTENSITY\":{\"byteOffset\":0, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},\"CLASSIFICATION\":{\"byteOffset\":1, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	if batchTable != expectedBatchTable {
		t.Errorf("Expected batch table: \r\n %s \r\n Got: %s", expectedBatchTable, batchTable)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var intensity = buffer[0]
	if intensity != 4 {
		t.Errorf("Expected blue: %d, got: %d", 4, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var classification = buffer[0]
	if classification != 5 {
		t.Errorf("Expected blue: %d, got: %d", 5, intensity)
	}
}

func TestConsumerOneChild(t *testing.T) {
	// generate mock node with one point and no children
	node := &mockNode{
		boundingBox: geometry.NewBoundingBox(13.7995147, 13.7995147, 42.3306312, 42.3306312, 0, 1),
		points: []*data.Point{
			data.NewPoint(13.7995147, 42.3306312, 1, 1, 2, 3, 4, 5),
		},
		depth:               1,
		globalChildrenCount: 2,
		internalSrid:        4326,
		localChildrenCount:  1,
		opts: &tiler.TilerOptions{
			Srid: 4326,
		},
		children: [8]octree.INode{
			&mockNode{
				boundingBox: geometry.NewBoundingBox(13.7995147, 13.7995147, 42.3306312, 42.3306312, 0.5, 1),
				points: []*data.Point{
					data.NewPoint(13.7995147, 42.3306312, 1, 4, 5, 6, 4, 5),
				},
				depth:               1,
				internalSrid:        4326,
				globalChildrenCount: 1,
				localChildrenCount:  1,
				opts: &tiler.TilerOptions{
					Srid: 4326,
				},
			},
		},
	}

	// generate a temp dir and defer its deletion
	tempdir, _ := ioutil.TempDir(tools.GetRootFolder(), "temp*")
	defer func() { _ = os.RemoveAll(tempdir) }()

	// generate a mock workunit
	workUnit := io.WorkUnit{
		Node:     node,
		Opts:     node.opts,
		BasePath: tempdir,
	}

	// create workChannel and errorChannel
	workChannel := make(chan *io.WorkUnit, 1)
	errorChannel := make(chan error)

	// create waitGroup and add consumer
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	// start consumer
	consumer := io.NewStandardConsumer(proj4_coordinate_converter.NewProj4CoordinateConverter(), tiler.RefineModeAdd)
	go consumer.Consume(workChannel, errorChannel, &waitGroup)

	// inject work unit in channel
	workChannel <- &workUnit

	// close workchannel
	close(workChannel)

	// wait consumer to finish
	waitGroup.Wait()

	// close error channel
	close(errorChannel)

	for err := range errorChannel {
		t.Errorf("Unexpected error found in error channel: %s", err.Error())
	}

	// read tileset.json and validate its content
	jsonFile, err := os.Open(path.Join(tempdir, "tileset.json"))
	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() { _ = jsonFile.Close() }()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result io.Tileset
	_ = json.Unmarshal([]byte(byteValue), &result)

	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}
	if result.Asset.Version != "1.0" {
		t.Errorf("Expected asset version %s, got %s", "1.0", result.Asset.Version)
	}
	if result.GeometricError != 0 {
		t.Errorf("Expected geometricError %f, got %f", 0.0, result.GeometricError)
	}
	if len(result.Root.Children) != 1 {
		t.Errorf("Expected root children number %d, got %d", 1, len(result.Root.Children))
	}
	if result.Root.Children[0].Content.Url != "0/tileset.json" {
		t.Errorf("Expected root children content url %s, got %s", "0/tileset.json", result.Root.Children[0].Content.Url)
	}
	if result.Root.Children[0].BoundingVolume.Region[0] != 0.24084696669235753 {
		t.Errorf("Different children region min x coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[1] != 0.7388088888874382 {
		t.Errorf("Different children region min y coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[2] != 0.24084696669235753 {
		t.Errorf("Different children region max x coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[3] != 0.7388088888874382 {
		t.Errorf("Different children region max y coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[4] != 0.5 {
		t.Errorf("Different children region min z coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different children region max z coordinate")
	}
	if result.Root.Children[0].GeometricError != 0.0 {
		t.Errorf("Expected child geometricError %f, got %f", 0.0, result.Root.Children[0].GeometricError)
	}
	if result.Root.Children[0].Refine != "ADD" {
		t.Errorf("Expected child geometricError %s, got %s", "ADD", result.Root.Children[0].Refine)
	}
	if result.Root.Content.Url != "content.pnts" {
		t.Errorf("Expected root content uri %s, got %s", "content.pnts", result.Root.Content.Url)
	}
	if result.Root.BoundingVolume.Region[0] != 0.24084696669235753 {
		t.Errorf("Different region min x coordinate")
	}
	if result.Root.BoundingVolume.Region[1] != 0.7388088888874382 {
		t.Errorf("Different region min y coordinate")
	}
	if result.Root.BoundingVolume.Region[2] != 0.24084696669235753 {
		t.Errorf("Different region max x coordinate")
	}
	if result.Root.BoundingVolume.Region[3] != 0.7388088888874382 {
		t.Errorf("Different region max y coordinate")
	}
	if result.Root.BoundingVolume.Region[4] != 0.0 {
		t.Errorf("Different region min z coordinate")
	}
	if result.Root.BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different region max z coordinate")
	}
	if result.Root.GeometricError != 0.0 {
		t.Errorf("Expected Root GeometricError %f, got %f", 0.0, result.Root.GeometricError)
	}
	if result.Root.Refine != "ADD" {
		t.Errorf("Expected Refine type %s, got %s", "ADD", result.Root.Refine)
	}

	pntsFile, err := os.Open(path.Join(tempdir, "content.pnts"))
	defer func() { _ = pntsFile.Close() }()

	if err != nil {
		t.Errorf("Error opening content.pnts: %s", err.Error())
	}

	var buffer = []byte{0, 0, 0, 0}
	_, err = pntsFile.Read(buffer)
	if err != nil {
		t.Errorf("Error reading magic bytes from content.pnts: %s", err.Error())
	}

	var magicString = string(buffer)
	if magicString != "pnts" {
		t.Errorf("Expected magic value: %s, got: %s", "pnts", magicString)
	}

	_, err = pntsFile.Read(buffer)
	var version = binary.LittleEndian.Uint32(buffer)
	if version != 1 {
		t.Errorf("Expected version value: %d, got: %d", 1, version)
	}

	_, err = pntsFile.Read(buffer)
	var length = binary.LittleEndian.Uint32(buffer)
	if length != 175 {
		t.Errorf("Expected len value: %d, got: %d", 175, length)
	}

	_, err = pntsFile.Read(buffer)
	var featureTableLength = binary.LittleEndian.Uint32(buffer)
	if featureTableLength != 132 {
		t.Errorf("Expected featureTableLength value: %d, got: %d", 132, featureTableLength)
	}

	_, err = pntsFile.Read(buffer)
	var positionPlusColors = binary.LittleEndian.Uint32(buffer)
	if positionPlusColors != 15 {
		t.Errorf("Expected position and color section length value: %d, got: %d", 15, positionPlusColors)
	}

	_, err = pntsFile.Read(buffer)
	var batchTableLen = binary.LittleEndian.Uint32(buffer)
	if batchTableLen != 164 {
		t.Errorf("Expected batch table length: %d, got: %d", 164, batchTableLen)
	}

	_, err = pntsFile.Read(buffer)
	var intensityAndClassificationLen = binary.LittleEndian.Uint32(buffer)
	if intensityAndClassificationLen != 2 {
		t.Errorf("Expected intensity and classification sections length: %d, got: %d", 2, intensityAndClassificationLen)
	}

	buffer = make([]byte, 132)
	_, err = pntsFile.Read(buffer)
	var featureTable = string(buffer)
	var expectedFeatureTable = "{\"POINTS_LENGTH\":1,\"RTC_CENTER\":[4586042.6311360,1126398.922751,4272825.711405],\"POSITION\":{\"byteOffset\":0},\"RGB\":{\"byteOffset\":12}}"
	if featureTable != expectedFeatureTable {
		t.Errorf("Expected feature table: \r\n %s \r\n Got: %s", expectedFeatureTable, featureTable)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != 0.0 {
		t.Errorf("Expected position X: %f  got: %f", 0.0, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != 0.0 {
		t.Errorf("Expected position Y: %f  got: %f", 0.0, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != 0.0 {
		t.Errorf("Expected position Z: %f  got: %f", 0.0, positionZ)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var red = buffer[0]
	if red != 1 {
		t.Errorf("Expected red: %d, got: %d", 1, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var green = buffer[0]
	if green != 2 {
		t.Errorf("Expected green: %d, got: %d", 2, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var blue = buffer[0]
	if blue != 3 {
		t.Errorf("Expected blue: %d, got: %d", 3, blue)
	}

	buffer = make([]byte, 164)
	_, err = pntsFile.Read(buffer)
	var batchTable = string(buffer)
	var expectedBatchTable = "{\"INTENSITY\":{\"byteOffset\":0, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},\"CLASSIFICATION\":{\"byteOffset\":1, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	if batchTable != expectedBatchTable {
		t.Errorf("Expected batch table: \r\n %s \r\n Got: %s", expectedBatchTable, batchTable)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var intensity = buffer[0]
	if intensity != 4 {
		t.Errorf("Expected blue: %d, got: %d", 4, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var classification = buffer[0]
	if classification != 5 {
		t.Errorf("Expected blue: %d, got: %d", 5, intensity)
	}
}

func TestConsumerOneChildRefineModeReplace(t *testing.T) {
	// generate mock node with one point and no children
	node := &mockNode{
		boundingBox: geometry.NewBoundingBox(13.6, 13.7995147, 42.3, 42.3306312, 0, 1),
		points: []*data.Point{
			data.NewPoint(13.7995147, 42.3306312, 1, 1, 2, 3, 4, 5),
		},
		depth:               1,
		globalChildrenCount: 2,
		internalSrid:        4326,
		localChildrenCount:  1,
		opts: &tiler.TilerOptions{
			Srid: 4326,
		},
		children: [8]octree.INode{},
	}
	node.children[0] =
		&mockNode{
			parent:      node,
			boundingBox: geometry.NewBoundingBox(13.6, 13.7995147, 42.3, 42.3306312, 0.5, 1),
			points: []*data.Point{
				data.NewPoint(13.6, 42.3, 1, 7, 8, 9, 10, 11),
			},
			depth:               1,
			internalSrid:        4326,
			globalChildrenCount: 1,
			localChildrenCount:  1,
			opts: &tiler.TilerOptions{
				Srid: 4326,
			},
		}

	// generate a temp dir and defer its deletion
	tempdir, _ := ioutil.TempDir(tools.GetRootFolder(), "temp*")
	defer func() { _ = os.RemoveAll(tempdir) }()

	// generate a mock workunit
	workUnit := io.WorkUnit{
		Node:     node,
		Opts:     node.opts,
		BasePath: tempdir,
	}
	workUnitChild := io.WorkUnit{
		Node:     node.children[0],
		Opts:     node.opts,
		BasePath: path.Join(tempdir, "0"),
	}

	// create workChannel and errorChannel
	workChannel := make(chan *io.WorkUnit, 1)
	errorChannel := make(chan error)

	// create waitGroup and add consumer
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	// start consumer
	consumer := io.NewStandardConsumer(proj4_coordinate_converter.NewProj4CoordinateConverter(), tiler.RefineModeReplace)
	go consumer.Consume(workChannel, errorChannel, &waitGroup)

	// inject work unit in channel
	workChannel <- &workUnit
	workChannel <- &workUnitChild

	// close workchannel
	close(workChannel)

	// wait consumer to finish
	waitGroup.Wait()

	// close error channel
	close(errorChannel)

	for err := range errorChannel {
		t.Errorf("Unexpected error found in error channel: %s", err.Error())
	}

	// read tileset.json and validate its content
	jsonFile, err := os.Open(path.Join(tempdir, "tileset.json"))
	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}

	// defer the closing of our jsonFile so that we can parse it later on
	defer func() { _ = jsonFile.Close() }()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var result io.Tileset
	_ = json.Unmarshal([]byte(byteValue), &result)

	if err != nil {
		t.Errorf("Error opening tileset.json: %s", err.Error())
	}
	if result.Asset.Version != "1.0" {
		t.Errorf("Expected asset version %s, got %s", "1.0", result.Asset.Version)
	}
	if result.GeometricError != 0 {
		t.Errorf("Expected geometricError %f, got %f", 0.0, result.GeometricError)
	}
	if len(result.Root.Children) != 1 {
		t.Errorf("Expected root children number %d, got %d", 1, len(result.Root.Children))
	}
	if result.Root.Children[0].Content.Url != "0/tileset.json" {
		t.Errorf("Expected root children content url %s, got %s", "0/tileset.json", result.Root.Children[0].Content.Url)
	}
	if result.Root.Children[0].BoundingVolume.Region[0] != 0.23736477827122882 {
		t.Errorf("Different children region min x coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[1] != 0.7382742735936013 {
		t.Errorf("Different children region min y coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[2] != 0.24084696669235753 {
		t.Errorf("Different children region max x coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[3] != 0.7388088888874382 {
		t.Errorf("Different children region max y coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[4] != 0.5 {
		t.Errorf("Different children region min z coordinate")
	}
	if result.Root.Children[0].BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different children region max z coordinate")
	}
	if result.Root.Children[0].GeometricError != 0.0 {
		t.Errorf("Expected child geometricError %f, got %f", 0.0, result.Root.Children[0].GeometricError)
	}
	if result.Root.Children[0].Refine != "REPLACE" {
		t.Errorf("Expected child geometricError %s, got %s", "REPLACE", result.Root.Children[0].Refine)
	}
	if result.Root.Content.Url != "content.pnts" {
		t.Errorf("Expected root content uri %s, got %s", "content.pnts", result.Root.Content.Url)
	}
	if result.Root.BoundingVolume.Region[0] != 0.23736477827122882 {
		t.Errorf("Different region min x coordinate")
	}
	if result.Root.BoundingVolume.Region[1] != 0.7382742735936013 {
		t.Errorf("Different region min y coordinate")
	}
	if result.Root.BoundingVolume.Region[2] != 0.24084696669235753 {
		t.Errorf("Different region max x coordinate")
	}
	if result.Root.BoundingVolume.Region[3] != 0.7388088888874382 {
		t.Errorf("Different region max y coordinate")
	}
	if result.Root.BoundingVolume.Region[4] != 0.0 {
		t.Errorf("Different region min z coordinate")
	}
	if result.Root.BoundingVolume.Region[5] != 1.0 {
		t.Errorf("Different region max z coordinate")
	}
	if result.Root.GeometricError != 0.0 {
		t.Errorf("Expected Root GeometricError %f, got %f", 0.0, result.Root.GeometricError)
	}
	if result.Root.Refine != "REPLACE" {
		t.Errorf("Expected Refine type %s, got %s", "REPLACE", result.Root.Refine)
	}

	pntsFile, err := os.Open(path.Join(tempdir, "content.pnts"))
	defer func() { _ = pntsFile.Close() }()

	if err != nil {
		t.Errorf("Error opening content.pnts: %s", err.Error())
	}

	var buffer = []byte{0, 0, 0, 0}
	_, err = pntsFile.Read(buffer)
	if err != nil {
		t.Errorf("Error reading magic bytes from content.pnts: %s", err.Error())
	}

	var magicString = string(buffer)
	if magicString != "pnts" {
		t.Errorf("Expected magic value: %s, got: %s", "pnts", magicString)
	}

	_, err = pntsFile.Read(buffer)
	var version = binary.LittleEndian.Uint32(buffer)
	if version != 1 {
		t.Errorf("Expected version value: %d, got: %d", 1, version)
	}

	_, err = pntsFile.Read(buffer)
	var length = binary.LittleEndian.Uint32(buffer)
	if length != 175 {
		t.Errorf("Expected len value: %d, got: %d", 175, length)
	}

	_, err = pntsFile.Read(buffer)
	var featureTableLength = binary.LittleEndian.Uint32(buffer)
	if featureTableLength != 132 {
		t.Errorf("Expected featureTableLength value: %d, got: %d", 132, featureTableLength)
	}

	_, err = pntsFile.Read(buffer)
	var positionPlusColors = binary.LittleEndian.Uint32(buffer)
	if positionPlusColors != 15 {
		t.Errorf("Expected position and color section length value: %d, got: %d", 15, positionPlusColors)
	}

	_, err = pntsFile.Read(buffer)
	var batchTableLen = binary.LittleEndian.Uint32(buffer)
	if batchTableLen != 164 {
		t.Errorf("Expected batch table length: %d, got: %d", 164, batchTableLen)
	}

	_, err = pntsFile.Read(buffer)
	var intensityAndClassificationLen = binary.LittleEndian.Uint32(buffer)
	if intensityAndClassificationLen != 2 {
		t.Errorf("Expected intensity and classification sections length: %d, got: %d", 2, intensityAndClassificationLen)
	}

	buffer = make([]byte, 132)
	_, err = pntsFile.Read(buffer)
	var featureTable = string(buffer)
	var expectedFeatureTable = "{\"POINTS_LENGTH\":1,\"RTC_CENTER\":[4586042.6311360,1126398.922751,4272825.711405],\"POSITION\":{\"byteOffset\":0},\"RGB\":{\"byteOffset\":12}}"
	if featureTable != expectedFeatureTable {
		t.Errorf("Expected feature table: \r\n %s \r\n Got: %s", expectedFeatureTable, featureTable)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != 0.0 {
		t.Errorf("Expected position X: %f  got: %f", 0.0, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != 0.0 {
		t.Errorf("Expected position Y: %f  got: %f", 0.0, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile.Read(buffer)
	var positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != 0.0 {
		t.Errorf("Expected position Z: %f  got: %f", 0.0, positionZ)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var red = buffer[0]
	if red != 1 {
		t.Errorf("Expected red: %d, got: %d", 1, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var green = buffer[0]
	if green != 2 {
		t.Errorf("Expected green: %d, got: %d", 2, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var blue = buffer[0]
	if blue != 3 {
		t.Errorf("Expected blue: %d, got: %d", 3, blue)
	}

	buffer = make([]byte, 164)
	_, err = pntsFile.Read(buffer)
	var batchTable = string(buffer)
	var expectedBatchTable = "{\"INTENSITY\":{\"byteOffset\":0, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},\"CLASSIFICATION\":{\"byteOffset\":1, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	if batchTable != expectedBatchTable {
		t.Errorf("Expected batch table: \r\n %s \r\n Got: %s", expectedBatchTable, batchTable)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var intensity = buffer[0]
	if intensity != 4 {
		t.Errorf("Expected intensity: %d, got: %d", 4, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile.Read(buffer)
	var classification = buffer[0]
	if classification != 5 {
		t.Errorf("Expected classification: %d, got: %d", 5, intensity)
	}

	pntsFile2, err := os.Open(path.Join(tempdir, "0/content.pnts"))
	defer func() { _ = pntsFile2.Close() }()

	if err != nil {
		t.Errorf("Error opening content.pnts: %s", err.Error())
	}

	buffer = []byte{0, 0, 0, 0}
	_, err = pntsFile2.Read(buffer)
	if err != nil {
		t.Errorf("Error reading magic bytes from content.pnts: %s", err.Error())
	}

	magicString = string(buffer)
	if magicString != "pnts" {
		t.Errorf("Expected magic value: %s, got: %s", "pnts", magicString)
	}

	_, err = pntsFile2.Read(buffer)
	version = binary.LittleEndian.Uint32(buffer)
	if version != 1 {
		t.Errorf("Expected version value: %d, got: %d", 1, version)
	}

	_, err = pntsFile2.Read(buffer)
	length = binary.LittleEndian.Uint32(buffer)
	if length != 190 {
		t.Errorf("Expected len value: %d, got: %d", 190, length)
	}

	_, err = pntsFile2.Read(buffer)
	featureTableLength = binary.LittleEndian.Uint32(buffer)
	if featureTableLength != 132 {
		t.Errorf("Expected featureTableLength value: %d, got: %d", 132, featureTableLength)
	}

	_, err = pntsFile2.Read(buffer)
	positionPlusColors = binary.LittleEndian.Uint32(buffer)
	if positionPlusColors != 30 {
		t.Errorf("Expected position and color section length value: %d, got: %d", 30, positionPlusColors)
	}

	_, err = pntsFile2.Read(buffer)
	batchTableLen = binary.LittleEndian.Uint32(buffer)
	if batchTableLen != 164 {
		t.Errorf("Expected batch table length: %d, got: %d", 164, batchTableLen)
	}

	_, err = pntsFile2.Read(buffer)
	intensityAndClassificationLen = binary.LittleEndian.Uint32(buffer)
	if intensityAndClassificationLen != 4 {
		t.Errorf("Expected intensity and classification sections length: %d, got: %d", 4, intensityAndClassificationLen)
	}

	buffer = make([]byte, 132)
	_, err = pntsFile2.Read(buffer)
	featureTable = string(buffer)
	expectedFeatureTable = "{\"POINTS_LENGTH\":2,\"RTC_CENTER\":[4589103.0762060,1118680.099726,4271567.721541],\"POSITION\":{\"byteOffset\":0},\"RGB\":{\"byteOffset\":24}}"
	if featureTable != expectedFeatureTable {
		t.Errorf("Expected feature table: \r\n %s \r\n Got: %s", expectedFeatureTable, featureTable)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != 3060.445068 {
		t.Errorf("Expected position X: %f  got: %f", 3060.445068, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != -7718.823242 {
		t.Errorf("Expected position Y: %f  got: %f", -7718.823242, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != -1257.989868 {
		t.Errorf("Expected position Z: %f  got: %f", -1257.989868, positionZ)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionX = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionX != -3060.445068 {
		t.Errorf("Expected position X: %f  got: %f", -3060.445068, positionX)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionY = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionY != 7718.823242 {
		t.Errorf("Expected position Y: %f  got: %f", 7718.823242, positionY)
	}

	buffer = make([]byte, 4)
	_, err = pntsFile2.Read(buffer)
	positionZ = math.Float32frombits(binary.LittleEndian.Uint32(buffer))
	if positionZ != 1257.989868 {
		t.Errorf("Expected position Z: %f  got: %f", 1257.989868, positionZ)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	red = buffer[0]
	if red != 7 {
		t.Errorf("Expected red: %d, got: %d", 7, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	green = buffer[0]
	if green != 8 {
		t.Errorf("Expected green: %d, got: %d", 8, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	blue = buffer[0]
	if blue != 9 {
		t.Errorf("Expected blue: %d, got: %d", 9, blue)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	red = buffer[0]
	if red != 1 {
		t.Errorf("Expected red: %d, got: %d", 1, red)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	green = buffer[0]
	if green != 2 {
		t.Errorf("Expected green: %d, got: %d", 2, green)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	blue = buffer[0]
	if blue != 3 {
		t.Errorf("Expected blue: %d, got: %d", 3, blue)
	}

	buffer = make([]byte, 164)
	_, err = pntsFile2.Read(buffer)
	batchTable = string(buffer)
	expectedBatchTable = "{\"INTENSITY\":{\"byteOffset\":0, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"},\"CLASSIFICATION\":{\"byteOffset\":2, \"componentType\":\"UNSIGNED_BYTE\", \"type\":\"SCALAR\"}}"
	if batchTable != expectedBatchTable {
		t.Errorf("Expected batch table: \r\n %s \r\n Got: %s", expectedBatchTable, batchTable)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	intensity = buffer[0]
	if intensity != 10 {
		t.Errorf("Expected intensity: %d, got: %d", 10, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	intensity = buffer[0]
	if intensity != 4 {
		t.Errorf("Expected intensity: %d, got: %d", 4, intensity)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	classification = buffer[0]
	if classification != 11 {
		t.Errorf("Expected classification: %d, got: %d", 11, classification)
	}

	buffer = make([]byte, 1)
	_, err = pntsFile2.Read(buffer)
	classification = buffer[0]
	if classification != 5 {
		t.Errorf("Expected classification: %d, got: %d", 5, classification)
	}
}
