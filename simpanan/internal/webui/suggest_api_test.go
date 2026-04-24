package webui

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"simpanan/internal"

	"github.com/stretchr/testify/assert"
)

func TestSuggestAPI_EmptyBufferReturnsConnectionLabels(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()

	// Seed two connections so SuggestForBuffer's stage_start branch
	// has something to return.
	_, _ = postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "pg", URI: "postgres://h/db",
	})
	_, _ = postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "mg", URI: "mongodb://h/db",
	})

	code, body := postJSON(t, base, "/api/suggest", suggestRequest{
		BufferText: "", CursorByteOffset: 0,
	})
	assert.Equal(t, http.StatusOK, code)
	var resp suggestResponse
	assert.NoError(t, json.Unmarshal(body, &resp))

	labels := map[string]bool{}
	for _, s := range resp.Suggestions {
		assert.Equal(t, internal.SuggestionConnectionLabel, s.Kind)
		labels[s.Text] = true
	}
	assert.True(t, labels["pg"], "pg label must be among suggestions")
	assert.True(t, labels["mg"], "mg label must be among suggestions")
}

func TestSuggestAPI_SqlKeywordPrefixSurfacesSelect(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	_, _ = postJSON(t, base, "/api/connections", addConnectionRequest{
		Label: "pg", URI: "postgres://h/db",
	})

	buf := "|pg> SEL"
	code, body := postJSON(t, base, "/api/suggest", suggestRequest{
		BufferText: buf, CursorByteOffset: len(buf),
	})
	assert.Equal(t, http.StatusOK, code)
	var resp suggestResponse
	assert.NoError(t, json.Unmarshal(body, &resp))

	found := false
	for _, s := range resp.Suggestions {
		if s.Text == "SELECT" && s.Kind == internal.SuggestionSqlKeyword {
			found = true
			break
		}
	}
	assert.True(t, found, "SELECT must surface for 'pg> SEL'; got %+v", resp.Suggestions)
	_ = filepath.Join // silence import if Go vet ever complains
}

func TestSuggestAPI_BadBody(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	resp, err := http.Post(base+"/api/suggest", "application/json", nil)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSuggestAPI_MethodNotAllowed(t *testing.T) {
	base, _, stop := startTestServer(t)
	defer stop()
	resp, err := http.Get(base + "/api/suggest")
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}
