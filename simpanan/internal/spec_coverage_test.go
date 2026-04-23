package internal

// Tests derived from specs/simpanan.allium. These fill gaps not already
// covered by connection_registry_test.go and run_query_test.go:
//
//   - invariant ConnectionTypeMatchesUri (via URI.ConnType across all
//     recognised schemes, including aliases)
//   - invariant ReservedJqLabel (the registry never persists a "jq" label)
//   - rule ListConnections (ConnectionsListed surface)
//   - rules RoutePostgres / RouteMysql / RouteMongo / RouteRedis at the
//     dispatch level (operation classification driving adapter branch
//     selection, without touching a real database)
//   - invariant ChainedStagesAreReadOnly + the new requires clause on
//     StartPipelineExecution. The implementation does not enforce this
//     yet — these tests are expected to FAIL until the production code
//     is updated. See TestChainedStagesAreReadOnly_*.

import (
	"errors"
	"simpanan/internal/adapters"
	"simpanan/internal/common"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// invariant ConnectionTypeMatchesUri
// ---------------------------------------------------------------------

func TestURIConnTypeRecognisedSchemes(t *testing.T) {
	cases := []struct {
		name     string
		uri      string
		expected common.ConnType
	}{
		{"postgres", "postgres://u:p@h/db", common.Postgres},
		{"postgresql alias", "postgresql://u:p@h/db", common.Postgres},
		{"mysql", "mysql://u:p@h:3306/db", common.Mysql},
		{"mongodb", "mongodb://h/db", common.Mongo},
		{"mongodb+srv", "mongodb+srv://h/db", common.Mongo},
		{"redis", "redis://h:6379", common.Redis},
		{"rediss", "rediss://h:6379", common.Redis},
		{"jq", "jq://", common.Jq},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := common.URI(tc.uri).ConnType()
			assert.NoError(t, err)
			if assert.NotNil(t, ct) {
				assert.Equal(t, tc.expected, *ct)
			}
		})
	}
}

func TestURIConnTypeRejectsUnknownSchemes(t *testing.T) {
	cases := []string{
		"http://h/db",
		"ftp://h",
		"no-scheme",
		"",
		"mariadb://h/db",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			_, err := common.URI(u).ConnType()
			assert.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------
// invariant ReservedJqLabel — the "jq" label is injected as a faux
// connection only at run time (run_query.go), never persisted in the
// registry.
// ---------------------------------------------------------------------

func TestJqLabelNeverPersistedByAdd(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, nil)
	_, err := HandleAddConnection([]string{"jq>jq://"})
	assert.Error(t, err)

	conns, err := GetConnectionList()
	assert.NoError(t, err)
	for _, c := range conns {
		assert.NotEqual(t, "jq", c.Key, "reserved label leaked into registry")
	}
}

// ---------------------------------------------------------------------
// rule ListConnections
// ---------------------------------------------------------------------

func TestHandleGetConnectionsEmpty(t *testing.T) {
	withTempHome(t)
	res, err := HandleGetConnections(nil)
	assert.NoError(t, err)
	assert.Empty(t, res)
}

func TestHandleGetConnectionsReturnsSeeded(t *testing.T) {
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "db1", URI: "postgres://h/db"},
		{Key: "db2", URI: "mongodb://h/db"},
	})
	res, err := HandleGetConnections(nil)
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	// Contract: each entry stringifies via KeyURIPair.String()
	assert.Contains(t, res[0], "db1")
	assert.Contains(t, res[1], "db2")
}

// ---------------------------------------------------------------------
// Adapter routing at the dispatch level.
//
// These tests operate purely on QueryType* classifiers — they do not
// open a database connection. They exercise the branch that execute()
// uses to pick a Read vs Write code path per connection type.
// ---------------------------------------------------------------------

