package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name           string
		arg            string
		connMap        map[string]string
		expectedResult QueryMetadata
		expectedError  error
	}{
		{
			name:           "connection key not found - bad arg",
			arg:            "abc",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{},
			expectedError:  errors.New("Connection key '' not found."),
		},
		{
			name:           "connection key not found - empty arg",
			arg:            "|>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{},
			expectedError:  errors.New("Connection key '' not found."),
		},
		{
			name:           "connection key not found - not found in connMap",
			arg:            "|connX>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{},
			expectedError:  errors.New("Connection key 'connX' not found."),
		},
		{
			name:           "no query",
			arg:            "|conn1>",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:           "whitespace query",
			arg:            "|conn1>         ",
			connMap:        map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{},
			expectedError:  errors.New("No query on the right hand side of connection."),
		},
		{
			name:    "parsed",
			arg:     "|conn1> select * from table",
			connMap: map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: QueryMetadata{
				Conn:     "postgres://root:root@localhost:port/db_name",
				ConnType: Postgres,
				ExecLine: "select * from table",
				ExecType: Query,
			},
			expectedError: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := parseQuery(test.arg, test.connMap, true)
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
		expectedResult []QueryMetadata
		expectedError  error
	}{
		{
			name:    "parsed multiline query",
			args:    []string{"conn1> select * from query", "continued"},
			connMap: map[string]string{"conn1": "postgres://root:root@localhost:port/db_name"},
			expectedResult: []QueryMetadata{
				{
					Conn:     "postgres://root:root@localhost:port/db_name",
					ConnType: Postgres,
					ExecLine: "select * from query continued",
					ExecType: Query,
				},
			},
			expectedError: nil,
		},
		{
			name: "parsed multiline queries",
			args: []string{"conn1> select * from query", "continued", "|conn2> select * from query2", "continued2"},
			connMap: map[string]string{
				"conn1": "postgres://root:root@localhost:port/db_name",
				"conn2": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []QueryMetadata{
				{
					Conn:     "postgres://root:root@localhost:port/db_name",
					ConnType: Postgres,
					ExecLine: "select * from query continued",
					ExecType: Query,
				},
				{
					Conn:     "postgres://root:root@localhost:port/db_name",
					ConnType: Postgres,
					ExecLine: "select * from query2 continued2",
					ExecType: Query,
				},
			},
			expectedError: nil,
		},
		{
			name: "parsed single line",
			args: []string{" pg0> select name from test_table;"},
			connMap: map[string]string{
				"pg0": "postgres://root:root@localhost:port/db_name",
			},
			expectedResult: []QueryMetadata{
				{
					Conn:     "postgres://root:root@localhost:port/db_name",
					ConnType: Postgres,
					ExecLine: "select name from test_table;",
					ExecType: Query,
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
