package adapters

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult []string
		expectedError  error
	}{
		{
			name:           "valid empty input",
			input:          "",
			expectedResult: []string(nil),
			expectedError:  fmt.Errorf("splitMethodParamStr: missing input string"),
		},
		{
			name:           "valid empty object",
			input:          "{}",
			expectedResult: []string{"{}"},
			expectedError:  nil,
		},
		{
			name:           "parse simple object",
			input:          `{"name": {"$regex": "abc"}}`,
			expectedResult: []string{"{\"name\":{\"$regex\":\"abc\"}}"},
			expectedError:  nil,
		},
		{
			name: "parse nested object",
			input: `{
    "$or": [
        { "field_1": {"$regex": "abc"} },
        { "field_2": {"$regex": "abc"} }
        ]
}, {"_id": 1}`,
			expectedResult: []string{"{\n\"$or\":[\n{\"field_1\":{\"$regex\":\"abc\"}},\n{\"field_2\":{\"$regex\":\"abc\"}}\n]\n},", `{"_id":1}`},
			expectedError:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := splitMethodParamStr(test.input)
			assert.Equal(t, test.expectedResult, res)
			assert.Equal(t, test.expectedError, err)
		})
	}
}