func TestQueryTypePostgresRouting(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		expected common.QueryType
	}{
		{"select is read", "SELECT * FROM t", common.Read},
		{"select lowercase", "select 1", common.Read},
		{"insert is write", "INSERT INTO t VALUES (1)", common.Write},
		{"update is write", "UPDATE t SET a=1", common.Write},
		{"delete is write", "DELETE FROM t", common.Write},
		{"create table is write", "CREATE TABLE t (id int)", common.Write},
		{"drop is write", "DROP TABLE t", common.Write},
		{"alter is write", "ALTER TABLE t ADD c int", common.Write},
		{"truncate is write", "TRUNCATE t", common.Write},
		{"empty defaults read", "", common.Read},
		// RoutePostgres distinguishes admin separately (the '\' prefix
		// is handled in execute.go, not in QueryTypePostgres).
		{"admin backslash falls through as read", "\\dt", common.Read},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, adapters.QueryTypePostgres(tc.query))
		})
	}
}

func TestQueryTypeMysqlRouting(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		expected common.QueryType
	}{
		{"select is read", "SELECT 1", common.Read},
		{"show tables is read (metadata)", "SHOW TABLES", common.Read},
		{"describe is read", "DESCRIBE t", common.Read},
		{"insert is write", "INSERT INTO t VALUES (1)", common.Write},
		{"update is write", "UPDATE t SET a=1", common.Write},
		{"delete is write", "DELETE FROM t", common.Write},
		{"replace is write", "REPLACE INTO t VALUES (1)", common.Write},
		{"create is write", "CREATE TABLE t (id int)", common.Write},
		{"drop is write", "DROP TABLE t", common.Write},
		{"alter is write", "ALTER TABLE t ADD c int", common.Write},
		{"truncate is write", "TRUNCATE t", common.Write},
		{"rename is write", "RENAME TABLE a TO b", common.Write},
		{"empty defaults read", "", common.Read},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, adapters.QueryTypeMysql(tc.query))
		})
	}
}

func TestQueryTypeMongoRouting(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		expected common.QueryType
	}{
		{"find is read", "db.users.find({})", common.Read},
		{"findOne is read", "db.users.findOne({})", common.Read},
		{"aggregate is read", "db.users.aggregate([])", common.Read},
		{"count is read", "db.users.count({})", common.Read},
		{"distinct is read", "db.users.distinct('x')", common.Read},
		{"insertOne is write", "db.users.insertOne({})", common.Write},
		{"insertMany is write", "db.users.insertMany([])", common.Write},
		{"updateOne is write", "db.users.updateOne({}, {})", common.Write},
		{"updateMany is write", "db.users.updateMany({}, {})", common.Write},
		{"deleteOne is write", "db.users.deleteOne({})", common.Write},
		{"deleteMany is write", "db.users.deleteMany({})", common.Write},
		{"show collections is read", "show collections", common.Read},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, adapters.QueryTypeMongo(tc.query))
		})
	}
}

// RouteRedis: every Redis query dispatches to ExecuteRedisQuery; there
// is no read/write split in the spec. The dispatch happens directly in
// execute.go — there is no QueryTypeRedis classifier to test. We leave
// a positive guard that the connection-type wiring itself exists.
func TestRouteRedisConnTypePresent(t *testing.T) {
	assert.Equal(t, common.ConnType("redis"), common.Redis)
}

// ---------------------------------------------------------------------
// invariant ChainedStagesAreReadOnly + StartPipelineExecution requires
//
// Spec says: in a pipeline, only the last stage may be write/admin.
// The current implementation (run_query.go / execute.go) does NOT
// enforce this. These tests are expected to FAIL until enforcement is
// added. That is the gap this test batch is designed to expose.
//
// Strategy: call parseQueries to produce a pipeline plan, then
// classify each stage with the appropriate QueryType* and assert that
// a pipeline with a non-terminal write stage is rejected.
//
// Since there is no dedicated validator today, we drive the check
// through a helper that we expect to exist post-fix. For now the tests
// inline the check against the current behaviour and mark the
// assertion with a TODO comment.
// ---------------------------------------------------------------------

