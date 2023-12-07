package grid_tree

import "C"
import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/mfbonfigli/gocesiumtiler/internal/converters"
	"github.com/mfbonfigli/gocesiumtiler/internal/data"
	"github.com/mfbonfigli/gocesiumtiler/internal/geometry"
)

// Models a node of the octree, which can either be a leaf (a node without children nodes) or not.
// Each Node can contain up to eight children nodes. The node uses a grid algorithm to decide which points to store.
// It divides its bounding box in gridCells and only stores points retained by these cells, propagating the ones rejected
// by the cells to its children which will have smaller cells.
type GridNode struct {
	nodeNID               string
	root                  bool
	parent                *GridNode
	boundingBox           *geometry.BoundingBox
	children              [8]*GridNode
	childrenPath          [8]string
	mergedChildren        []*GridWrapNode
	cells                 map[gridIndex]*gridCell
	points                []*data.Point
	cellSize              float64
	minCellSize           float64
	totalNumberOfPoints   int64
	numberOfPoints        int32
	leaf                  int32
	isChildrenInitialized bool
	extend                *GridNodeExtend

	sync.RWMutex
}

type GridNodeExtend struct {
	tree *GridTree
}

type GridWrapNode struct {
	totalNumberOfPoints int64     // merged points_number
	nodeIndexList       []int     // merged child_node_index
	nodeIndex           int       // current child_node_index
	node                *GridNode // current child_node
}

// Instantiates a new GridNode
func NewGridNode(
	nodeNID string,
	tree *GridTree,
	parent *GridNode,
	boundingBox *geometry.BoundingBox,
	maxCellSize float64,
	minCellSize float64,
	root bool,
) *GridNode {
	// log.Println("new-node nodeNID:", nodeNID)
	node := GridNode{
		nodeNID:               nodeNID,
		parent:                parent,                        // the parent node
		root:                  root,                          // if the node is the tree root
		boundingBox:           boundingBox,                   // bounding box of the node
		mergedChildren:        nil,                           // merge small node
		cellSize:              maxCellSize,                   // max size setting to use for gridCells
		minCellSize:           minCellSize,                   // min size setting to use for gridCells
		points:                make([]*data.Point, 0),        // slice keeping references to points stored in the gridCells
		cells:                 make(map[gridIndex]*gridCell), // gridCells that subdivide this node bounding box
		totalNumberOfPoints:   0,                             // total number of points stored in this node and its children
		numberOfPoints:        0,                             // number of points stored in this node (children excluded)
		leaf:                  1,                             // 1 if is a leaf, 0 otherwise
		isChildrenInitialized: false,                         // flag to see if the node has been initialized
		extend: &GridNodeExtend{
			tree: tree,
		},
	}

	return &node
}

// Adds a Point to the GridNode and propagates the point eventually pushed out to the appropriate children
func (n *GridNode) AddDataPoint(point *data.Point, isFollowSizeThreshold bool) {
	// if !isFollowSizeThreshold || n.cellSize < n.minCellSize/2 {
	// 	log.Println(*point)
	// }
	if point == nil {
		return
	}

	n.Lock()
	if !n.IsChildrenInitialized() {
		n.initializeChildren()
	}
	n.Unlock()

	// isFollowSizeThreshold only valid for current-node
	pushedOutPoint := n.pushPointToCell(point, isFollowSizeThreshold)
	if pushedOutPoint != nil {
		n.addPointToChildren(pushedOutPoint, true)
	} else {
		// if no point was rejected then the number of points stored is increased by 1
		atomic.AddInt32(&n.numberOfPoints, 1)
	}

	// in any case the total number of points stored by the n or its children increases by one
	atomic.AddInt64(&n.totalNumberOfPoints, 1)
}

// Adds a Point to the GridNode
func (n *GridNode) AddDataPointForce(point *data.Point) {
	if point == nil {
		return
	}

	n.points = append(n.points, point)

	// the number of points stored is increased by 1
	atomic.AddInt32(&n.numberOfPoints, 1)

	// the total number of points stored is increased by one
	atomic.AddInt64(&n.totalNumberOfPoints, 1)
}

func (n *GridNode) GetInternalSrid() int {
	return internalCoordinateEpsgCode
}

func (n *GridNode) GetBoundingBoxRegion(converter converters.CoordinateConverter) (*geometry.BoundingBox, error) {
	reg, err := converter.Convert2DBoundingboxToWGS84Region(n.boundingBox, n.GetInternalSrid())

	if err != nil {
		return nil, err
	}

	return reg, nil
}

