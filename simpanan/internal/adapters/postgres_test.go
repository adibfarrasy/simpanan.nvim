package adapters

import (
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryTypePostgres(t *testing.T) {
	tests := []struct {
		query string
		want  common.QueryType
	}{
		// DML writes
		{"INSERT INTO t VALUES (1)", common.Write},
		{"update t set x=1", common.Write},
		{"DELETE FROM t", common.Write},
		// DDL — previously misclassified as read
		{"CREATE TABLE t (id int)", common.Write},
		{"drop table t", common.Write},
		{"ALTER TABLE t ADD c int", common.Write},
		{"truncate t", common.Write},
		{"grant select on t to u", common.Write},
		{"REVOKE all on t from u", common.Write},
		// Reads
		{"SELECT * FROM t", common.Read},
		{"with cte as (select 1) select * from cte", common.Read},
		{"   select 1  ", common.Read},
		// Edge
		{"", common.Read},
		{"   ", common.Read},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			assert.Equal(t, tc.want, QueryTypePostgres(tc.query))
		})
	}
}

func TestConvertToTypePreservesIntegerPrecision(t *testing.T) {
	// bigint values beyond 2^53 must not be coerced to float64.
	b := []byte("9007199254740993") // 2^53 + 1
	got := convertToType(b)
	asInt, ok := got.(int64)
	assert.True(t, ok, "expected int64, got %T", got)
	assert.Equal(t, int64(9007199254740993), asInt)
}

func TestConvertToTypeIntegerStaysInteger(t *testing.T) {
	got := convertToType([]byte("42"))
	asInt, ok := got.(int64)
	assert.True(t, ok, "expected int64, got %T", got)
	assert.Equal(t, int64(42), asInt)
}

func TestConvertToTypeFloatStaysFloat(t *testing.T) {
	got := convertToType([]byte("1.5"))
	asFloat, ok := got.(float64)
	assert.True(t, ok, "expected float64, got %T", got)
	assert.Equal(t, 1.5, asFloat)
}

func TestConvertToTypeBooleanLiteralsOnly(t *testing.T) {
	// "true"/"false" map to boolean.
	assert.Equal(t, true, convertToType([]byte("true")))
	assert.Equal(t, false, convertToType([]byte("false")))

	// "1" and "0" are integers, not booleans — matches the pg driver's
	// raw bytes for integer columns.
	assert.Equal(t, int64(1), convertToType([]byte("1")))
	assert.Equal(t, int64(0), convertToType([]byte("0")))
}

func TestConvertToTypeFallsBackToString(t *testing.T) {
	assert.Equal(t, "hello", convertToType([]byte("hello")))
}
