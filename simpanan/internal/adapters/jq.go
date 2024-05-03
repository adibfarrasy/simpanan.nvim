package adapters

import (
	"encoding/json"
	"simpanan/internal/common"

	"github.com/itchyny/gojq"
)

func ExecuteJqQuery(q common.QueryMetadata, previousResults []byte) ([]byte, error) {
	var input any

	var sliceRes []any
	var mapRes map[string]any
	if err := json.Unmarshal(previousResults, &sliceRes); err != nil {
		if err := json.Unmarshal(previousResults, &mapRes); err != nil {
			return nil, err
		} else {
			input = mapRes
		}
	} else {
		input = sliceRes
	}

	query, err := gojq.Parse(q.QueryLine)
	if err != nil {
		return nil, err
	}
	iter := query.Run(input)

	var replacement any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if err, ok := err.(*gojq.HaltError); ok && err.Value() == nil {
				break
			}
			return nil, err
		}
		replacement = v
	}

	return json.Marshal(replacement)
}