func (n *GridNode) GetBoundingBox() *geometry.BoundingBox {
	return n.boundingBox
}

func (n *GridNode) GetCellSize() float64 {
	return n.cellSize
}

func (n *GridNode) GetChildren() [8]*GridNode {
	return n.children
}

func (n *GridNode) GetChildrenPath() [8]string {
	return n.childrenPath
}

func (n *GridNode) GetPoints() []*data.Point {
	return n.points
}

func (n *GridNode) TotalNumberOfPoints() int64 {
	return n.totalNumberOfPoints
}

func (n *GridNode) NumberOfPoints() int32 {
	return n.numberOfPoints
}

func (n *GridNode) IsLeaf() bool {
	return atomic.LoadInt32(&n.leaf) == 1
}

func (n *GridNode) IsChildrenInitialized() bool {
	return n.isChildrenInitialized
}

func (n *GridNode) IsRoot() bool {
	return n.root
}

// Computes the geometric error for the given GridNode
func (n *GridNode) ComputeGeometricError() float64 {
	treeExtend := n.extend.tree.extend

	if !treeExtend.useEdgeCalculateGeometricError {

		if n.IsRoot() {
			var w = math.Abs(n.boundingBox.Xmax - n.boundingBox.Xmin)
			var l = math.Abs(n.boundingBox.Ymax - n.boundingBox.Ymin)
			var h = math.Abs(n.boundingBox.Zmax - n.boundingBox.Zmin)
			return math.Sqrt(w*w + l*l + h*h)
		}

		// geometric error is estimated as the maximum possible distance between two points lying in the cell
		return n.cellSize * math.Sqrt(3) * 2

	} else {

		w := treeExtend.chunkEdgeX
		l := treeExtend.chunkEdgeY
		h := treeExtend.chunkEdgeZ
		diagonal := math.Sqrt(w*w + l*l + h*h)

		cellSize := n.cellSize
		rootCellSize := n.extend.tree.rootNode.GetCellSize()
		scale := float64(32) // match js.tilesetMaxScreenSpaceError = 16

		// increase geometricError for small-cell-size in case:split-big-node
		if 2*cellSize < n.minCellSize {
			count := 0
			for {
				if 2*cellSize >= n.minCellSize {
					break
				}
				cellSize *= 2
				count += 1
			}
			cellSize = cellSize * (1.0 - 0.1*float64(count))
		}

		return cellSize / rootCellSize * diagonal / scale
	}

}

// Returns the index of the octant that contains the given Point within this boundingBox
func getOctantFromElement(element *data.Point, bbox *geometry.BoundingBox) uint8 {
	var result uint8 = 0
	if float64(element.X) > bbox.Xmid {
		result += 1
	}
	if float64(element.Y) > bbox.Ymid {
		result += 2
	}
	if float64(element.Z) > bbox.Zmid {
		result += 4
	}
	return result
}

// loads the points stored in the grid cells into the slice data structure
// and recursively builds the points of its children.
// sets the slice reference to nil to allow GC to happen as the cells won't be used anymore
func (n *GridNode) BuildPoints() {
	var points []*data.Point
	for _, cell := range n.cells {
		points = append(points, cell.points...)
	}
	n.points = points
	n.cells = make(map[gridIndex]*gridCell)

	for _, child := range n.children {
		if child != nil {
			child.BuildPoints()
		}
	}
}

func (n *GridNode) GetParent() *GridNode {
	return n.parent
}

// gets the grid cell where the given point falls into, eventually creating it if it does not exist
func (n *GridNode) getPointGridCell(point *data.Point) *gridCell {
	index := n.getPointGridCellIndex(point)

	n.RLock()
	cell := n.cells[*index]
	n.RUnlock()

	// if n.cellSize < n.minCellSize/2 {
	// 	log.Println(*point)
	// }

	if cell == nil {
		// if n.root {
		// 	log.Println("grid-cell-index. cellSize:", n.cellSize, ", x:", index.x, ",y:", index.y, ",z:", index.z)
		// }
		return n.initializeGridCell(index)
	}

	return cell
}

// returns the index of the cell where the input point is falling in
func (n *GridNode) getPointGridCellIndex(point *data.Point) *gridIndex {
	return &gridIndex{
		getDimensionIndex(point.X, n.cellSize),
		getDimensionIndex(point.Y, n.cellSize),
		getDimensionIndex(point.Z, n.cellSize),
	}
}

