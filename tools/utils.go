package tools

import (
	"encoding/json"
	"math"
)

const (
	ChunkTilesetFilePrefix = "chunk-tileset-"
)

func FmtJSONString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "marshal data fail"
	}
	return string(data)
}

const (
	FloatMin  = 0.000001
	RadiusMin = float64(0.0000000001)
)

func IsFloatEqual(f1, f2 float64) bool {
	return math.Dim(f1, f2) < FloatMin
}

func IsRadiusEqual(r1, r2 float64) bool {
	return math.Dim(r1, r2) < RadiusMin
}
