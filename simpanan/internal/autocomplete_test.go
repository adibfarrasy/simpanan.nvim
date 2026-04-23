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
	"simpanan/internal/common"
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
	cases := []struct {
		name           string
		connType       common.ConnType
		wantSqlKeyword string // a representative keyword that must be present
		wantMongoOp    string
		wantMongoAggOp string
		wantRedisCmd   string
		wantJqOperator string
	}{
		{name: "postgres catalog", connType: common.Postgres, wantSqlKeyword: "SELECT"},
		{name: "mysql catalog", connType: common.Mysql, wantSqlKeyword: "SHOW"},
		{name: "mongo catalog", connType: common.Mongo, wantMongoOp: "find", wantMongoAggOp: "$match"},
		{name: "redis catalog", connType: common.Redis, wantRedisCmd: "GET"},
		{name: "jq catalog", connType: common.Jq, wantJqOperator: "select"},
	}
	contains := func(haystack []string, needle string) bool {
		for _, h := range haystack {
			if h == needle {
				return true
			}
		}
		return false
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cat := GetBuiltinCatalog(tc.connType)
			if cat.ConnectionType != tc.connType {
				t.Fatalf("ConnectionType: want %q, got %q", tc.connType, cat.ConnectionType)
			}
			if tc.wantSqlKeyword != "" && !contains(cat.SqlKeywords, tc.wantSqlKeyword) {
				t.Fatalf("SqlKeywords must contain %q", tc.wantSqlKeyword)
			}
			if tc.wantMongoOp != "" && !contains(cat.MongoCollectionOperations, tc.wantMongoOp) {
				t.Fatalf("MongoCollectionOperations must contain %q", tc.wantMongoOp)
			}
			if tc.wantMongoAggOp != "" && !contains(cat.MongoAggregationOperators, tc.wantMongoAggOp) {
				t.Fatalf("MongoAggregationOperators must contain %q", tc.wantMongoAggOp)
			}
			if tc.wantRedisCmd != "" && !contains(cat.RedisCommands, tc.wantRedisCmd) {
				t.Fatalf("RedisCommands must contain %q", tc.wantRedisCmd)
			}
			if tc.wantJqOperator != "" && !contains(cat.JqOperators, tc.wantJqOperator) {
				t.Fatalf("JqOperators must contain %q", tc.wantJqOperator)
			}
		})
	}
}

// -----------------------------------------------------------------------
// rule ClassifyContext — black-box parser; the spec only constrains the
// shape of the output. The skeleton below records the known contextual
// branches; grammar-level assertions are explicitly out of scope (see
// spec Excludes).
// -----------------------------------------------------------------------

func TestClassifyContext(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
		{Key: "mg", URI: "mongodb://h/db"},
		{Key: "rd", URI: "redis://h"},
	})

	cases := []struct {
		name    string
		buf     string
		cursor  int
		wantCtx CompletionContext
	}{
		{"empty buffer", "", 0, CtxStageStart},
		{"typing label prefix", "p", 1, CtxStageStart},
		{"after pg> space only", "pg> ", 4, CtxSqlKeywordPrefix},
		{"after SELECT partial keyword", "pg> SEL", 7, CtxSqlKeywordPrefix},
		{"after FROM", "pg> SELECT * FROM ", 18, CtxSqlTableExpected},
		{"after INSERT INTO", "pg> INSERT INTO ", 16, CtxSqlTableExpected},
		{"after UPDATE", "pg> UPDATE ", 11, CtxSqlTableExpected},
		{"after WHERE", "pg> SELECT * FROM t WHERE ", 26, CtxSqlColumnExpected},
		{"after JOIN ON", "pg> SELECT * FROM a JOIN b ON ", 30, CtxSqlColumnExpected},
		{"alias qualified column", "pg> SELECT * FROM users u WHERE u.", 34, CtxSqlColumnExpected},
		{"after db.", "mg> db.", 7, CtxMongoDatabaseExpected},
		{"after db.app.", "mg> db.app.", 11, CtxMongoCollectionExpected},
		{"after db.app.users.", "mg> db.app.users.", 17, CtxMongoOperationExpected},
		{"inside $match field pos", "mg> db.app.users.aggregate([{$match: {", 38, CtxMongoFieldExpected},
		{"redis command prefix", "rd> G", 5, CtxRedisCommandPrefix},
		{"jq placeholder in later stage", "pg> SELECT 1\npg> SELECT {{.foo", 30, CtxJqPlaceholder},
		{"explicit jq stage", "jq> .", 5, CtxJqPlaceholder},
		{"unknown label", "other> SELECT", 13, CtxUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyContext(tc.buf, tc.cursor)
			if got.Context != tc.wantCtx {
				t.Fatalf("want %q, got %q (prefix=%q, label=%q)",
					tc.wantCtx, got.Context, got.Prefix, got.ConnectionLabel)
			}
		})
	}
}

func TestClassifyContext_SqlAliasesExtracted(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})

	buf := "pg> SELECT u.id FROM users u JOIN orders o ON o.user_id = u.id WHERE "
	got := ClassifyContext(buf, len(buf))
	if got.Context != CtxSqlColumnExpected {
		t.Fatalf("want sql_column_expected, got %q", got.Context)
	}
	if got.SqlAliases["u"] != "users" {
		t.Fatalf("alias u must map to users; got %q", got.SqlAliases["u"])
	}
	if got.SqlAliases["o"] != "orders" {
		t.Fatalf("alias o must map to orders; got %q", got.SqlAliases["o"])
	}
}

func TestClassifyContext_MultiStageUsesCorrectConnection(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
		{Key: "mg", URI: "mongodb://h/db"},
	})
	buf := "pg> SELECT id FROM users\nmg> db."
	got := ClassifyContext(buf, len(buf))
	if got.Context != CtxMongoDatabaseExpected {
		t.Fatalf("want mongo_database_expected, got %q", got.Context)
	}
	if got.ConnectionLabel != "mg" {
		t.Fatalf("want label=mg, got %q", got.ConnectionLabel)
	}
	if got.StageIndex != 1 {
		t.Fatalf("want stage_index=1, got %d", got.StageIndex)
	}
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
