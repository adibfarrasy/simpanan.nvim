package webui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func postJSON(t *testing.T, base, path string, body interface{}) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	assert.NoError(t, err)
	resp, err := http.Post(base+path, "application/json", bytes.NewReader(buf))
	assert.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

func getBody(t *testing.T, url string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(url)
	assert.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func TestAPI_OpenFileHappyPath(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte("|pg> SELECT 1"), 0644))

	code, body := postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})
	assert.Equal(t, http.StatusOK, code)
	var f OpenFile
	assert.NoError(t, json.Unmarshal(body, &f))
	assert.Equal(t, "|pg> SELECT 1", f.BufferContents)
	assert.Equal(t, StatusClean, f.Status)
}

func TestAPI_OpenRejectsNonSimp(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	nonSimp := filepath.Join(t.TempDir(), "plain.txt")
	assert.NoError(t, os.WriteFile(nonSimp, []byte("x"), 0644))

	code, body := postJSON(t, base, "/api/files/open", openRequest{Path: nonSimp})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "not a .simp")
}

func TestAPI_OpenRejectsMissingFile(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	code, body := postJSON(t, base, "/api/files/open", openRequest{Path: "/does/not/exist.simp"})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, string(body), "does not exist")
}

func TestAPI_OpenRejectsDuplicateOpen(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte(""), 0644))

	code, _ := postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})
	assert.Equal(t, http.StatusOK, code)
	code, body := postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})
	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, string(body), "already open")
}

func TestAPI_EditUpdatesBuffer(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte("|pg> SELECT 1"), 0644))
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})

	code, body := postJSON(t, base, "/api/files/edit", editRequest{
		Path: simpPath, BufferContents: "|pg> SELECT 2", CursorByteOffset: 12,
	})
	assert.Equal(t, http.StatusOK, code)
	var f OpenFile
	assert.NoError(t, json.Unmarshal(body, &f))
	assert.Equal(t, StatusModified, f.Status)
	assert.Equal(t, "|pg> SELECT 2", f.BufferContents)
}

func TestAPI_SaveWritesToDisk(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte("|pg> SELECT 1"), 0644))
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})
	_, _ = postJSON(t, base, "/api/files/edit", editRequest{
		Path: simpPath, BufferContents: "|pg> SELECT 42",
	})

	code, body := postJSON(t, base, "/api/files/save", saveRequest{Path: simpPath})
	assert.Equal(t, http.StatusOK, code)
	var f OpenFile
	assert.NoError(t, json.Unmarshal(body, &f))
	assert.Equal(t, StatusClean, f.Status)

	onDisk, _ := os.ReadFile(simpPath)
	assert.Equal(t, "|pg> SELECT 42", string(onDisk))
}

func TestAPI_CloseRemovesFromList(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte(""), 0644))
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})

	code, _ := postJSON(t, base, "/api/files/close", closeRequest{Path: simpPath})
	assert.Equal(t, http.StatusOK, code)

	code, body := getBody(t, base+"/api/files")
	assert.Equal(t, http.StatusOK, code)
	var list filesListResponse
	assert.NoError(t, json.Unmarshal(body, &list))
	assert.Equal(t, 0, len(list.Files))
}

func TestAPI_GetFileReturnsFullState(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	simpPath := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(simpPath, []byte("|pg> SELECT 1"), 0644))
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: simpPath})

	code, body := getBody(t, base+"/api/files/get?path="+simpPath)
	assert.Equal(t, http.StatusOK, code)
	var f OpenFile
	assert.NoError(t, json.Unmarshal(body, &f))
	assert.Equal(t, simpPath, f.Path)
	assert.Equal(t, "|pg> SELECT 1", f.BufferContents)
}

func TestAPI_ListFilesReflectsActive(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	a := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(a, []byte(""), 0644))
	b := filepath.Join(t.TempDir(), "b.simp")
	assert.NoError(t, os.WriteFile(b, []byte(""), 0644))

	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: a})
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: b})

	code, body := getBody(t, base+"/api/files")
	assert.Equal(t, http.StatusOK, code)
	var list filesListResponse
	assert.NoError(t, json.Unmarshal(body, &list))
	assert.Equal(t, 2, len(list.Files))
	assert.Equal(t, a, list.Active, "first-opened stays active")
}

func TestAPI_SwitchActive(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	a := filepath.Join(t.TempDir(), "a.simp")
	assert.NoError(t, os.WriteFile(a, []byte(""), 0644))
	b := filepath.Join(t.TempDir(), "b.simp")
	assert.NoError(t, os.WriteFile(b, []byte(""), 0644))
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: a})
	_, _ = postJSON(t, base, "/api/files/open", openRequest{Path: b})

	code, _ := postJSON(t, base, "/api/files/switch-active", switchActiveRequest{Path: b})
	assert.Equal(t, http.StatusOK, code)

	_, body := getBody(t, base+"/api/files")
	var list filesListResponse
	assert.NoError(t, json.Unmarshal(body, &list))
	assert.Equal(t, b, list.Active)
}

func TestAPI_MethodNotAllowed(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	resp, err := http.Get(base + "/api/files/open")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestAPI_InvalidJSONBody(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	resp, err := http.Post(base+"/api/files/open", "application/json", bytes.NewReader([]byte("{not json")))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
