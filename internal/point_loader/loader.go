package point_loader

import (
	"github.com/ecopia-map/cesium_tiler/internal/data"
)

// A Tree contains methods to store and properly shuffle points for subsequent retrieval in the generation of the
// tree structure
type Loader interface {
	// Adds a Point to the Tree
	AddPoint(e *data.Point)

	// Returns the next random Point from the Tree
	GetNext() (*data.Point, bool)

	// Initializes the structure to allow proper retrieval of points. Must be called after last element has been added but
	// before first call to GetNext
	InitializeLoader()
	ClearLoader()

	// Returns the bounding box extremes of the stored cloud minX, maxX, minY, maxY, minZ, maxZ
	GetBounds() []float64
}