func (n *GridNode) initializeGridCell(index *gridIndex) *gridCell {
	n.Lock()

	// if n.cellSize < n.minCellSize/2 {
	// 	log.Println(*index, n.cells)
	// }

	if n.cells == nil {
		if n.cellSize < n.minCellSize/2 {
			log.Println("nil cells. nodeNID:", n.nodeNID, *index)
		}
		n.cells = make(map[gridIndex]*gridCell)
	}

	cell := n.cells[*index]
	if cell == nil {
		cell = &gridCell{
			index:         *index,
			size:          n.cellSize,
			sizeThreshold: n.minCellSize,
			points:        nil,
		}
		n.cells[*index] = cell
	}

	n.Unlock()

	return cell
}

// atomically checks if the node is empty
func (n *GridNode) isEmpty() bool {
	return atomic.LoadInt32(&n.numberOfPoints) == 0
}

// pushes a point to its gridcell and returns the point eventually pushed out
func (n *GridNode) pushPointToCell(point *data.Point, isFollowSizeThreshold bool) *data.Point {
	// if !isFollowSizeThreshold || n.cellSize < n.minCellSize/2 {
	// 	log.Println(*point)
	// }
	return n.getPointGridCell(point).pushPoint(point, isFollowSizeThreshold)
}

// add a point to the node children and clears the leaf flag from this node
func (n *GridNode) addPointToChildren(point *data.Point, isFollowSizeThreshold bool) {
	n.children[getOctantFromElement(point, n.boundingBox)].AddDataPoint(point, isFollowSizeThreshold)
	n.clearLeafFlag()
}

// sets the leaf flag to 0 atomically
func (n *GridNode) clearLeafFlag() {
	atomic.StoreInt32(&n.leaf, 0)
}

// initializes the children to new empty nodes
func (n *GridNode) initializeChildren() {
	for i := uint8(0); i < 8; i++ {
		if n.children[i] == nil {
			n.childrenPath[i] = fmt.Sprintf("%d", i)
			n.children[i] = NewGridNode(
				fmt.Sprintf("%s-%d", n.nodeNID, i),
				n.extend.tree,
				n,
				getOctantBoundingBox(&i, n.boundingBox),
				n.cellSize/2.0,
				n.minCellSize,
				false,
			)
		}
	}
	n.isChildrenInitialized = true
}

func (n *GridNode) SetChildren(children []*GridNode) {
	n.TruncateChildren()
	n.totalNumberOfPoints = int64(n.numberOfPoints)

	for i, child := range children {
		n.children[i] = children[i]
		n.totalNumberOfPoints += child.totalNumberOfPoints
	}
}

func (n *GridNode) SetSpartialBoundingBoxByMergeBbox(bboxList []*geometry.BoundingBox) error {

	nBbox := n.GetBoundingBox()
	minX, maxX, minY, maxY, minZ, maxZ := nBbox.Xmin, nBbox.Xmax, nBbox.Ymin, nBbox.Ymax, nBbox.Zmin, nBbox.Zmax
	for _, bbox := range bboxList {
		minX = math.Min(float64(bbox.Xmin), minX)
		minY = math.Min(float64(bbox.Ymin), minY)
		minZ = math.Min(float64(bbox.Zmin), minZ)
		maxX = math.Max(float64(bbox.Xmax), maxX)
		maxY = math.Max(float64(bbox.Ymax), maxY)
		maxZ = math.Max(float64(bbox.Zmax), maxZ)
	}

	newBbox := &geometry.BoundingBox{
		Xmin: minX,
		Xmax: maxX,
		Ymin: minY,
		Ymax: maxY,
		Zmin: minZ,
		Zmax: maxZ,
		Xmid: (minX + maxX) / 2,
		Ymid: (minY + maxY) / 2,
		Zmid: (minZ + maxZ) / 2,
	}

	n.boundingBox = newBbox

	return nil
}

func (n *GridNode) MergeBoundingBox(bbox *geometry.BoundingBox) error {
	return n.SetSpartialBoundingBoxByMergeBbox([]*geometry.BoundingBox{n.boundingBox, bbox})
}

