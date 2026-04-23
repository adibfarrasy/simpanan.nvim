package internal

import (
	"encoding/json"
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandleSuggest_RejectsMissingArgs(t *testing.T) {
	_, err := HandleSuggest([]string{"only-buffer-no-cursor"})
	assert.Error(t, err)
}

func TestHandleSuggest_RejectsNonIntegerCursor(t *testing.T) {
	_, err := HandleSuggest([]string{"pg> ", "not-a-number"})
	assert.Error(t, err)
}

func TestHandleSuggest_ReturnsJsonArray(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})

	out, err := HandleSuggest([]string{"pg> SEL", "7"})
	assert.NoError(t, err)

	var suggestions []Suggestion
	assert.NoError(t, json.Unmarshal([]byte(out), &suggestions))

	found := false
	for _, s := range suggestions {
		if s.Text == "SELECT" && s.Kind == SuggestionSqlKeyword {
			found = true
			break
		}
	}
	assert.True(t, found, "HandleSuggest must surface SELECT for 'pg> SEL'; got %s", out)
}

func TestHandleSuggest_EmptyBufferYieldsStageLabels(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
		{Key: "mg", URI: "mongodb://h/db"},
	})
	out, err := HandleSuggest([]string{"", "0"})
	assert.NoError(t, err)
	var suggestions []Suggestion
	assert.NoError(t, json.Unmarshal([]byte(out), &suggestions))
	assert.Equal(t, 2, len(suggestions))
	for _, s := range suggestions {
		assert.Equal(t, SuggestionConnectionLabel, s.Kind)
	}
}