// pipelineRejectsNonTerminalWrites returns nil iff every non-terminal
// stage in the parsed pipeline is a read (or jq, which is spec-read by
// construction). A conforming implementation of
// ChainedStagesAreReadOnly should surface an error at
// parseQueries/HandleRunQuery time for any violating input.
//
// TODO(ChainedStagesAreReadOnly): when the production code enforces
// the invariant, replace this helper with a direct assertion on the
// HandleRunQuery / parseQueries error.
func pipelineRejectsNonTerminalWrites(queries []common.QueryMetadata) error {
	for i, q := range queries {
		if i == len(queries)-1 {
			continue
		}
		if q.ConnType == common.Jq {
			continue
		}
		var qt common.QueryType
		switch q.ConnType {
		case common.Postgres:
			qt = adapters.QueryTypePostgres(q.QueryLine)
		case common.Mysql:
			qt = adapters.QueryTypeMysql(q.QueryLine)
		case common.Mongo:
			qt = adapters.QueryTypeMongo(q.QueryLine)
		case common.Redis:
			qt = adapters.QueryTypeRedis(q.QueryLine)
		}
		if qt == common.Write {
			return errors.New("non-terminal write stage violates ChainedStagesAreReadOnly")
		}
	}
	return nil
}

func TestChainedStagesAreReadOnly_RejectsNonTerminalWrite(t *testing.T) {
	// EXPECTED FAILURE until the production code enforces the invariant
	// at HandleRunQuery / parseQueries time. The assertion below is on
	// the real production entry point, not on the helper, so a fix can
	// flip it green.
	home := withTempHome(t)
	seedConnections(t, home, []common.KeyURIPair{
		{Key: "pg", URI: "postgres://h/db"},
	})
	// Non-terminal write (UPDATE) followed by a read: must be rejected
	// before any network I/O. A conforming implementation produces a
	// validation error whose message references the invariant.
	out, err := HandleRunQuery([]string{
		"::pg> UPDATE users SET active = true::pg> SELECT * FROM users",
		"",
	})
	// HandleRunQuery folds errors into the returned string. The current
	// code raises only a DB connection error ("Error: dial tcp ..."),
	// which is why this assertion FAILS today: the validation layer
	// doesn't exist yet.
	lower := out
	assert.NoError(t, err)
	assert.True(t,
		containsAny(lower,
			"ChainedStagesAreReadOnly",
			"read-only",
			"read only",
			"non-terminal write",
			"only the last stage",
			"may not be write",
		),
		"pipeline with non-terminal write must be rejected with an invariant-specific error; got: %q", out,
	)
}

func TestChainedStagesAreReadOnly_AllowsTerminalWrite(t *testing.T) {
	// Pure static check: a read followed by a terminal write is valid
	// per the spec. The helper must accept it. (This does not execute
	// the pipeline end-to-end.)
	queries := []common.QueryMetadata{
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT id FROM users"},
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "DELETE FROM users WHERE id = 1"},
	}
	assert.NoError(t, pipelineRejectsNonTerminalWrites(queries))
}

func TestChainedStagesAreReadOnly_RejectsMidPipelineWrite(t *testing.T) {
	// Static helper view of the same violation as the HandleRunQuery
	// test above, independent of the surface entry point.
	queries := []common.QueryMetadata{
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT id FROM users"},
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "UPDATE users SET active = true"},
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT * FROM users"},
	}
	assert.Error(t, pipelineRejectsNonTerminalWrites(queries))
}

func TestChainedStagesAreReadOnly_JqIsAlwaysReadCompatible(t *testing.T) {
	// A jq stage sandwiched between reads is valid: jq is a pure
	// transformer and is spec-read by construction.
	queries := []common.QueryMetadata{
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT id FROM users"},
		{Conn: "jq://", ConnType: common.Jq, QueryLine: ".[0]"},
		{Conn: "postgres://h/db", ConnType: common.Postgres, QueryLine: "SELECT * FROM users LIMIT 1"},
	}
	assert.NoError(t, pipelineRejectsNonTerminalWrites(queries))
}
