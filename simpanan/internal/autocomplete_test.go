//go:build autocomplete

package internal

// Skeleton tests for the unimplemented autocomplete feature described
// in specs/simpanan.allium (rules ClassifyContext / ComputeSuggestions
// / ProbeJqPaths, entity BuiltinCatalog, enum CompletionContext, enum
// SuggestionKind, value Suggestion).
//
// Gated behind the `autocomplete` build tag — see
// schema_cache_test.go for the rationale. Every test starts with
// t.Skip; cases tables are filled in where the spec makes the inputs
// and expected outputs concrete.

import (
	"testing"
	"time"
)

// TODO: signature to be finalised during implementation
// type Suggestion struct { Text string; Kind SuggestionKind }
// type SuggestionKind string
// type CompletionContext string
// type ContextClassification struct {
//     Context         CompletionContext
//     Prefix          string
//     StageIndex      int
//     ConnectionLabel string
//     SqlAliases      map[string]string
// }
// type BuiltinCatalog struct {
//     ConnectionType             common.ConnType
//     SqlKeywords                []string
//     RedisCommands              []string
//     MongoAggregationOperators  []string
//     MongoCollectionOperations  []string
//     JqOperators                []string
// }

// -----------------------------------------------------------------------
// entity BuiltinCatalog — defaults per connection type (one default
// instance per type; see `default BuiltinCatalog *_catalog` in the spec).
// -----------------------------------------------------------------------

func TestBuiltinCatalogDefaults(t *testing.T) {
	t.Skip("autocomplete not implemented: entity BuiltinCatalog / default catalogs")

	// TODO: signature to be finalised during implementation
	// GetBuiltinCatalog(ct common.ConnType) BuiltinCatalog

	cases := []struct {
		name                string
		connType            string
		wantSqlKeyword      string // a representative keyword that must be present
		wantMongoOp         string
		wantMongoAggOp      string
		wantRedisCmd        string
		wantJqOperator      string
	}{
		{name: "postgres catalog", connType: "postgres", wantSqlKeyword: "SELECT"},
		{name: "mysql catalog", connType: "mysql", wantSqlKeyword: "SHOW"},
		{name: "mongo catalog", connType: "mongo", wantMongoOp: "find", wantMongoAggOp: "$match"},
		{name: "redis catalog", connType: "redis", wantRedisCmd: "GET"},
		{name: "jq catalog", connType: "jq", wantJqOperator: "select"},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// rule ClassifyContext — black-box parser; the spec only constrains the
// shape of the output. The skeleton below records the known contextual
// branches; grammar-level assertions are explicitly out of scope (see
// spec Excludes).
// -----------------------------------------------------------------------

func TestClassifyContext(t *testing.T) {
	t.Skip("autocomplete not implemented: rule ClassifyContext (grammar is a spec-declared black box)")

	// TODO: signature to be finalised during implementation
	// ClassifyContext(bufferText string, cursorPos int) ContextClassification

	// Representative cursor scenarios. Actual grammar is
	// implementation-defined, so only a handful of unambiguous
	// positions are pinned down here.
	cases := []struct {
		name     string
		buf      string
		cursor   int
		wantCtx  string // CompletionContext variant name
	}{
		{"empty buffer -> stage_start", "", 0, "stage_start"},
		{"after pg> (no query yet) -> sql_keyword_prefix", "pg> ", 4, "sql_keyword_prefix"},
		{"after FROM -> sql_table_expected", "pg> SELECT * FROM ", 18, "sql_table_expected"},
		{"after WHERE -> sql_column_expected", "pg> SELECT * FROM t WHERE ", 26, "sql_column_expected"},
		{"after db. -> mongo_database_expected", "mg> db.", 7, "mongo_database_expected"},
		{"inside jq placeholder -> jq_placeholder", "pg> SELECT 1\npg> SELECT {{.foo}}", 28, "jq_placeholder"},
	}
	_ = cases
}

// -----------------------------------------------------------------------
// rule ComputeSuggestions — every enum variant of CompletionContext is
// a test obligation. Each sub-test below covers one branch.
// -----------------------------------------------------------------------

func TestComputeSuggestions_StageStart(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/stage_start (enum branch)")

	// Obligation: returns connection_label_suggestions over the full
	// registry.
	cases := []struct {
		name              string
		registeredLabels  []string
		wantLabels        []string
	}{
		{"empty registry", nil, nil},
		{"two connections", []string{"pg", "mg"}, []string{"pg", "mg"}},
	}
	_ = cases
}

func TestComputeSuggestions_SqlKeywordPrefix(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/sql_keyword_prefix (enum branch)")

	// Obligation: returns catalog.sql_keywords filtered by prefix.
	cases := []struct {
		name     string
		connType string
		prefix   string
		want     []string // representative expected keywords
	}{
		{"postgres SEL prefix", "postgres", "SEL", []string{"SELECT"}},
		{"mysql SH prefix", "mysql", "SH", []string{"SHOW"}},
		{"empty prefix returns all", "postgres", "", []string{"SELECT", "FROM", "WHERE"}},
	}
	_ = cases
}

func TestComputeSuggestions_SqlTableExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/sql_table_expected (enum branch)")

	// Obligation: returns Database + Table suggestions across the
	// cache; empty when cache is null.
	cases := []struct {
		name       string
		hasCache   bool
		wantNonEmpty bool
	}{
		{"no cache returns empty", false, false},
		{"populated cache returns tables", true, true},
	}
	_ = cases
}

func TestComputeSuggestions_SqlColumnExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/sql_column_expected (enum branch)")

	// Obligation: if prefix is "alias.col", resolve alias to one
	// table's columns; else union columns across all FROM/JOIN tables
	// in scope.
	cases := []struct {
		name    string
		aliases map[string]string
		prefix  string
	}{
		{"alias.col prefix resolves via alias map", map[string]string{"u": "users"}, "u."},
		{"bare prefix unions columns across scope", map[string]string{"u": "users", "o": "orders"}, ""},
		{"unknown alias returns empty", map[string]string{"u": "users"}, "z."},
	}
	_ = cases
}

