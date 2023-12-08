package unit

import (
	"github.com/ecopia-map/cesium_tiler/internal/data"
	"github.com/ecopia-map/cesium_tiler/internal/geometry"
	"github.com/ecopia-map/cesium_tiler/internal/io"
	"github.com/ecopia-map/cesium_tiler/internal/octree"
	"github.com/ecopia-map/cesium_tiler/internal/tiler"
	"path"
	"sync"
	"testing"
)

func TestProducerInjectsWorkUnits(t *testing.T) {
	var opts = tiler.TilerOptions{
		Srid: 4326,
	}

	rootNode := &mockNode{
		boundingBox: geometry.NewBoundingBox(13.7995147, 13.7995147, 42.3306312, 42.3306312, 0, 1),
		points: []*data.Point{
			data.NewPoint(13.7995147, 42.3306312, 1, 1, 2, 3, 4, 5),
		},
		depth:               1,
		globalChildrenCount: 2,
		localChildrenCount:  1,
		initialized:         true,
		opts:                &opts,
		children: [8]octree.INode{
			&mockNode{
				boundingBox: geometry.NewBoundingBox(13.7995147, 13.7995147, 42.3306312, 42.3306312, 0.5, 1),
				points: []*data.Point{
					data.NewPoint(13.7995147, 42.3306312, 1, 4, 5, 6, 4, 5),
				},
				depth:               1,
				globalChildrenCount: 1,
				localChildrenCount:  1,
				initialized:         true,
				opts:                &opts,
			},
		},
	}

	workChannel := make(chan *io.WorkUnit, 3)
	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	producer := io.NewStandardProducer("basepath", "", &opts)
	producer.Produce(workChannel, &waitGroup, rootNode)
	waitGroup.Wait() // if the test waits here indefinitely then producer is not deregistering itself from the waitgroup with waitGroup.Done()

	if len(workChannel) != 2 {
		t.Errorf("Expected to find %d items in the workchannel but %d were found", 2, len(workChannel))
	}

	rootWorkUnit := <-workChannel
	if rootWorkUnit.Node != rootNode {
		t.Errorf("Missing root node in workchannel")
	}
	if rootWorkUnit.BasePath != "basepath" {
		t.Errorf("Expected basepath: %s got %s", "basepath", rootWorkUnit.BasePath)
	}
	if rootWorkUnit.Opts != &opts {
		t.Errorf("Missing expected tiler options")
	}

	childWorkUnit := <-workChannel
	if childWorkUnit.Node != rootNode.children[0] {
		t.Errorf("Missing child node in workchannel")
	}
	if childWorkUnit.BasePath != path.Join("basepath", "0") {
		t.Errorf("Expected basepath: %s got %s", path.Join("basepath", "0"), childWorkUnit.BasePath)
	}
	if childWorkUnit.Opts != &opts {
		t.Errorf("Missing expected tiler options")
	}

	select {
	case <-workChannel:
	default:
		t.Error("StandardProducer didn't close the WorkChannel was not closed")
	}

}
