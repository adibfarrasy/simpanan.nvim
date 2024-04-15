package internal

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	_ "github.com/lib/pq"
)

func execute(q QueryMetadata, previousResults []RowData) ([]RowData, error) {
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

func executePostgresQuery(q QueryMetadata, previousResults []RowData) ([]RowData, error) {
	db, err := sql.Open("postgres", q.Conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	argCount, err := checkArgsValidity(q, previousResults)
	if err != nil {
		return nil, err
	}

	var rows *sql.Rows
	if argCount == 0 {
		rows, err = db.Query(q.ExecLine)
		if err != nil {
			return nil, err
		}
	} else {
		// since it's not possible to accommodate all the previousResults,
		// only the first index is used in the query.
		rows, err = db.Query(q.ExecLine, previousResults[0])
		if err != nil {
			return nil, err
		}
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

	var results []RowData
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		rowResults := RowData{}
		for i, col := range values {
			if col == nil {
				rowResults = append(rowResults, ColumnValuePair([]string{columns[i], "NULL"}))
			} else {
				rowResults = append(rowResults, ColumnValuePair([]string{columns[i], string(col)}))
			}
		}

		results = append(results, rowResults)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func checkArgsValidity(q QueryMetadata, previousResults []RowData) (int, error) {
	match := regexp.MustCompile(`$>`).FindAllSubmatch([]byte(q.ExecLine), -1)

	if len(previousResults) > 0 && len(match) != len(previousResults[0]) {
		return 0, fmt.Errorf("Number of given and required arguments mismatched. Given %d, required %d.", len(previousResults[0]), len(match))
	}

	return len(match), nil
}
