package offset_elevation_corrector

import "github.com/ecopia-map/cesium_tiler/internal/converters"

type OffsetElevationCorrector struct {
	Offset float64
}

func NewOffsetElevationCorrector(offset float64) converters.ElevationCorrector {
	return &OffsetElevationCorrector{
		Offset: offset,
	}
}

func (c *OffsetElevationCorrector) CorrectElevation(lon, lat, z float64) float64 {
	return z + c.Offset
}
