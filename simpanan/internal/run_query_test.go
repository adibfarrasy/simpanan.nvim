package internal

import (
	"errors"
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Stage syntax is `|<label>>...`. Tests below cover the new
// leading-pipe header. Args without a leading `|` are no longer
// considered stage headers — they're either continuation lines
// (when handled by parseQueries) or rejected (when handed directly
// to parseQuery).

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name           string
		arg            string
		connMap        map[string]string
		expectedResult common.QueryMetadata
		expectedError  error
	}{
		{
			name:           "missing leading pipe — bad arg",
			arg:            "abc",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New(`Stage missing leading '|<label>>' header in: "abc"`),
		},
		{
			name:           "missing leading pipe — only > present",
			arg:            ">",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New(`Stage missing leading '|<label>>' header in: ">"`),
		},
		{
			name:           "label not in connMap",
			arg:            "|connX>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("Connection key 'connX' not found."),
		},
		{
			name:           "no query",
			arg:            "|conn1>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:           "whitespace query",
			arg:            "|conn1>         ",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:    "parsed",
			arg:     "|conn1> select * from table",
			connMap: map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{
				Conn:      "postgres://root:root@localhost:port/db_name",
				ConnType:  common.Postgres,
				QueryLine: "select * from table",
			},
			expectedError: nil,
		},
		{
			name:    "parsed jq query",
			arg:     "|jq> .",
			connMap: map[string]string{"jq": "jq://"},
			expectedResult: common.QueryMetadata{
				Conn:      "jq://",
				ConnType:  common.Jq,
				QueryLine: ".",
			},
			expectedError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := parseQuery(test.arg, test.connMap)
			assert.Equal(t, test.expectedResult, res)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

// Regression: the body of a stage may legitimately contain text that
// looks like an old-style header (e.g. a SQL string literal containing
// '>'). The new |-prefixed grammar makes this unambiguous: the leading
// '|' marks the header, and the rest of the line is body until end of
// line (or until the next |-prefixed line in parseQueries).
func TestParseQueryPreservesBodyContainingAngleBracket(t *testing.T) {
	connMap := map[string]string{"pg0": "postgres://u:p@h/db"}
	res, err := parseQuery("|pg0> select 'pg0>' as literal", connMap)
	assert.NoError(t, err)
	assert.Equal(t, "select 'pg0>' as literal", res.QueryLine)
}

func TestParseQueries(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		connMap        map[string]string
		expectedResult []common.QueryMetadata
		expectedError  error
	}{
		{
			name:    "parsed multiline query",
			args:    []string{"|conn1> select * from query", "continued"},
			connMap: map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: []common.QueryMetadata{
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from query continued",
				},
			},
			expectedError: nil,
		},
		{
			name: "parsed multiline queries",
			args: []string{"|conn1> select * from query", "continued", "|conn2> select * from query2", "continued2"},
			connMap: map[string]string{
				"conn1": "postgres://root:root@localhost:port/db_name",
				"conn2": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []common.QueryMetadata{
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from query continued",
				},
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from query2 continued2",
				},
			},
			expectedError: nil,
		},
		{
			name: "parsed single line",
			args: []string{" |pg0> select name from test_table;"},
			connMap: map[string]string{
				"pg0": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []common.QueryMetadata{
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select name from test_table;",
				},
			},
			expectedError: nil,
		},
		{
			name: "two stages",
			args: []string{"|rew-dev> select * from reward_balances order by created_at desc limit 3",
				"|rew-dev> select * from rewards where id = '{{.[0].reward_id}}';"},
			connMap: map[string]string{
				"rew-dev": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []common.QueryMetadata{
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from reward_balances order by created_at desc limit 3",
				},
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from rewards where id = '{{.[0].reward_id}}';",
				},
			},
			expectedError: nil,
		},
		{
			name: "handle comment line",
			args: []string{"// some comment", "|rew-dev> select * from reward_balances order by created_at desc limit 3",
				"|rew-dev> select * from rewards where id = '{{.[0].reward_id}}';"},
			connMap: map[string]string{
				"rew-dev": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []common.QueryMetadata{
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from reward_balances order by created_at desc limit 3",
				},
				{
					Conn:      "postgres://root:root@localhost:port/db_name",
					ConnType:  common.Postgres,
					QueryLine: "select * from rewards where id = '{{.[0].reward_id}}';",
				},
			},
			expectedError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := parseQueries(test.args, test.connMap)
			assert.Equal(t, test.expectedResult, res)
			assert.Equal(t, test.expectedError, err)
		})
	}
}
