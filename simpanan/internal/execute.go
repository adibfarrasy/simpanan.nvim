package internal

import (
	"database/sql"
	"errors"

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
				rowResults = append(rowResults, columnValuePair{key: columns[i], value: string(col)})
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
