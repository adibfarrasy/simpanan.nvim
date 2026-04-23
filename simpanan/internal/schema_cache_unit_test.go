package internal

import (
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSqlDatabases(t *testing.T) {
	rows := []sqlColumnRow{
		{"analytics", "users", "id"},
		{"analytics", "users", "email"},
		{"analytics", "users", "created_at"},
		{"analytics", "events", "id"},
		{"analytics", "events", "user_id"},
		{"reporting", "daily_rollup", "day"},
		{"reporting", "daily_rollup", "count"},
	}

	got := buildSqlDatabases(rows)

	assert.Equal(t, 2, len(got))
	assert.Equal(t, "analytics", got[0].Name)
	assert.Equal(t, "reporting", got[1].Name)

	assert.Equal(t, 2, len(got[0].Tables))
	assert.Equal(t, "users", got[0].Tables[0].Name)
	assert.Equal(t, []string{"id", "email", "created_at"}, got[0].Tables[0].Columns)
	assert.Equal(t, "events", got[0].Tables[1].Name)
	assert.Equal(t, []string{"id", "user_id"}, got[0].Tables[1].Columns)

	assert.Equal(t, 1, len(got[1].Tables))
	assert.Equal(t, "daily_rollup", got[1].Tables[0].Name)
	assert.Equal(t, []string{"day", "count"}, got[1].Tables[0].Columns)
}

func TestBuildSqlDatabases_PreservesOrderAcrossInterleaving(t *testing.T) {
	// Database and table order follows first-seen, even when the input
	// stream interleaves them. (The introspection query orders by
	// schema/table in practice, but the builder must not rely on that.)
	rows := []sqlColumnRow{
		{"a", "t1", "c1"},
		{"b", "t1", "c1"},
		{"a", "t2", "c1"},
		{"b", "t2", "c1"},
		{"a", "t1", "c2"},
	}

	got := buildSqlDatabases(rows)

	assert.Equal(t, []string{"a", "b"}, []string{got[0].Name, got[1].Name})
	assert.Equal(t, []string{"t1", "t2"}, []string{got[0].Tables[0].Name, got[0].Tables[1].Name})
	assert.Equal(t, []string{"c1", "c2"}, got[0].Tables[0].Columns)
}

func TestBuildSqlDatabases_Empty(t *testing.T) {
	got := buildSqlDatabases(nil)
	assert.Equal(t, 0, len(got))
}

func TestIntrospectSchema_RedisAndJqAreIneligible(t *testing.T) {
	for _, ct := range []common.ConnType{common.Redis, common.Jq} {
		got, err := IntrospectSchema("x", "ignored://", ct)
		assert.NoError(t, err)
		assert.Nil(t, got, "ct=%s must return nil cache", ct)
	}
}
