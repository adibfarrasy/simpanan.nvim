package webui

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteAPI_RejectsEmptySelection(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	code, body := postJSON(t, base, "/api/execute", executeRequest{Selection: "  \n  "})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "selection is empty")
}

func TestExecuteAPI_RejectsInvalidJsonBody(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	resp, err := http.Post(base+"/api/execute", "application/json", nil)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestExecuteAPI_NonExistentConnectionFoldsErrorIntoResult(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	// No connection registered → RunPipeline returns ("Error: …", nil).
	code, body := postJSON(t, base, "/api/execute", executeRequest{
		Selection: "missing> SELECT 1",
	})
	assert.Equal(t, http.StatusOK, code)

	var resp executeResponse
	assert.NoError(t, json.Unmarshal(body, &resp))
	assert.Contains(t, resp.Result, "Error:")
	assert.Contains(t, resp.Result, "missing")
}

func TestExecuteAPI_MethodNotAllowed(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	resp, err := http.Get(base + "/api/execute")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
