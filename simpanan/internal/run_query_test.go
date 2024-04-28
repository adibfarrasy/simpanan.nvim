package internal

import (
	"errors"
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name           string
		arg            string
		connMap        map[string]string
		expectedResult common.QueryMetadata
		expectedError  error
	}{
		{
			name:           "connection key not found - bad arg",
			arg:            "abc",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("Connection key '' not found."),
		},
		{
			name:           "connection key not found - empty arg",
			arg:            ">",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("Connection key '' not found."),
		},
		{
			name:           "connection key not found - not found in connMap",
			arg:            "connX>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("Connection key 'connX' not found."),
		},
		{
			name:           "no query",
			arg:            "conn1>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:           "whitespace query",
			arg:            "conn1>         ",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:    "parsed",
			arg:     "conn1> select * from table",
			connMap: map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: common.QueryMetadata{
				Conn:      "postgres://root:root@localhost:port/db_name",
				ConnType:  common.Postgres,
				QueryLine: "select * from table",
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
			args:    []string{"conn1> select * from query", "continued"},
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
			args: []string{"conn1> select * from query", "continued", "conn2> select * from query2", "continued2"},
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
			name: "bug case: parsed single line",
			args: []string{" pg0> select name from test_table;"},
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
			name: "bug case: should return second result",
			args: []string{"rew-dev> select * from reward_balances order by created_at desc limit 3",
				"rew-dev> select * from rewards where id = '{{.[0].reward_id}}';"},
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
