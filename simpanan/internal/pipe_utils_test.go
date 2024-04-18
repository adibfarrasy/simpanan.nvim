package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipeData(t *testing.T) {
	tests := []struct {
		name             string
		qm               QueryMetadata
		payload          []byte
		expectedError    error
		expectedExecLine string
	}{
		{
			name: "JSON array payload",
			qm: QueryMetadata{
				ExecLine: "SELECT * FROM blah WHERE id = {{.[0].foo}};",
			},
			payload:          []byte("[{\"foo\": 1}]"),
			expectedError:    nil,
			expectedExecLine: "SELECT * FROM blah WHERE id = 1;",
		},
		{
			name: "JSON object payload",
			qm: QueryMetadata{
				ExecLine: "SELECT * FROM blah WHERE id = {{.foo}};",
			},
			payload:          []byte("{\"foo\": 1}"),
			expectedError:    nil,
			expectedExecLine: "SELECT * FROM blah WHERE id = 1;",
		},
		{
			name: "multiple swap",
			qm: QueryMetadata{
				ExecLine: "SELECT * FROM blah WHERE id = {{.foo}} AND status = \"{{.bar}}\";",
			},
			payload:          []byte("{\"foo\": 1, \"bar\": \"hello\"}"),
			expectedError:    nil,
			expectedExecLine: "SELECT * FROM blah WHERE id = 1 AND status = \"hello\";",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := pipeData(&test.qm, test.payload)
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, test.expectedExecLine, test.qm.ExecLine)
		})
	}
}
