package webui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// writeSimp creates a .simp file in t.TempDir with the given body
// and returns its absolute path.
func writeSimp(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	assert.NoError(t, os.WriteFile(path, []byte(body), 0644))
	return path
}

func TestBufferStore_OpenSimpFileLoadsContentsClean(t *testing.T) {
	path := writeSimp(t, "analytics.simp", "pg> SELECT 1\n")
	s := NewBufferStore()

	f, err := s.Open(path)
	assert.NoError(t, err)
	assert.Equal(t, path, f.Path)
	assert.Equal(t, "pg> SELECT 1\n", f.BufferContents)
	assert.Equal(t, f.BufferContents, f.DiskContents)
	assert.Equal(t, StatusClean, f.Status)
	assert.Equal(t, 0, f.CursorByteOffset)
	assert.Equal(t, path, s.Active(), "first opened file becomes active")
}

func TestBufferStore_OpenRejectsNonSimpExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.txt")
	assert.NoError(t, os.WriteFile(path, []byte("x"), 0644))

	s := NewBufferStore()
	_, err := s.Open(path)
	assert.True(t, errors.Is(err, ErrNotSimpFile))
}

func TestBufferStore_OpenRejectsMissingFile(t *testing.T) {
	s := NewBufferStore()
	_, err := s.Open("/does/not/exist.simp")
	assert.True(t, errors.Is(err, ErrPathNotFound))
}

func TestBufferStore_OpenRejectsEmptyPath(t *testing.T) {
	s := NewBufferStore()
	_, err := s.Open("")
	assert.True(t, errors.Is(err, ErrEmptyPath))
}

func TestBufferStore_OpenRejectsAlreadyOpen(t *testing.T) {
	path := writeSimp(t, "a.simp", "")
	s := NewBufferStore()
	_, err := s.Open(path)
	assert.NoError(t, err)
	_, err = s.Open(path)
	assert.True(t, errors.Is(err, ErrAlreadyOpen))
}

func TestBufferStore_EditTransitionsToModified(t *testing.T) {
	path := writeSimp(t, "a.simp", "pg> SELECT 1")
	s := NewBufferStore()
	_, _ = s.Open(path)

	f, err := s.Edit(path, "pg> SELECT 2", 12)
	assert.NoError(t, err)
	assert.Equal(t, StatusModified, f.Status)
	assert.Equal(t, 12, f.CursorByteOffset)
	assert.Equal(t, "pg> SELECT 2", f.BufferContents)
	assert.Equal(t, "pg> SELECT 1", f.DiskContents, "disk contents unchanged until save")
}

func TestBufferStore_EditBackToDiskReturnsToClean(t *testing.T) {
	path := writeSimp(t, "a.simp", "pg> SELECT 1")
	s := NewBufferStore()
	_, _ = s.Open(path)
	_, _ = s.Edit(path, "pg> SELECT 2", 0)
	f, err := s.Edit(path, "pg> SELECT 1", 0)
	assert.NoError(t, err)
	assert.Equal(t, StatusClean, f.Status, "edit back to disk contents → clean")
}

func TestBufferStore_EditUnknownPath(t *testing.T) {
	s := NewBufferStore()
	_, err := s.Edit("/nope.simp", "", 0)
	assert.True(t, errors.Is(err, ErrFileNotOpen))
}

func TestBufferStore_SaveWritesBufferToDiskAndClears(t *testing.T) {
	path := writeSimp(t, "a.simp", "pg> SELECT 1")
	s := NewBufferStore()
	_, _ = s.Open(path)
	_, _ = s.Edit(path, "pg> SELECT 42", 0)

	assert.NoError(t, s.Save(path))
	onDisk, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "pg> SELECT 42", string(onDisk))

	f, _ := s.Get(path)
	assert.Equal(t, StatusClean, f.Status)
	assert.Equal(t, "pg> SELECT 42", f.DiskContents)
}

func TestBufferStore_SaveUnknownPath(t *testing.T) {
	s := NewBufferStore()
	assert.True(t, errors.Is(s.Save("/nope.simp"), ErrFileNotOpen))
}

func TestBufferStore_CloseRemovesAndPromotesActive(t *testing.T) {
	a := writeSimp(t, "a.simp", "")
	b := writeSimp(t, "b.simp", "")
	s := NewBufferStore()
	_, _ = s.Open(a)
	_, _ = s.Open(b)
	assert.Equal(t, a, s.Active())

	assert.NoError(t, s.Close(a))
	// After closing the active file, another open file gets promoted
	// so ActiveFileIsOpen keeps holding.
	assert.Equal(t, b, s.Active())

	assert.NoError(t, s.Close(b))
	assert.Equal(t, "", s.Active(), "no files open → active is empty")
}

func TestBufferStore_CloseUnknown(t *testing.T) {
	s := NewBufferStore()
	assert.True(t, errors.Is(s.Close("/nope.simp"), ErrFileNotOpen))
}

func TestBufferStore_SwitchActiveRequiresOpenFile(t *testing.T) {
	a := writeSimp(t, "a.simp", "")
	b := writeSimp(t, "b.simp", "")
	s := NewBufferStore()
	_, _ = s.Open(a)
	_, _ = s.Open(b)
	assert.NoError(t, s.SwitchActive(b))
	assert.Equal(t, b, s.Active())

	err := s.SwitchActive("/nope.simp")
	assert.True(t, errors.Is(err, ErrFileNotOpen))
}

func TestBufferStore_ListReturnsSnapshot(t *testing.T) {
	a := writeSimp(t, "a.simp", "hello")
	s := NewBufferStore()
	_, _ = s.Open(a)
	list := s.List()
	assert.Equal(t, 1, len(list))

	// Mutating the returned slice's entries must not affect the store.
	list[0].BufferContents = "tampered"
	f, _ := s.Get(a)
	assert.Equal(t, "hello", f.BufferContents)
}

func TestOpenFile_RecomputeHonoursInvariant(t *testing.T) {
	f := &OpenFile{DiskContents: "abc", BufferContents: "abc"}
	f.Recompute()
	assert.Equal(t, StatusClean, f.Status)
	f.BufferContents = "abd"
	f.Recompute()
	assert.Equal(t, StatusModified, f.Status)
}
