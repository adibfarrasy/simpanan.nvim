package webui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func deleteJSON(t *testing.T, base, path string, body interface{}) (int, []byte) {
	t.Helper()
	buf, err := json.Marshal(body)
	assert.NoError(t, err)
	req, err := http.NewRequest(http.MethodDelete, base+path, bytes.NewReader(buf))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

func TestConnectionsAPI_ListEmpty(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	code, body := getBody(t, base+"/api/connections")
	assert.Equal(t, http.StatusOK, code)
	var resp connectionListResponse
	assert.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, 0, len(resp.Connections))
}

func TestConnectionsAPI_AddListDeleteRoundTrip(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	code, body := postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "pg", URI: "postgres://h/db",
	})
	assert.Equal(t, http.StatusOK, code, "add: %s", string(body))

	code, body = getBody(t, base+"/api/connections")
	assert.Equal(t, http.StatusOK, code)
	var listResp connectionListResponse
	assert.NoError(t, json.Unmarshal(body, &listResp))
	assert.Equal(t, 1, len(listResp.Connections))
	assert.Equal(t, "pg", listResp.Connections[0].Label)
	assert.Equal(t, "postgres://h/db", listResp.Connections[0].URI)

	code, _ = deleteJSON(t, base, "/api/connections", deleteConnectionRequest{Label: "pg"})
	assert.Equal(t, http.StatusOK, code)

	code, body = getBody(t, base+"/api/connections")
	assert.Equal(t, http.StatusOK, code)
	assert.NoError(t, json.Unmarshal(body, &listResp))
	assert.Equal(t, 0, len(listResp.Connections))
}

func TestConnectionsAPI_AddRejectsInvalidUri(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	code, body := postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "x", URI: "ftp://nope",
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "Unrecognised uri scheme")
}

func TestConnectionsAPI_AddRejectsReservedJqLabel(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	code, body := postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "jq", URI: "postgres://h/db",
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "jq")
}

func TestConnectionsAPI_AddRejectsDuplicateLabel(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	code, _ := postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "pg", URI: "postgres://h/db",
	})
	assert.Equal(t, http.StatusOK, code)
	code, body := postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "pg", URI: "postgres://other/db",
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, string(body), "already exists")
}

func TestConnectionsAPI_DeleteUnknownIs404(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	code, body := deleteJSON(t, base, "/api/connections", deleteConnectionRequest{Label: "nope"})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Contains(t, string(body), "does not exist")
}

func TestConnectionsAPI_MethodNotAllowed(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	req, _ := http.NewRequest(http.MethodPut, base+"/api/connections", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
