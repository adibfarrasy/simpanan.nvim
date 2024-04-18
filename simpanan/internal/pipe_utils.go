package internal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"
)

func pipeData(q *QueryMetadata, pipedData []byte) error {
	var input any

	var sliceRes []any
	var mapRes map[string]any
	if err := json.Unmarshal(pipedData, &sliceRes); err != nil {
		if err := json.Unmarshal(pipedData, &mapRes); err != nil {
			return err
		} else {
			input = mapRes
		}
	} else {
		input = sliceRes
	}

	match := regexp.MustCompile(`\{\{([^{}]+)\}\}`).FindAllStringSubmatch(q.ExecLine, -1)
	for _, m := range match {
		captured := m[1]
		query, err := gojq.Parse(captured)
		if err != nil {
			return err
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
				return err
			}
			replacement = v
		}

		q.ExecLine = strings.Replace(q.ExecLine, m[0], fmt.Sprintf("%v", replacement), 1)
	}

	return nil
}
