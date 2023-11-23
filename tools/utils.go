package tools

import (
	"encoding/json"
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
