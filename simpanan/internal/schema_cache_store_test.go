package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func sampleCache(label string) *SchemaCache {
	t0 := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	return &SchemaCache{
		ConnectionLabel:    label,
		PopulatedAt:        &t0,
		LastRefreshAttempt: &t0,
		Databases: []DatabaseSchema{
			{
				Name: "analytics",
				Tables: []TableSchema{
					{Name: "users", Columns: []string{"id", "email"}},
				},
			},
		},
	}
}

func TestSaveLoadSchemaCache_RoundTrip(t *testing.T) {
	withTempHome(t)
	c := sampleCache("pg")
	assert.NoError(t, SaveSchemaCache(c))

	got, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, c.ConnectionLabel, got.ConnectionLabel)
		assert.Equal(t, c.Databases, got.Databases)
		assert.Equal(t, c.PopulatedAt.UTC(), got.PopulatedAt.UTC())
	}
}

func TestLoadSchemaCache_Missing(t *testing.T) {
	withTempHome(t)
	got, err := LoadSchemaCache("nope")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestLoadSchemaCache_Corrupted(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".local/share/nvim", schemaCacheDirName)
	assert.NoError(t, os.MkdirAll(dir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "pg.json"), []byte("{not json"), 0644))

	got, err := LoadSchemaCache("pg")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestSaveSchemaCache_AtomicOverwrite(t *testing.T) {
	withTempHome(t)
	first := sampleCache("pg")
	assert.NoError(t, SaveSchemaCache(first))

	second := sampleCache("pg")
	second.Databases[0].Tables[0].Columns = append(second.Databases[0].Tables[0].Columns, "created_at")
	assert.NoError(t, SaveSchemaCache(second))

	got, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Equal(t, []string{"id", "email", "created_at"}, got.Databases[0].Tables[0].Columns)
}

func TestLoadAllSchemaCaches(t *testing.T) {
	withTempHome(t)
	for _, label := range []string{"pg", "mg", "my"} {
		assert.NoError(t, SaveSchemaCache(sampleCache(label)))
	}

	caches, err := LoadAllSchemaCaches()
	assert.NoError(t, err)
	got := map[string]bool{}
	for _, c := range caches {
		got[c.ConnectionLabel] = true
	}
	assert.Equal(t, map[string]bool{"pg": true, "mg": true, "my": true}, got)
}

func TestLoadAllSchemaCaches_MissingDir(t *testing.T) {
	withTempHome(t)
	caches, err := LoadAllSchemaCaches()
	assert.NoError(t, err)
	assert.Nil(t, caches)
}

func TestLoadAllSchemaCaches_SkipsMalformed(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".local/share/nvim", schemaCacheDirName)
	assert.NoError(t, os.MkdirAll(dir, 0755))
	// One good, one bad.
	good, _ := json.Marshal(sampleCache("pg"))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "pg.json"), good, 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0644))

	caches, err := LoadAllSchemaCaches()
	assert.Error(t, err) // firstErr reported
	assert.Equal(t, 1, len(caches))
	assert.Equal(t, "pg", caches[0].ConnectionLabel)
}

func TestDeleteSchemaCacheFile(t *testing.T) {
	withTempHome(t)
	assert.NoError(t, SaveSchemaCache(sampleCache("pg")))
	assert.NoError(t, DeleteSchemaCacheFile("pg"))

	got, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestDeleteSchemaCacheFile_Missing(t *testing.T) {
	withTempHome(t)
	// Idempotent: deleting something that was never there must not error.
	assert.NoError(t, DeleteSchemaCacheFile("nope"))
}

func TestSchemaCachePath_RejectsUnsafeLabels(t *testing.T) {
	withTempHome(t)
	for _, bad := range []string{"", "..", ".", "foo/bar", "foo\\bar"} {
		_, err := schemaCachePath(bad)
		assert.Error(t, err, "label %q must be rejected", bad)
	}
}
