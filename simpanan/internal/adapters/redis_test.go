package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenizeRedisCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"simple", "GET foo", []string{"GET", "foo"}, false},
		{"collapses multiple spaces", "GET  foo", []string{"GET", "foo"}, false},
		{"leading and trailing spaces", "  GET foo  ", []string{"GET", "foo"}, false},
		{"double-quoted value with space", `SET k "a b"`, []string{"SET", "k", "a b"}, false},
		{"single-quoted value", `SET k 'a b'`, []string{"SET", "k", "a b"}, false},
		{"escaped quote inside string", `SET k "a\"b"`, []string{"SET", "k", `a"b`}, false},
		{"quoted empty string", `SET k ""`, []string{"SET", "k", ""}, false},
		{"tab separator", "GET\tfoo", []string{"GET", "foo"}, false},
		{"unterminated quote", `SET k "foo`, nil, true},
		{"empty input", "", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tokenizeRedisCommand(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
