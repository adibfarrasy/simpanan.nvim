package adapters

import (
	"encoding/json"
	"fmt"
	"simpanan/internal/common"

	"github.com/itchyny/gojq"
)

func ExecuteJqQuery(q common.QueryMetadata, previousResults []byte) ([]byte, error) {
	if len(previousResults) == 0 {
		return nil, fmt.Errorf("jq stage: no input from previous stage")
	}

	var input any
	if err := json.Unmarshal(previousResults, &input); err != nil {
		return nil, fmt.Errorf("jq stage: previous result is not valid JSON: %w", err)
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