func (n *GridNode) TruncateChildren() {
	log.Println("clear merged-tree.children. children-length:", len(n.children))
	for i := 0; i < 8; i++ {
		n.children[i] = nil
	}

	if n.TotalNumberOfPoints() != int64(n.NumberOfPoints()) {
		log.Printf("clear merged-tree.children. NumberOfPoints[%d] TotalNumberOfPoints:[%d]",
			n.NumberOfPoints(), n.TotalNumberOfPoints())
		n.totalNumberOfPoints = int64(n.numberOfPoints)
	}

	n.leaf = 1

}

func (n *GridNode) MergeSmallChildren(minPointsNum int64) error {
	if n.IsLeaf() {
		return nil
	}

	// merge children.children first
	for i, child := range n.children {
		if child.IsLeaf() {
			continue
		}
		if err := n.children[i].MergeSmallChildren(minPointsNum); err != nil {
			log.Fatal(err)
		}

	}

	// merge children
	wrapChildren := make([]*GridWrapNode, 0)
	branchChildrenCount := 0

	// merge children --- prepare wrap_node
	for i, child := range n.children {
		if child == nil {
			continue
		}
		if !child.IsLeaf() {
			branchChildrenCount += 1
			continue
		}
		wrapChildren = append(wrapChildren, &GridWrapNode{
			totalNumberOfPoints: child.TotalNumberOfPoints(),
			nodeIndexList:       []int{i},
			nodeIndex:           i,
			node:                n.children[i],
		})
	}

	// merge children --- merge-index by sort
	wrapChildrenLen := len(wrapChildren)
	for {
		if wrapChildrenLen < 2 {
			break
		}
		sort.Slice(wrapChildren, func(i, j int) bool {
			return wrapChildren[i].totalNumberOfPoints < wrapChildren[j].totalNumberOfPoints ||
				(wrapChildren[i].totalNumberOfPoints == wrapChildren[j].totalNumberOfPoints &&
					wrapChildren[i].nodeIndex > wrapChildren[j].nodeIndex)
		})

		if wrapChildren[0].totalNumberOfPoints > 4*minPointsNum ||
			(wrapChildren[0].totalNumberOfPoints+wrapChildren[1].totalNumberOfPoints) > 8*minPointsNum {
			// break if no longer suitable for merge
			break
		}

		// merge children[0] to children[1]
		wrapChildren[1].totalNumberOfPoints += wrapChildren[0].totalNumberOfPoints
		wrapChildren[1].nodeIndexList = append(wrapChildren[1].nodeIndexList, wrapChildren[0].nodeIndexList...)

		// // log
		// node0 := wrapChildren[0].node
		// node1 := wrapChildren[1].node
		// log.Printf("merge leaf-node. "+
		// 	"nodeNID:[%s] numPoints:[%d] totalPoints:[%d] to "+
		// 	"nodeNID:[%s] numPoints:[%d] totalPoints:[%d] "+
		// 	"totalMerge:[%d] nodeIndexList:[%s]",
		// 	node0.nodeNID, node0.NumberOfPoints(), node0.TotalNumberOfPoints(),
		// 	node1.nodeNID, node1.NumberOfPoints(), node1.TotalNumberOfPoints(),
		// 	wrapChildren[1].totalNumberOfPoints, tools.FmtJSONString(wrapChildren[1].nodeIndexList),
		// )

		// merge - shrink
		wrapChildren = wrapChildren[1:]
		wrapChildrenLen = wrapChildrenLen - 1

	}

	n.mergedChildren = wrapChildren

	// merge children --- merge-points
	for _, wrapChild := range n.mergedChildren {
		if len(wrapChild.nodeIndexList) < 2 {
			continue
		}
		nodeLen := len(wrapChild.nodeIndexList)
		mainNodeIndex := wrapChild.nodeIndexList[0]
		for j := 1; j < nodeLen; j++ {
			nodeIndex := wrapChild.nodeIndexList[j]

			n.children[mainNodeIndex].numberOfPoints += n.children[nodeIndex].numberOfPoints
			n.children[mainNodeIndex].totalNumberOfPoints += n.children[nodeIndex].totalNumberOfPoints
			n.children[mainNodeIndex].points = append(n.children[mainNodeIndex].points, n.children[nodeIndex].points...)
			n.children[mainNodeIndex].MergeBoundingBox(n.children[nodeIndex].boundingBox)
			n.childrenPath[mainNodeIndex] += n.childrenPath[nodeIndex]

			n.children[nodeIndex].points = nil
			n.children[nodeIndex].mergedChildren = nil
			n.children[nodeIndex].cells = nil

			n.children[nodeIndex].numberOfPoints = 0
			n.children[nodeIndex].totalNumberOfPoints = 0
			n.children[nodeIndex].leaf = 1

			n.children[nodeIndex] = nil
		}
	}

	// merge children to parent
	wrapChildrenLen = len(wrapChildren)
	var node *GridNode = nil
	if branchChildrenCount == 0 && wrapChildrenLen == 1 {
		nodeIndex := wrapChildren[0].nodeIndexList[0]
		node = wrapChildren[0].node

		if n.totalNumberOfPoints <= 4*minPointsNum &&
			(n.totalNumberOfPoints+node.totalNumberOfPoints <= 8*minPointsNum) {
			// merge to parent
			n.numberOfPoints += node.numberOfPoints
			n.points = append(n.points, node.points...)
			n.leaf = 1

			n.children[nodeIndex].points = nil
			n.children[nodeIndex].mergedChildren = nil
			n.children[nodeIndex].cells = nil

			n.children[nodeIndex].numberOfPoints = 0
			n.children[nodeIndex].totalNumberOfPoints = 0
			n.children[nodeIndex].leaf = 1

			n.children[nodeIndex] = nil

		}
	}

	return nil
}

