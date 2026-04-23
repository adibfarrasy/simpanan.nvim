package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleRefreshSchemas_SingleLabel(t *testing.T) {
	withTempHome(t)
	assert.NoError(t, SaveSchemaCache(sampleCache("pg")))
	assert.NoError(t, SaveSchemaCache(sampleCache("mg")))

	res, err := HandleRefreshSchemas([]string{"pg"})
	assert.NoError(t, err)
	assert.Contains(t, res, "pg")

	pg, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Nil(t, pg, "pg cache must be dropped")

	mg, err := LoadSchemaCache("mg")
	assert.NoError(t, err)
	assert.NotNil(t, mg, "mg cache must be untouched")
}

func TestHandleRefreshSchemas_EmptyLabelRefreshesAll(t *testing.T) {
	withTempHome(t)
	for _, label := range []string{"pg", "mg", "my"} {
		assert.NoError(t, SaveSchemaCache(sampleCache(label)))
	}
	res, err := HandleRefreshSchemas([]string{""})
	assert.NoError(t, err)
	assert.Contains(t, res, "all")

	caches, err := LoadAllSchemaCaches()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(caches))
}

func TestHandleRefreshSchemas_NoArgsRefreshesAll(t *testing.T) {
	withTempHome(t)
	assert.NoError(t, SaveSchemaCache(sampleCache("pg")))
	res, err := HandleRefreshSchemas(nil)
	assert.NoError(t, err)
	assert.Contains(t, res, "all")

	pg, err := LoadSchemaCache("pg")
	assert.NoError(t, err)
	assert.Nil(t, pg)
}

func TestHandleRefreshSchemas_MissingCacheIsNoOp(t *testing.T) {
	withTempHome(t)
	// No caches on disk; should succeed anyway (idempotent).
	res, err := HandleRefreshSchemas([]string{"nope"})
	assert.NoError(t, err)
	assert.Contains(t, res, "nope")
}

func TestHandleRefreshSchemas_MissingCacheDirIsNoOp(t *testing.T) {
	withTempHome(t)
	// Not even the cache dir exists.
	_, err := HandleRefreshSchemas([]string{""})
	assert.NoError(t, err)
}

func TestHandleRefreshSchemas_SkipsNonJsonFiles(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".local/share/nvim", schemaCacheDirName)
	assert.NoError(t, os.MkdirAll(dir, 0755))
	assert.NoError(t, SaveSchemaCache(sampleCache("pg")))
	// An unrelated file in the dir must survive a refresh-all.
	stray := filepath.Join(dir, "readme.txt")
	assert.NoError(t, os.WriteFile(stray, []byte("hello"), 0644))

	_, err := HandleRefreshSchemas(nil)
	assert.NoError(t, err)

	_, err = os.Stat(stray)
	assert.NoError(t, err, "non-json file must be left alone")
}
