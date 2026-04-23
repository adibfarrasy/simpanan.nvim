package internal

import (
	"fmt"
	"simpanan/internal/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// swapIntrospect replaces the package-level introspector for the
// duration of a test.
func swapIntrospect(t *testing.T, fn func(label, uri string, ct common.ConnType) (*SchemaCache, error)) {
	t.Helper()
	prev := introspectFn
	introspectFn = fn
	t.Cleanup(func() { introspectFn = prev })
}

func freshCache(label string) *SchemaCache {
	now := time.Now()
	return &SchemaCache{
		ConnectionLabel:    label,
		PopulatedAt:        &now,
		LastRefreshAttempt: &now,
		Databases: []DatabaseSchema{
			{Name: "app", Tables: []TableSchema{{Name: "users", Columns: []string{"id"}}}},
		},
	}
}

func TestEnsureSchemaCache_UnregisteredLabelIsNil(t *testing.T) {
	withTempHome(t)
	seedConnections(t, t.TempDir(), nil) // no connections registered at $HOME
	got, err := EnsureSchemaCache("nope")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestEnsureSchemaCache_RedisIsIneligible(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "rd", URI: "redis://h"},
	})
	called := false
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		called = true
		return nil, nil
	})
	got, err := EnsureSchemaCache("rd")
	assert.NoError(t, err)
	assert.Nil(t, got)
	assert.False(t, called, "introspector must not be called for redis")
}

func TestEnsureSchemaCache_FreshCacheReturnedWithoutIntrospection(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	assert.NoError(t, SaveSchemaCache(freshCache("pg")))

	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		t.Fatalf("introspector must not be called for a fresh cache")
		return nil, nil
	})
	got, err := EnsureSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "pg", got.ConnectionLabel)
		assert.Equal(t, "app", got.Databases[0].Name)
	}
}

func TestEnsureSchemaCache_MissingCachePopulatesAndPersists(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return &SchemaCache{
			ConnectionLabel: label,
			Databases: []DatabaseSchema{
				{Name: "analytics", Tables: []TableSchema{{Name: "events", Columns: []string{"id"}}}},
			},
		}, nil
	})
	got, err := EnsureSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "analytics", got.Databases[0].Name)
		assert.NotNil(t, got.PopulatedAt, "PopulatedAt must be stamped")
	}
	// Persisted.
	onDisk, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, onDisk) {
		assert.Equal(t, "analytics", onDisk.Databases[0].Name)
	}
}

func TestEnsureSchemaCache_StaleCacheRefreshes(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	// Write a stale cache (PopulatedAt well beyond the refresh interval).
	old := time.Now().Add(-2 * time.Hour)
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "pg",
		PopulatedAt:     &old,
		Databases: []DatabaseSchema{
			{Name: "stale", Tables: []TableSchema{{Name: "t", Columns: []string{"c"}}}},
		},
	}))

	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return &SchemaCache{
			ConnectionLabel: label,
			Databases: []DatabaseSchema{
				{Name: "fresh", Tables: []TableSchema{{Name: "t", Columns: []string{"c"}}}},
			},
		}, nil
	})
	got, err := EnsureSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "fresh", got.Databases[0].Name)
	}
}

func TestEnsureSchemaCache_FailedRefreshKeepsStaleDataUpdatesAttempt(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	old := time.Now().Add(-2 * time.Hour)
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "pg",
		PopulatedAt:     &old,
		Databases: []DatabaseSchema{
			{Name: "stale", Tables: []TableSchema{{Name: "t", Columns: []string{"c"}}}},
		},
	}))

	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return nil, fmt.Errorf("db unreachable")
	})
	got, err := EnsureSchemaCache("pg")
	assert.Error(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, "stale", got.Databases[0].Name)
		assert.Equal(t, old.UTC(), got.PopulatedAt.UTC(), "PopulatedAt must not change on failure")
		assert.NotNil(t, got.LastRefreshAttempt)
		assert.True(t, got.LastRefreshAttempt.After(old), "LastRefreshAttempt must be updated")
	}

	// And the updated attempt timestamp is persisted too.
	onDisk, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, onDisk) {
		assert.Equal(t, old.UTC(), onDisk.PopulatedAt.UTC())
		assert.True(t, onDisk.LastRefreshAttempt.After(old))
	}
}

func TestEnsureSchemaCache_FailedRefreshWithNoPriorReturnsNil(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return nil, fmt.Errorf("db unreachable")
	})
	got, err := EnsureSchemaCache("pg")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestHandleAddConnection_WarmsSchemaCache(t *testing.T) {
	withTempHome(t)
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return &SchemaCache{
			ConnectionLabel: label,
			Databases: []DatabaseSchema{
				{Name: "warm", Tables: []TableSchema{{Name: "t", Columns: []string{"c"}}}},
			},
		}, nil
	})

	res, err := HandleAddConnection([]string{"pg>postgres://h/db"})
	assert.NoError(t, err)
	assert.Equal(t, "Success", res)

	onDisk, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, onDisk) {
		assert.Equal(t, "warm", onDisk.Databases[0].Name)
	}
}

func TestHandleAddConnection_IntrospectionFailureDoesNotFailTheCommand(t *testing.T) {
	withTempHome(t)
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return nil, fmt.Errorf("db unreachable")
	})

	res, err := HandleAddConnection([]string{"pg>postgres://h/db"})
	assert.NoError(t, err)
	assert.Equal(t, "Success", res)

	// No cache persisted on failure — EnsureSchemaCache will warm lazily.
	onDisk, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Nil(t, onDisk)
}

func TestHandleAddConnection_RedisDoesNotAttemptIntrospection(t *testing.T) {
	withTempHome(t)
	called := false
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		called = true
		return nil, nil
	})
	res, err := HandleAddConnection([]string{"rd>redis://h"})
	assert.NoError(t, err)
	assert.Equal(t, "Success", res)
	assert.False(t, called, "introspector must not be called for redis")
}

func TestHandleDeleteConnection_DropsSchemaCacheFile(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})
	assert.NoError(t, SaveSchemaCache(freshCache("pg")))

	res, err := HandleDeleteConnection([]string{"pg"})
	assert.NoError(t, err)
	assert.Equal(t, "Success", res)

	onDisk, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Nil(t, onDisk, "cache file must be removed after delete")
}

func TestIsSchemaCacheStale(t *testing.T) {
	now := time.Now()
	tenMinAgo := now.Add(-10 * time.Minute)
	twoHoursAgo := now.Add(-2 * time.Hour)

	assert.True(t, isSchemaCacheStale(nil, now), "nil cache is stale")
	assert.True(t, isSchemaCacheStale(&SchemaCache{}, now), "never-populated cache is stale")
	assert.False(t, isSchemaCacheStale(&SchemaCache{PopulatedAt: &tenMinAgo}, now))
	assert.True(t, isSchemaCacheStale(&SchemaCache{PopulatedAt: &twoHoursAgo}, now))
}
