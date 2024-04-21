package adapters

import (
	"database/sql"
	"fmt"
	"simpanan/internal/common"
	"strconv"
	"strings"
)

func ExecutePostgresQuery(q common.QueryMetadata) ([]byte, error) {
	db, err := sql.Open("postgres", q.Conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows
	rows, err = db.Query(q.QueryLine)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err.Error(), q.QueryLine)
	}

	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Make a slice to hold the values
	values := make([]sql.RawBytes, len(columns))
	dest := make([]any, len(values))
	for i := range values {
		dest[i] = &values[i]
	}

	var results [][]byte
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		rowResults := common.RowData{}
		for i, col := range values {
			if col == nil {
				rowResults = append(rowResults, common.ColumnValuePair{Key: columns[i], Value: "NULL"})
			} else {
				rowResults = append(rowResults, common.ColumnValuePair{Key: columns[i], Value: convertToType(col)})
			}
		}

		resBytes, err := rowResults.MarshallJSON()
		if err != nil {
			return nil, err
		}

		results = append(results, resBytes)
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

func convertToType(col sql.RawBytes) any {
	colB := []byte(col)
	floatValue, err := bytesToFloat64(colB)
	if err == nil {
		return floatValue
	}
	intValue, err := bytesToInt64(colB)
	if err == nil {
		return intValue
	}
	boolValue, err := bytesToBool(colB)
	if err == nil {
		return boolValue
	}
	return string(colB)
}

func bytesToInt64(b []byte) (int64, error) {
	return strconv.ParseInt(string(b), 10, 64)
}

func bytesToFloat64(b []byte) (float64, error) {
	return strconv.ParseFloat(string(b), 64)
}

func bytesToBool(b []byte) (bool, error) {
	s := strings.ToLower(string(b))
	switch s {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

func QueryTypePostgres(query string) common.QueryType {
	action := strings.ToLower(strings.Split(query, " ")[0])
	if action == "update" || action == "delete" || action == "insert" {
		return common.Write
	}

	return common.Read
}
