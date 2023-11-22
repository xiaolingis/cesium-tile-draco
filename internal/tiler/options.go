package tiler

import "strings"

type Algorithm string
type RefineMode string

const (
	// Uniform random pick among all loaded elements. points will tend to be selected in areas with higher density.
	Grid Algorithm = "GRID"

	// Uniform pick in small boxes of points randomly ordered. Point will tend to be more evenly spaced at lower zoom levels.
	// points are grouped in buckets of 1e-6 deg of latitude and longitude. Boxes are randomly sorted and the next data
	// is selected at random from the first box. Next data is taken at random from the following box. When boxes have all been visited
	// the selection will begin again from the first one. If one box becomes empty is removed and replaced with the last one in the set.
	Random    Algorithm = "RANDOM"
	RandomBox Algorithm = "RANDOMBOX"
)

const (
	RefineModeAdd     RefineMode = "ADD"
	RefineModeReplace RefineMode = "REPLACE"
)

func (e RefineMode) String() string {
	if e == RefineModeAdd {
		return "ADD"
	} else if e == RefineModeReplace {
		return "REPLACE"
	}
	return ""
}

func ParseRefineMode(value string) RefineMode {
	normalizedValue := strings.Trim(strings.ToUpper(value), " ")
	if normalizedValue == "ADD" {
		return RefineModeAdd
	} else if normalizedValue == "REPLACE" {
		return RefineModeReplace
	}
	return ""
}

// Contains the options needed for the tiling algorithm
type TilerOptions struct {
	Input                  string     // Input LAS file/folder
	Srid                   int        // EPSG code for SRID of input LAS points
	EightBitColors         bool       // if true assume that LAS uses 8bit color depth
	ZOffset                float64    // Z Offset in meters to apply to points during conversion
	MaxNumPointsPerNode    int32      // Maximum allowed number of points per node for Random and RandomBox Algorithms
	EnableGeoidZCorrection bool       // Enables the conversion from geoid to ellipsoid height
	FolderProcessing       bool       // Enables the processing of all LAS files in folder
	Recursive              bool       // Recursive lookup of LAS files in subfolders
	Algorithm              Algorithm  // Algorithm to use
	CellMaxSize            float64    // Max cell size for grid algorithm
	CellMinSize            float64    // Min cell size for grid algorithm
	RefineMode             RefineMode // Refine mode to use to generate the tileset

	TilerIndexOptions *TilerIndexOptions
	TilerMergeOptions *TilerMergeOptions
}

type TilerIndexOptions struct {
	Output string // Output Cesium Tileset folder
}

type TilerMergeOptions struct {
	Output string // Output Cesium Tileset folder
}
