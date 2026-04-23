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

	"github.com/stretchr/testify/assert"
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

func suggestionTexts(suggestions []Suggestion) []string {
	out := make([]string, len(suggestions))
	for i, s := range suggestions {
		out[i] = s.Text
	}
	return out
}

func containsAllSuggestions(texts []string, want []string) bool {
	have := map[string]bool{}
	for _, t := range texts {
		have[t] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

func TestComputeSuggestions_StageStart(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
		{Key: "mg", URI: "mongodb://h/db"},
	})
	got := ComputeSuggestions(ContextClassification{Context: CtxStageStart, Prefix: ""})
	texts := suggestionTexts(got)
	if !containsAllSuggestions(texts, []string{"mg", "pg"}) {
		t.Fatalf("want both pg and mg; got %v", texts)
	}
	// Kind is always ConnectionLabel for this branch.
	for _, s := range got {
		if s.Kind != SuggestionConnectionLabel {
			t.Fatalf("want ConnectionLabel kind, got %q", s.Kind)
		}
	}
}

func TestComputeSuggestions_SqlKeywordPrefix(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
		{Key: "my", URI: "mysql://h/db"},
	})
	cases := []struct {
		name   string
		label  string
		prefix string
		want   []string
	}{
		{"postgres SEL prefix", "pg", "SEL", []string{"SELECT"}},
		{"mysql SH prefix", "my", "SH", []string{"SHOW"}},
		{"empty prefix returns cross-dialect set", "pg", "", []string{"SELECT", "FROM", "WHERE"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeSuggestions(ContextClassification{
				Context:         CtxSqlKeywordPrefix,
				ConnectionLabel: tc.label,
				Prefix:          tc.prefix,
			})
			if !containsAllSuggestions(suggestionTexts(got), tc.want) {
				t.Fatalf("want %v, got %v", tc.want, suggestionTexts(got))
			}
		})
	}
}

func TestComputeSuggestions_SqlTableExpected(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return nil, nil // no live DB
	})

	// No cache: empty.
	got := ComputeSuggestions(ContextClassification{
		Context:         CtxSqlTableExpected,
		ConnectionLabel: "pg",
	})
	if len(got) != 0 {
		t.Fatalf("no cache must yield empty; got %v", suggestionTexts(got))
	}

	// Populated cache: databases and tables returned.
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "pg",
		PopulatedAt:     timePtr(time.Now()),
		Databases: []DatabaseSchema{
			{Name: "analytics", Tables: []TableSchema{{Name: "users", Columns: []string{"id"}}, {Name: "events", Columns: []string{"id"}}}},
			{Name: "reporting", Tables: []TableSchema{{Name: "daily", Columns: []string{"day"}}}},
		},
	}))

	got = ComputeSuggestions(ContextClassification{
		Context:         CtxSqlTableExpected,
		ConnectionLabel: "pg",
	})
	if !containsAllSuggestions(suggestionTexts(got), []string{"analytics", "reporting", "users", "events", "daily"}) {
		t.Fatalf("want databases + tables; got %v", suggestionTexts(got))
	}
}

func TestComputeSuggestions_SqlColumnExpected(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "pg",
		PopulatedAt:     timePtr(time.Now()),
		Databases: []DatabaseSchema{
			{Name: "app", Tables: []TableSchema{
				{Name: "users", Columns: []string{"id", "email", "created_at"}},
				{Name: "orders", Columns: []string{"id", "user_id", "total"}},
			}},
		},
	}))

	// alias.col prefix resolves via alias map.
	got := ComputeSuggestions(ContextClassification{
		Context:         CtxSqlColumnExpected,
		ConnectionLabel: "pg",
		SqlAliases:      map[string]string{"u": "users"},
		Prefix:          "u.",
	})
	if !containsAllSuggestions(suggestionTexts(got), []string{"id", "email", "created_at"}) {
		t.Fatalf("alias-scoped columns: got %v", suggestionTexts(got))
	}
	for _, s := range suggestionTexts(got) {
		if s == "user_id" {
			t.Fatalf("must NOT include columns from orders table; got %v", suggestionTexts(got))
		}
	}

	// Bare prefix unions columns across scope.
	got = ComputeSuggestions(ContextClassification{
		Context:         CtxSqlColumnExpected,
		ConnectionLabel: "pg",
		SqlAliases:      map[string]string{"u": "users", "o": "orders"},
		Prefix:          "",
	})
	if !containsAllSuggestions(suggestionTexts(got), []string{"id", "email", "total"}) {
		t.Fatalf("union of columns: got %v", suggestionTexts(got))
	}

	// Unknown alias returns empty (not a fallback to union).
	got = ComputeSuggestions(ContextClassification{
		Context:         CtxSqlColumnExpected,
		ConnectionLabel: "pg",
		SqlAliases:      map[string]string{"u": "users"},
		Prefix:          "z.",
	})
	if len(got) != 0 {
		t.Fatalf("unknown alias must return empty; got %v", suggestionTexts(got))
	}
}

