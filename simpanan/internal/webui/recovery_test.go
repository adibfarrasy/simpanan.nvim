package webui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// withTempHome points HOME at a fresh temp dir so the recovery file
// lives somewhere isolated. Mirrors the helper used by the
// connection-registry tests.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestFlushAndLoadRecovery_RoundTrip(t *testing.T) {
	withTempHome(t)
	a := writeSimp(t, "a.simp", "pg> SELECT 1")
	b := writeSimp(t, "b.simp", "mg> db.users.find()")

	src := NewBufferStore()
	_, _ = src.Open(a)
	_, _ = src.Open(b)
	_, _ = src.Edit(b, "mg> db.users.findOne()", 22)
	_ = src.SwitchActive(b)
	assert.NoError(t, src.FlushRecovery())

	dst := NewBufferStore()
	assert.NoError(t, dst.LoadRecovery())

	got := dst.List()
	assert.Equal(t, 2, len(got))
	assert.Equal(t, b, dst.Active())

	// Lookup by path, since map iteration order is not stable.
	gotMap := map[string]OpenFile{}
	for _, f := range got {
		gotMap[f.Path] = f
	}
	assert.Equal(t, "pg> SELECT 1", gotMap[a].BufferContents)
	assert.Equal(t, StatusClean, gotMap[a].Status)
	assert.Equal(t, "mg> db.users.findOne()", gotMap[b].BufferContents)
	assert.Equal(t, StatusModified, gotMap[b].Status,
		"buffer differs from disk after restore → modified")
	assert.Equal(t, 22, gotMap[b].CursorByteOffset)
}

func TestLoadRecovery_MissingFileIsCleanStart(t *testing.T) {
	withTempHome(t)
	s := NewBufferStore()
	assert.NoError(t, s.LoadRecovery())
	assert.Equal(t, 0, len(s.List()))
	assert.Equal(t, "", s.Active())
}

func TestLoadRecovery_CorruptFileSurfacesErrorAndLeavesEmpty(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".local/share/nvim")
	assert.NoError(t, os.MkdirAll(dir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, recoveryFileName),
		[]byte("{not json"), 0644))

	s := NewBufferStore()
	err := s.LoadRecovery()
	assert.Error(t, err)
	assert.Equal(t, 0, len(s.List()))
}

func TestLoadRecovery_UnknownVersionSkippedSilently(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".local/share/nvim")
	assert.NoError(t, os.MkdirAll(dir, 0755))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, recoveryFileName),
		[]byte(`{"version":99,"files":[],"active":""}`), 0644))

	s := NewBufferStore()
	err := s.LoadRecovery()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(s.List()))
}

func TestLoadRecovery_ExternallyEditedDiskReflectedInStatus(t *testing.T) {
	withTempHome(t)
	a := writeSimp(t, "a.simp", "pg> SELECT 1")

	src := NewBufferStore()
	_, _ = src.Open(a)
	assert.NoError(t, src.FlushRecovery())

	// External edit between sessions: someone else writes new content.
	assert.NoError(t, os.WriteFile(a, []byte("pg> SELECT 999"), 0644))

	dst := NewBufferStore()
	assert.NoError(t, dst.LoadRecovery())

	f, _ := dst.Get(a)
	assert.Equal(t, "pg> SELECT 1", f.BufferContents,
		"in-memory buffer survives the restart")
	assert.Equal(t, "pg> SELECT 999", f.DiskContents,
		"disk_contents reflects what is actually on disk now")
	assert.Equal(t, StatusModified, f.Status,
		"divergence between buffer and disk → modified")
}

func TestLoadRecovery_VanishedDiskFileRestoredAsModified(t *testing.T) {
	withTempHome(t)
	a := writeSimp(t, "a.simp", "pg> SELECT 1")

	src := NewBufferStore()
	_, _ = src.Open(a)
	assert.NoError(t, src.FlushRecovery())

	// Someone deleted the file between sessions.
	assert.NoError(t, os.Remove(a))

	dst := NewBufferStore()
	assert.NoError(t, dst.LoadRecovery())
	f, ok := dst.Get(a)
	assert.True(t, ok, "vanished file is still restored so user does not lose buffer")
	assert.Equal(t, "pg> SELECT 1", f.BufferContents)
	assert.Equal(t, "", f.DiskContents)
	assert.Equal(t, StatusModified, f.Status)
}

func TestLoadRecovery_ActiveSkippedIfNotInOpenFiles(t *testing.T) {
	home := withTempHome(t)
	a := writeSimp(t, "a.simp", "x")
	dir := filepath.Join(home, ".local/share/nvim")
	assert.NoError(t, os.MkdirAll(dir, 0755))
	body := `{"version":1,"active":"/no/such.simp","files":[{"path":"` + a + `","disk_contents":"x","buffer_contents":"x","cursor_byte_offset":0,"status":"clean"}]}`
	assert.NoError(t, os.WriteFile(filepath.Join(dir, recoveryFileName), []byte(body), 0644))

	s := NewBufferStore()
	assert.NoError(t, s.LoadRecovery())
	assert.Equal(t, "", s.Active(),
		"active gets blanked when the recorded path is not among restored files (ActiveFileIsOpen)")
}

func TestServerStartFlushOnShutdown(t *testing.T) {
	withTempHome(t)
	a := writeSimp(t, "a.simp", "pg> SELECT 1")
	_, srv, stop := startTestServer(t)
	_, err := srv.buffers.Open(a)
	assert.NoError(t, err)
	stop()

	// After shutdown the recovery file exists and contains a.
	dst := NewBufferStore()
	assert.NoError(t, dst.LoadRecovery())
	_, ok := dst.Get(a)
	assert.True(t, ok, "shutdown must flush the recovery file")
}
