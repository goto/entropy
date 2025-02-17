package json

import (
	"encoding/json"

	"github.com/tidwall/sjson"
)

func SetJSONField(json json.RawMessage, key string, value interface{}) (json.RawMessage, error) {
	sjson, err := sjson.SetBytes(json, key, value)

	if err != nil {
		return nil, err
	}

	return sjson, nil
}
