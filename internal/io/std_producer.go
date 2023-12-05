package io

import (
	"path"
	"sort"
	"sync"

	"github.com/mfbonfigli/gocesiumtiler/internal/octree/grid_tree"
	"github.com/mfbonfigli/gocesiumtiler/internal/tiler"
)

type StandardProducer struct {
	basePath string
	options  *tiler.TilerOptions
}

func NewStandardProducer(basepath string, subfolder string, options *tiler.TilerOptions) *StandardProducer {
	return &StandardProducer{
		basePath: path.Join(basepath, subfolder),
		options:  options,
	}
}

// Parses a tree node and submits WorkUnits the the provided workchannel. Should be called only on the tree root node.
// Closes the channel when all work is submitted.
func (p *StandardProducer) Produce(work chan *WorkUnit, wg *sync.WaitGroup, node *grid_tree.GridNode) {
	p.produce(p.basePath, node, work, wg)
	close(work)
	wg.Done()
}

// Parses a tree node and submits WorkUnits the the provided workchannel.
func (p *StandardProducer) produce(basePath string, node *grid_tree.GridNode, work chan *WorkUnit, wg *sync.WaitGroup) {
	// if node contains points (it should always be the case), then submit work
	if node.NumberOfPoints() > 0 {
		work <- &WorkUnit{
			Node:     node,
			BasePath: basePath,
			Opts:     p.options,
		}
	}

	// iterate all non nil children and recursively submit all work units
	for i, child := range node.GetChildren() {
		if child != nil && child.IsInitialized() {
			childrenPath := node.GetChildrenPath()
			childPath := childrenPath[i]
			// sort "74520" to "02457" for merge_children case
			if len(childPath) > 1 {
				childList := []byte(childPath)
				sort.Slice(childList, func(i, j int) bool { return childList[i] < childList[j] })
				childPath = string(childList)
			}
			p.produce(path.Join(basePath, childPath), child, work, wg)
		}
	}
}

type StandardMergeProducer struct {
	basePath string
	options  *tiler.TilerOptions
}

func NewStandardMergeProducer(basepath string, subfolder string, options *tiler.TilerOptions) *StandardMergeProducer {
	return &StandardMergeProducer{
		basePath: path.Join(basepath, subfolder),
		options:  options,
	}
}

// Parses a tree node and submits WorkUnits the the provided workchannel. Should be called only on the tree root node.
// Closes the channel when all work is submitted.
func (p *StandardMergeProducer) Produce(work chan *WorkUnit, wg *sync.WaitGroup, node *grid_tree.GridNode) {
	p.produce(p.basePath, node, work, wg)
	close(work)
	wg.Done()
}

// Parses a tree node and submits WorkUnits the the provided workchannel.
func (p *StandardMergeProducer) produce(basePath string, node *grid_tree.GridNode, work chan *WorkUnit, wg *sync.WaitGroup) {
	// if node contains points (it should always be the case), then submit work
	if node.NumberOfPoints() > 0 {
		work <- &WorkUnit{
			Node:     node,
			BasePath: basePath,
			Opts:     p.options,
		}
	}

}
