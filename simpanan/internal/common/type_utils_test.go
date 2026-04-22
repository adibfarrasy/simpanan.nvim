package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURIConnType(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    *ConnType
		wantErr bool
	}{
		{"postgres scheme", "postgres://u:p@h/db", &Postgres, false},
		{"postgresql scheme", "postgresql://u:p@h/db", &Postgres, false},
		{"mysql scheme", "mysql://u:p@h:3306/db", &Mysql, false},
		{"mongodb scheme", "mongodb://h/db", &Mongo, false},
		{"mongodb+srv scheme", "mongodb+srv://h/db", &Mongo, false},
		{"redis scheme", "redis://h:6379", &Redis, false},
		{"rediss scheme", "rediss://h:6379", &Redis, false},
		{"jq scheme", "jq://", &Jq, false},
		{"unknown scheme", "http://h", nil, true},
		{"missing scheme separator", "postgres", nil, true},
		{"empty string", "", nil, true},
		// Loose substring match used to accept any scheme containing
		// the substring "postgres". It must now reject.
		{"spurious substring is rejected", "my-postgres-proxy://h", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := URI(tc.uri).ConnType()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetConfigIgnoresMalformedOptions(t *testing.T) {
	// No panic on options missing the '=' separator.
	err := SetConfig([]string{"debug_mode", "=value", "max_row_limit=50"})
	assert.NoError(t, err)
	assert.Equal(t, 50, GetConfig().MaxRowLimit)
}
