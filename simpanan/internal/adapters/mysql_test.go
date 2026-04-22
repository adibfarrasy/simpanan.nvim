package adapters

import (
	"simpanan/internal/common"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMysqlDSN(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			"user and password",
			"mysql://root:secret@127.0.0.1:3306/mydb",
			"root:secret@tcp(127.0.0.1:3306)/mydb",
			false,
		},
		{
			"user only",
			"mysql://root@host:3306/mydb",
			"root@tcp(host:3306)/mydb",
			false,
		},
		{
			"no credentials",
			"mysql://host:3306/mydb",
			"tcp(host:3306)/mydb",
			false,
		},
		{
			"query params passed through",
			"mysql://root:secret@host:3306/mydb?parseTime=true&charset=utf8",
			"root:secret@tcp(host:3306)/mydb?parseTime=true&charset=utf8",
			false,
		},
		{
			"no database name is allowed (driver treats it as 'no default db')",
			"mysql://root@host:3306/",
			"root@tcp(host:3306)/",
			false,
		},
		{
			"wrong scheme is rejected",
			"postgres://h/db",
			"",
			true,
		},
		{
			"missing host is rejected",
			"mysql:///db",
			"",
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mysqlDSN(tc.uri)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestQueryTypeMysql(t *testing.T) {
	tests := []struct {
		query string
		want  common.QueryType
	}{
		// Reads — including metadata-style queries, which are ordinary
		// SQL on MySQL and classified as read (no separate admin class).
		{"SELECT * FROM t", common.Read},
		{"show tables", common.Read},
		{"SHOW DATABASES", common.Read},
		{"DESCRIBE t", common.Read},
		{"EXPLAIN SELECT * FROM t", common.Read},
		// Writes — DML
		{"INSERT INTO t VALUES (1)", common.Write},
		{"update t set x=1", common.Write},
		{"DELETE FROM t", common.Write},
		{"REPLACE INTO t VALUES (1)", common.Write},
		// Writes — DDL
		{"CREATE TABLE t (id int)", common.Write},
		{"DROP TABLE t", common.Write},
		{"ALTER TABLE t ADD c int", common.Write},
		{"TRUNCATE t", common.Write},
		{"RENAME TABLE t TO u", common.Write},
		// Writes — permissions
		{"grant select on t to u", common.Write},
		{"REVOKE all on t from u", common.Write},
		// Edge
		{"", common.Read},
		{"   ", common.Read},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			assert.Equal(t, tc.want, QueryTypeMysql(tc.query))
		})
	}
}
