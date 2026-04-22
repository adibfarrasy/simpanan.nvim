package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMongoDBName(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{"simple", "mongodb://localhost:27017/mydb", "mydb", false},
		{"with credentials", "mongodb://user:pass@host:27017/mydb", "mydb", false},
		{"srv scheme", "mongodb+srv://host/mydb", "mydb", false},
		{"db with auth source", "mongodb://host/mydb?authSource=admin", "mydb", false},
		{"no database segment", "mongodb://localhost:27017", "", true},
		{"no database segment with trailing slash", "mongodb://localhost:27017/", "", true},
		{"only host", "mongodb://localhost", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mongoDBName(tc.uri)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSplitCommaSeparatedObjStrPreservesSpacesInStrings(t *testing.T) {
	// Regression: spaces inside string literals were previously stripped,
	// mangling values like "John Doe".
	input := `{"name": "John Doe"}`
	got, err := splitCommaSeparatedObjStr(input)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Contains(t, got[0], `"John Doe"`)
}

func TestSplitCommaSeparatedObjStrPreservesSpacesInEscapedString(t *testing.T) {
	input := `{"name": "a \"quoted b\" c"}`
	got, err := splitCommaSeparatedObjStr(input)
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Contains(t, got[0], `a \"quoted b\" c`)
}

func TestSplitCommaSeparatedObjStrSplitsTopLevelCommas(t *testing.T) {
	input := `{"a": 1}, {"b": "x y"}`
	got, err := splitCommaSeparatedObjStr(input)
	assert.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Contains(t, got[1], `"x y"`)
}