func TestComputeSuggestions_MongoDatabaseExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/mongo_database_expected (enum branch)")

	cases := []struct {
		name     string
		hasCache bool
	}{
		{"cache null returns empty", false},
		{"cache populated returns database names", true},
	}
	_ = cases
}

func TestComputeSuggestions_MongoCollectionExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/mongo_collection_expected (enum branch)")

	// Obligation: returns collections under the qualified database.
}

func TestComputeSuggestions_MongoOperationExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/mongo_operation_expected (enum branch)")

	// Obligation: returns catalog.mongo_collection_operations
	// (find, findOne, aggregate, insertOne, ...).
	wantRepresentative := []string{"find", "findOne", "aggregate", "insertOne"}
	_ = wantRepresentative
}

func TestComputeSuggestions_MongoFieldExpected(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/mongo_field_expected (enum branch)")

	// Obligation: returns the collection's field set, filtered by prefix.
}

func TestComputeSuggestions_RedisCommandPrefix(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/redis_command_prefix (enum branch)")

	cases := []struct {
		name   string
		prefix string
		want   []string
	}{
		{"GE prefix narrows to GET", "GE", []string{"GET"}},
		{"empty prefix returns full command set", "", []string{"GET", "SET", "DEL"}},
	}
	_ = cases
}

func TestComputeSuggestions_JqPlaceholder(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/jq_placeholder (enum branch — fans out to ProbeJqPaths)")

	// Obligation: does NOT return suggestions synchronously; emits
	// JqPathProbeRequested instead.
}

func TestComputeSuggestions_Unknown(t *testing.T) {
	t.Skip("autocomplete not implemented: ComputeSuggestions/unknown (enum branch)")

	// Obligation: SuggestionsComputed with an empty set.
}

// -----------------------------------------------------------------------
// rule ProbeJqPaths
//
// Probes the prior sub-pipeline read-only with jq_path_probe_timeout.
// On success, returns jq operators + paths mined from the JSON result.
// On any failure / timeout, returns just the operators (silent
// degradation).
// -----------------------------------------------------------------------

func TestProbeJqPaths(t *testing.T) {
	t.Skip("autocomplete not implemented: rule ProbeJqPaths (needs clock-injection and process seam)")

	// TODO: signature to be finalised during implementation
	// ProbeJqPaths(ctx context.Context, bufferTextPrefix string, prefix string, jqOperators []string, timeout time.Duration) []Suggestion

	cases := []struct {
		name            string
		probeSucceeds   bool
		probeTimesOut   bool
		wantIncludesOps bool
		wantIncludesPaths bool
	}{
		{
			name:              "success: operators + paths",
			probeSucceeds:     true,
			wantIncludesOps:   true,
			wantIncludesPaths: true,
		},
		{
			name:              "failure: only operators (silent degrade)",
			probeSucceeds:     false,
			wantIncludesOps:   true,
			wantIncludesPaths: false,
		},
		{
			name:              "timeout: only operators (silent degrade)",
			probeTimesOut:     true,
			wantIncludesOps:   true,
			wantIncludesPaths: false,
		},
	}
	_ = cases
	_ = time.Second
}
