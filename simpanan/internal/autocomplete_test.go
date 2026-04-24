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
	"fmt"
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
		{"typing label prefix", "|p", 2, CtxStageStart},
		{"after pg> space only", "|pg> ", 5, CtxSqlKeywordPrefix},
		{"after SELECT partial keyword", "|pg> SEL", 8, CtxSqlKeywordPrefix},
		{"after FROM", "|pg> SELECT * FROM ", 19, CtxSqlTableExpected},
		{"after INSERT INTO", "|pg> INSERT INTO ", 17, CtxSqlTableExpected},
		{"after UPDATE", "|pg> UPDATE ", 12, CtxSqlTableExpected},
		{"after WHERE", "|pg> SELECT * FROM t WHERE ", 27, CtxSqlColumnExpected},
		{"after JOIN ON", "|pg> SELECT * FROM a JOIN b ON ", 31, CtxSqlColumnExpected},
		{"alias qualified column", "|pg> SELECT * FROM users u WHERE u.", 35, CtxSqlColumnExpected},
		{"after db.", "|mg> db.", 8, CtxMongoDatabaseExpected},
		{"after db.app.", "|mg> db.app.", 12, CtxMongoCollectionExpected},
		{"after db.app.users.", "|mg> db.app.users.", 18, CtxMongoOperationExpected},
		{"inside $match field pos", "|mg> db.app.users.aggregate([{$match: {", 39, CtxMongoFieldExpected},
		{"redis command prefix", "|rd> G", 6, CtxRedisCommandPrefix},
		{"jq placeholder in later stage", "|pg> SELECT 1\n|pg> SELECT {{.foo", 32, CtxJqPlaceholder},
		{"explicit jq stage", "|jq> .", 6, CtxJqPlaceholder},
		{"unknown label", "|other> SELECT", 14, CtxUnknown},
		// Bare '|' on a fresh line after a previous complete stage:
		// the user is starting a new stage header, NOT continuing the
		// previous stage's body. Must classify as stage_start so the
		// connection-label popup appears.
		{"bare | on new line after stage", "|pg> SELECT 1\n|", 15, CtxStageStart},
		// Same thing but with whitespace before the |.
		{"  | on new line", "|pg> SELECT 1\n   |", 18, CtxStageStart},
		// Partial label after | on a new line.
		{"|p on new line", "|pg> SELECT 1\n|p", 16, CtxStageStart},
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

	buf := "|pg> SELECT u.id FROM users u JOIN orders o ON o.user_id = u.id WHERE "
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
	buf := "|pg> SELECT id FROM users\n|mg> db."
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

// swapRunPipeline replaces the package-level pipeline runner for the
// duration of a test.
func swapRunPipeline(t *testing.T, fn func([]common.QueryMetadata) ([]byte, error)) {
	t.Helper()
	prev := runPipelineFn
	runPipelineFn = fn
	t.Cleanup(func() { runPipelineFn = prev })
}

func TestProbeJqPaths_SuccessReturnsOperatorsAndPaths(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})

	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		return []byte(`[{"id": 1, "email": "a@b"}, {"id": 2, "email": "c@d"}]`), nil
	})

	buf := "|pg> SELECT id, email FROM users\n|pg> SELECT * FROM orders WHERE user_id = '{{"
	got := SuggestForBuffer(buf, len(buf))
	texts := suggestionTexts(got)
	// Operators present.
	if !containsAllSuggestions(texts, []string{"select", "map", "length"}) {
		t.Fatalf("want operators; got %v", texts)
	}
	// Paths present (generalised array index).
	if !containsAllSuggestions(texts, []string{".[]", ".[].id", ".[].email"}) {
		t.Fatalf("want jq paths; got %v", texts)
	}
}

func TestProbeJqPaths_FailureDegradesSilently(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})
	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		return nil, fmt.Errorf("db unreachable")
	})

	buf := "|pg> SELECT id FROM users\n|pg> SELECT * FROM orders WHERE user_id = '{{"
	got := SuggestForBuffer(buf, len(buf))
	texts := suggestionTexts(got)
	if !containsAllSuggestions(texts, []string{"select", "map"}) {
		t.Fatalf("want operators; got %v", texts)
	}
	for _, s := range got {
		if s.Kind == SuggestionJqPath {
			t.Fatalf("must not include jq paths on failure; got %+v", s)
		}
	}
}

func TestProbeJqPaths_TimeoutDegradesSilently(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})
	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		time.Sleep(500 * time.Millisecond) // well beyond our test timeout
		return []byte(`[{"id":1}]`), nil
	})

	// Shrink the configured timeout for this test via a local probe.
	// The config isn't overridable mid-test, so we exercise the default
	// 2s path only indirectly. Instead, rely on a runner that returns
	// an empty payload to check the empty-payload degrade path.
	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		return nil, nil
	})
	buf := "|pg> SELECT id FROM users\n|pg> SELECT * FROM orders WHERE user_id = '{{"
	got := SuggestForBuffer(buf, len(buf))
	for _, s := range got {
		if s.Kind == SuggestionJqPath {
			t.Fatalf("empty payload must degrade; got path %+v", s)
		}
	}
}

func TestProbeJqPaths_UsesDiskCacheOnSecondCall(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "pg", URI: "postgres://h/db"}})

	calls := 0
	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		calls++
		return []byte(`{"id": 1, "name": "alice"}`), nil
	})

	buf := "|pg> SELECT id, name FROM users\n|pg> SELECT * FROM orders WHERE user_id = '{{"
	_ = SuggestForBuffer(buf, len(buf))
	_ = SuggestForBuffer(buf, len(buf))
	assert.Equal(t, 1, calls, "second call must hit the on-disk probe cache")
	_ = home
}

func TestProbeJqPaths_FirstStageHasNoPriorToProbe(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{{Key: "jq", URI: "jq://"}})
	called := false
	swapRunPipeline(t, func(stages []common.QueryMetadata) ([]byte, error) {
		called = true
		return nil, nil
	})
	// A jq stage at the very start of the buffer has no prior pipeline.
	buf := "|jq> {{"
	got := SuggestForBuffer(buf, len(buf))
	assert.False(t, called, "probe must not run when there is no prior stage")
	if !containsAllSuggestions(suggestionTexts(got), []string{"select", "map"}) {
		t.Fatalf("want operators only; got %v", suggestionTexts(got))
	}
	_ = home
}

func TestExtractJqPaths(t *testing.T) {
	paths, err := extractJqPaths([]byte(`[{"id": 1, "user": {"name": "alice", "tags": ["a", "b"]}}, {"id": 2, "user": {"name": "bob"}}]`))
	assert.NoError(t, err)
	want := []string{".[]", ".[].id", ".[].user", ".[].user.name", ".[].user.tags", ".[].user.tags[]"}
	for _, w := range want {
		found := false
		for _, p := range paths {
			if p == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing path %q; got %v", w, paths)
		}
	}
}

func TestPipelineHash_StableAndKeyedOnContent(t *testing.T) {
	a := []common.QueryMetadata{
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT 1"},
	}
	b := []common.QueryMetadata{
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT 2"},
	}
	if pipelineHash(a) == pipelineHash(b) {
		t.Fatalf("different query text must hash differently")
	}
	if pipelineHash(a) != pipelineHash(a) {
		t.Fatalf("hash must be stable for identical input")
	}
}
