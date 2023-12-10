package io

import (
	"github.com/ecopia-map/cesium_tiler/internal/octree"
	"sync"
)

type Producer interface {
	Produce(work chan *WorkUnit, wg *sync.WaitGroup, node octree.INode)
}