func TestComputeSuggestions_MongoDatabaseExpected(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "mg", URI: "mongodb://h/db"}})
	swapIntrospect(t, func(label, uri string, ct common.ConnType) (*SchemaCache, error) {
		return nil, nil
	})

	got := ComputeSuggestions(ContextClassification{Context: CtxMongoDatabaseExpected, ConnectionLabel: "mg"})
	if len(got) != 0 {
		t.Fatalf("cache null: expect empty; got %v", suggestionTexts(got))
	}

	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "mg",
		PopulatedAt:     timePtr(time.Now()),
		Databases: []DatabaseSchema{
			{Name: "app", Collections: []CollectionSchema{{Name: "users"}}},
			{Name: "analytics", Collections: []CollectionSchema{{Name: "events"}}},
		},
	}))
	got = ComputeSuggestions(ContextClassification{Context: CtxMongoDatabaseExpected, ConnectionLabel: "mg"})
	if !containsAllSuggestions(suggestionTexts(got), []string{"app", "analytics"}) {
		t.Fatalf("want both databases; got %v", suggestionTexts(got))
	}
}

func TestComputeSuggestions_MongoCollectionExpected(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "mg", URI: "mongodb://h/db"}})
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "mg",
		PopulatedAt:     timePtr(time.Now()),
		Databases: []DatabaseSchema{
			{Name: "app", Collections: []CollectionSchema{{Name: "users"}, {Name: "orders"}}},
			{Name: "analytics", Collections: []CollectionSchema{{Name: "events"}}},
		},
	}))

	got := ComputeSuggestions(ContextClassification{
		Context:         CtxMongoCollectionExpected,
		ConnectionLabel: "mg",
		Prefix:          "db.app.",
	})
	if !containsAllSuggestions(suggestionTexts(got), []string{"users", "orders"}) {
		t.Fatalf("want collections of app database; got %v", suggestionTexts(got))
	}
	for _, s := range suggestionTexts(got) {
		if s == "events" {
			t.Fatalf("must not include collections from other databases; got %v", suggestionTexts(got))
		}
	}
}

func TestComputeSuggestions_MongoOperationExpected(t *testing.T) {
	got := ComputeSuggestions(ContextClassification{Context: CtxMongoOperationExpected})
	want := []string{"find", "findOne", "aggregate", "insertOne"}
	if !containsAllSuggestions(suggestionTexts(got), want) {
		t.Fatalf("want %v, got %v", want, suggestionTexts(got))
	}
}

func TestComputeSuggestions_MongoFieldExpected(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "mg", URI: "mongodb://h/db"}})
	assert.NoError(t, SaveSchemaCache(&SchemaCache{
		ConnectionLabel: "mg",
		PopulatedAt:     timePtr(time.Now()),
		Databases: []DatabaseSchema{
			{Name: "app", Collections: []CollectionSchema{
				{Name: "users", Fields: []string{"email", "name", "created_at"}},
			}},
		},
	}))

	got := ComputeSuggestions(ContextClassification{
		Context:         CtxMongoFieldExpected,
		ConnectionLabel: "mg",
		Prefix:          "em",
	})
	if !containsAllSuggestions(suggestionTexts(got), []string{"email"}) {
		t.Fatalf("want email under em prefix; got %v", suggestionTexts(got))
	}
}

func TestComputeSuggestions_RedisCommandPrefix(t *testing.T) {
	cases := []struct {
		name   string
		prefix string
		want   []string
	}{
		{"GE prefix", "GE", []string{"GET"}},
		{"empty prefix", "", []string{"GET", "SET", "DEL"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeSuggestions(ContextClassification{Context: CtxRedisCommandPrefix, Prefix: tc.prefix})
			if !containsAllSuggestions(suggestionTexts(got), tc.want) {
				t.Fatalf("want %v, got %v", tc.want, suggestionTexts(got))
			}
		})
	}
}

func TestComputeSuggestions_JqPlaceholder(t *testing.T) {
	got := ComputeSuggestions(ContextClassification{Context: CtxJqPlaceholder, Prefix: ""})
	// M6 returns operators only; path-mining arrives in M7.
	if !containsAllSuggestions(suggestionTexts(got), []string{"select", "map", "length"}) {
		t.Fatalf("want jq operators; got %v", suggestionTexts(got))
	}
}

func TestComputeSuggestions_Unknown(t *testing.T) {
	got := ComputeSuggestions(ContextClassification{Context: CtxUnknown})
	if got != nil {
		t.Fatalf("unknown context must yield nil; got %v", got)
	}
}

func timePtr(t time.Time) *time.Time { return &t }

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
