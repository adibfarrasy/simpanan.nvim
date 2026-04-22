package adapters

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"simpanan/internal/common"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// mysqlDSN converts a user-facing `mysql://user:pass@host:port/db?params`
// URI into the DSN format the go-sql-driver/mysql expects:
//
//	user:pass@tcp(host:port)/dbname?params
//
// The URL form is what simpanan uses everywhere else; the driver's native
// DSN form is exposed nowhere else in the codebase.
func mysqlDSN(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("invalid mysql uri: %s", err)
	}
	if u.Scheme != "mysql" {
		return "", fmt.Errorf("mysql uri must use mysql:// scheme, got %q", u.Scheme)
	}

	host := u.Host
	if host == "" {
		return "", fmt.Errorf("mysql uri is missing host")
	}

	userinfo := ""
	if u.User != nil {
		if pw, ok := u.User.Password(); ok {
			userinfo = fmt.Sprintf("%s:%s@", u.User.Username(), pw)
		} else {
			userinfo = fmt.Sprintf("%s@", u.User.Username())
		}
	}

	dbName := strings.TrimPrefix(u.Path, "/")

	dsn := fmt.Sprintf("%stcp(%s)/%s", userinfo, host, dbName)
	if u.RawQuery != "" {
		dsn = dsn + "?" + u.RawQuery
	}
	return dsn, nil
}

func QueryTypeMysql(query string) common.QueryType {
	fields := strings.Fields(query)
	if len(fields) == 0 {
		return common.Read
	}
	switch strings.ToLower(fields[0]) {
	case "insert", "update", "delete", "replace",
		"create", "drop", "alter", "truncate", "rename",
		"grant", "revoke":
		return common.Write
	}
	return common.Read
}

func ExecuteMysqlReadQuery(q common.QueryMetadata) ([]byte, error) {
	dsn, err := mysqlDSN(q.Conn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(q.QueryLine)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err.Error(), q.QueryLine)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]sql.RawBytes, len(columns))
	dest := make([]any, len(values))
	for i := range values {
		dest[i] = &values[i]
	}

	rowCount := 0
	var results [][]byte
	for rows.Next() {
		if rowCount == common.GetConfig().MaxRowLimit {
			break
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		rowResults := common.RowData{}
		for i, col := range values {
			if col == nil {
				rowResults = append(rowResults, common.ColumnValuePair{Key: columns[i], Value: nil})
			} else {
				rowResults = append(rowResults, common.ColumnValuePair{Key: columns[i], Value: convertToType(col)})
			}
		}

		resBytes, err := rowResults.MarshallJSON()
		if err != nil {
			return nil, err
		}
		results = append(results, resBytes)
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	jsonArrB := []byte{'['}
	for i, r := range results {
		jsonArrB = append(jsonArrB, r...)
		if i != len(results)-1 {
			jsonArrB = append(jsonArrB, ',')
		}
	}
	jsonArrB = append(jsonArrB, ']')
	return jsonArrB, nil
}

func ExecuteMysqlWriteQuery(q common.QueryMetadata) ([]byte, error) {
	dsn, err := mysqlDSN(q.Conn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	res, err := db.Exec(q.QueryLine)
	if err != nil {
		return nil, err
	}

	lastInsertID, _ := res.LastInsertId()
	rowsAffected, _ := res.RowsAffected()
	return json.Marshal(struct {
		LastInsertID int64 `json:"last_insert_id"`
		RowsAffected int64 `json:"rows_affected"`
	}{lastInsertID, rowsAffected})
}
