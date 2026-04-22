package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

// withTempHome points HOME at a fresh temp dir so the connection file
// lives somewhere we can reason about and clean up.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func connFilePath(home string) string {
	return filepath.Join(home, ".local/share/nvim/simpanan_connections.json")
}

func seedConnections(t *testing.T, home string, conns []common.KeyURIPair) {
	t.Helper()
	path := connFilePath(home)
	assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	data, err := json.Marshal(conns)
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(path, data, 0644))
}

func readConnections(t *testing.T, home string) []common.KeyURIPair {
	t.Helper()
	data, err := os.ReadFile(connFilePath(home))
	assert.NoError(t, err)
	var res []common.KeyURIPair
	assert.NoError(t, json.Unmarshal(data, &res))
	return res
}

func TestHandleAddConnectionRejectsEmptyInput(t *testing.T) {
	withTempHome(t)
	_, err := HandleAddConnection([]string{""})
	assert.Error(t, err)
	_, err = HandleAddConnection([]string{})
	assert.Error(t, err)
}

func TestHandleAddConnectionRejectsReservedJqLabel(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"jq>jq://"})
	assert.Error(t, err)
}

func TestHandleAddConnectionRejectsEmptyLabel(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{">postgres://h/db"})
	assert.Error(t, err)
}

func TestHandleAddConnectionRejectsUnrecognisedScheme(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"bad>http://example.com"})
	assert.Error(t, err)
}

func TestHandleAddConnectionAcceptsMysqlScheme(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"my1>mysql://root:pw@localhost:3306/app"})
	assert.NoError(t, err)

	conns := readConnections(t, home)
	assert.Equal(t, []common.KeyURIPair{{Key: "my1", URI: "mysql://root:pw@localhost:3306/app"}}, conns)
}

func TestHandleAddConnectionRejectsMissingSeparator(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"no-separator-here"})
	assert.Error(t, err)
}

func TestHandleAddConnectionRejectsDuplicateLabel(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "db1", URI: "postgres://h/db"}})
	_, err := HandleAddConnection([]string{"db1>postgres://h/other"})
	assert.Error(t, err)
}

func TestHandleAddConnectionCreatesParentDirectory(t *testing.T) {
	home := withTempHome(t)
	// no seed: ~/.local/share/nvim does not exist yet
	_, err := HandleAddConnection([]string{"db1>postgres://h/db"})
	assert.NoError(t, err)

	conns := readConnections(t, home)
	assert.Equal(t, []common.KeyURIPair{{Key: "db1", URI: "postgres://h/db"}}, conns)
}

func TestHandleAddConnectionPreservesUriWithGreaterThan(t *testing.T) {
	// Only the first '>' separates label from uri; everything after it
	// belongs to the uri. (URIs with '>' are unusual but must not be
	// silently truncated the way strings.Split would do.)
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"db1>postgres://h/db?tag=a>b"})
	assert.NoError(t, err)

	conns := readConnections(t, home)
	assert.Len(t, conns, 1)
	assert.Equal(t, common.URI("postgres://h/db?tag=a>b"), conns[0].URI)
}

func TestHandleDeleteConnectionReportsMissingLabel(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "db1", URI: "postgres://h/db"}})
	_, err := HandleDeleteConnection([]string{"nope"})
	assert.Error(t, err, "deleting a non-existent label must report an error, not silent success")
}

func TestHandleDeleteConnectionRemovesEntry(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "db1", URI: "postgres://h/db"},
		{Key: "db2", URI: "mongodb://h/db"},
	})
	_, err := HandleDeleteConnection([]string{"db1"})
	assert.NoError(t, err)

	conns := readConnections(t, home)
	assert.Equal(t, []common.KeyURIPair{{Key: "db2", URI: "mongodb://h/db"}}, conns)
}

func TestHandleDeleteConnectionRejectsEmptyArgs(t *testing.T) {
	withTempHome(t)
	_, err := HandleDeleteConnection([]string{""})
	assert.Error(t, err)
	_, err = HandleDeleteConnection([]string{})
	assert.Error(t, err)
}