func (n *GridNode) SplitBigNode(maxPointsNum int32) error {
	// traverse children first
	for i, child := range n.children {
		if n.children[i] == nil {
			continue
		}

		if child.IsLeaf() {
			if err := n.children[i].SplitBigLeafNode(maxPointsNum); err != nil {
				log.Fatal(err)
				return err
			}
		} else {
			if err := n.children[i].SplitBigBranchNode(maxPointsNum); err != nil {
				log.Fatal(err)
				return err
			}
		}

	}
	return nil
}

func (n *GridNode) SplitBigBranchNode(maxPointsNum int32) error {
	if n.IsLeaf() {
		return nil
	}

	// ##########################################################################
	// split branch-node
	// ##########################################################################
	// ./content.pnts =>
	// 		- 901/content.pnts
	// 		- 902/content.pnts
	// 		...
	// 		- 907/content.pnts

	// TODO ...

	for i := range n.children {
		if n.children[i] == nil {
			continue
		}
		if err := n.children[i].SplitBigNode(maxPointsNum); err != nil {
			log.Fatal(err)
			return err
		}

	}

	return nil

}

func (n *GridNode) SplitBigLeafNode(maxPointsNum int32) error {
	if !n.IsLeaf() {
		return nil
	}

	if n.NumberOfPoints() <= maxPointsNum {
		// log.Printf("split leaf-node leaf. nodeNID:[%s] numberOfPoints:[%d] totalNumberOfPoints:[%d]  points.len:[%d]",
		// 	n.nodeNID, n.NumberOfPoints(), n.TotalNumberOfPoints(), len(n.points))
		return nil
	}

	// log.Printf("split leaf-node begin. nodeNID:[%s] numberOfPoints:[%d] points.len:[%d]",
	// 	n.nodeNID, n.NumberOfPoints(), len(n.points))

	// ##########################################################################
	// split leaf-node
	// ##########################################################################

	// backup raw points from cur-node
	points := make([]*data.Point, 0)
	points = append(points, n.points...)

	// init cur-node
	n.cells = make(map[gridIndex]*gridCell)
	n.points = make([]*data.Point, 0)
	n.numberOfPoints = 0
	n.totalNumberOfPoints = 0
	n.leaf = 1
	n.mergedChildren = nil

	// push points to cur-node without follow-size-threshold
	isFollowSizeThreshold := false
	for i := range points {
		n.AddDataPoint(points[i], isFollowSizeThreshold)
	}
	n.BuildPoints()

	// log.Printf("split leaf-node end. nodeNID:[%s] numberOfPoints:[%d] totalNumberOfPoints:[%d]  points.len:[%d]",
	// 	n.nodeNID, n.NumberOfPoints(), n.TotalNumberOfPoints(), len(n.points))

	// split children
	for i := range n.children {
		if n.children[i] == nil {
			continue
		}
		if err := n.children[i].SplitBigLeafNode(maxPointsNum); err != nil {
			log.Fatal(err)
			return err
		}
	}

	return nil
}

// Returns a bounding box from the given box and the given octant index
func getOctantBoundingBox(octant *uint8, bbox *geometry.BoundingBox) *geometry.BoundingBox {
	return geometry.NewBoundingBoxFromParent(bbox, octant)
}
