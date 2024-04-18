package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

func execute(q QueryMetadata, previousResults []byte) ([]byte, error) {
	switch q.ConnType {
	case Postgres:
		if q.ExecType == Query {
			return executePostgresQuery(q, previousResults)
		} else {
			// TODO: implement this
			return nil, errors.New("Not implemented.")
		}
	case Mongo:
		// TODO: implement this
		return nil, errors.New("Not implemented.")
	case Redis:
		// TODO: implement this
		return nil, errors.New("Not implemented.")
	default:
		return nil, errors.New("Unknown connection type.")
	}
}

func executePostgresQuery(q QueryMetadata, previousResults []byte) ([]byte, error) {
	db, err := sql.Open("postgres", q.Conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows
	if len(previousResults) == 0 {
		rows, err = db.Query(q.ExecLine)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: handle pipeline and query with the args
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

		rowResults := rowData{}
		for i, col := range values {
			if col == nil {
				rowResults = append(rowResults, columnValuePair{key: columns[i], value: "NULL"})
			} else {
				rowResults = append(rowResults, columnValuePair{key: columns[i], value: convertToType(col)})
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
