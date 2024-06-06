package adapters

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"simpanan/internal/common"
	"strconv"
	"strings"
)

func ExecutePostgresReadQuery(q common.QueryMetadata) ([]byte, error) {
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

func ExecutePostgresAdminCmd(q common.QueryMetadata) ([]byte, error) {
	switch q.QueryLine {
	case "\\dt":
		q.QueryLine = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'"
		return ExecutePostgresReadQuery(q)
	default:
		matches := regexp.MustCompile(`\\d (.*)`).FindAllStringSubmatch(q.QueryLine, -1)
		if len(matches) != 1 {
			return nil, fmt.Errorf("ExecutePostgresAdminCmd: invalid query format %s.", q.QueryLine)
		}

		tableName := matches[0][1]

		q.QueryLine = fmt.Sprintf("SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name = '%s'", tableName)
		colDef, err := ExecutePostgresReadQuery(q)
		if err != nil {
			return nil, err
		}
		var colDefMap []map[string]any
		if err := json.Unmarshal(colDef, &colDefMap); err != nil {
			return nil, err
		}

		q.QueryLine = fmt.Sprintf(`SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = '%s';`, tableName)
		indices, err := ExecutePostgresReadQuery(q)
		if err != nil {
			return nil, err
		}
		var indicesMap []map[string]any
		if err := json.Unmarshal(indices, &indicesMap); err != nil {
			return nil, err
		}

		q.QueryLine = fmt.Sprintf(`SELECT tc.constraint_name, tc.constraint_type, ccu.column_name
FROM information_schema.table_constraints tc 
JOIN information_schema.constraint_column_usage ccu 
ON tc.constraint_name = ccu.constraint_name 
WHERE tc.table_name = '%s';`, tableName)
		constraints, err := ExecutePostgresReadQuery(q)
		if err != nil {
			return nil, err
		}
		var constraintMap []map[string]any
		if err := json.Unmarshal(constraints, &constraintMap); err != nil {
			return nil, err
		}

		result := struct {
			ColumnDefinitions []map[string]any `json:"column_definitions"`
			Indices           []map[string]any `json:"indices"`
			Constraints       []map[string]any `json:"constraints"`
		}{
			ColumnDefinitions: colDefMap,
			Indices:           indicesMap,
			Constraints:       constraintMap,
		}

		return json.Marshal(result)
	}
}

func ExecutePostgresWriteQuery(q common.QueryMetadata) ([]byte, error) {
	db, err := sql.Open("postgres", q.Conn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	res, err := db.Exec(q.QueryLine)
	if err != nil {
		return nil, err
	}

	return json.Marshal(res)
}
