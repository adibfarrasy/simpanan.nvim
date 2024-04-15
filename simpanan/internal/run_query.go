package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func HandleRunQuery(args []string) (string, error) {
	args = []string{strings.ReplaceAll(args[0], "::", " ")}

	conns, err := GetConnectionList()
	if err != nil {
		return "", err
	}

	connMap := KeyURIPairs(conns).Map()

	queries, err := parseQueries(args, connMap)
	if err != nil {
		return "", err
	}

	tmpRes := []RowData{}
	for i, q := range queries {
		if i > 0 && len(tmpRes) == 0 {
			return "", errors.New("No arguments passed to one of the pipelines.")
		}

		res, err := execute(q, tmpRes)
		if err != nil {
			return "", err
		}

		tmpRes = res
	}

	payload := []map[string]string{}
	for _, tr := range tmpRes {
		payload = append(payload, tr.Map())
	}

	res, err := json.MarshalIndent(payload, "", "\t")

	return string(res), err
}

func parseQueries(args []string, connMap map[string]string) ([]QueryMetadata, error) {
	queries := []QueryMetadata{}

	tmpQueryMeta := QueryMetadata{}
	for i, a := range args {
		if len(strings.TrimLeft(a, " ")) == 0 {
			continue
		}

		if i == 0 {
			q, err := parseQuery(a, connMap, false)
			if err != nil {
				return nil, err
			}
			tmpQueryMeta = q

			if len(args) == 1 {
				queries = append(queries, tmpQueryMeta)
			}
			continue
		}

		if !hasConnArg(a) {
			tmpQueryMeta.ExecLine += fmt.Sprintf(" %s", a)
		} else {
			queries = append(queries, tmpQueryMeta)

			q, err := parseQuery(a, connMap, true)
			if err != nil {
				return nil, err
			}
			tmpQueryMeta = q
		}

		if i == len(args)-1 {
			queries = append(queries, tmpQueryMeta)
		}
	}
	return queries, nil
}

// TODO: figure out the pipelining between multiple queries
func parseQuery(a string, connMap map[string]string, piped bool) (QueryMetadata, error) {
	var match []string
	if piped {
		match = regexp.MustCompile(`\|(.*)>`).FindStringSubmatch(a)
	} else {
		match = regexp.MustCompile(`(.*)>`).FindStringSubmatch(a)
	}
	conn := ""
	if len(match) > 0 {
		conn = strings.TrimLeft(match[1], " ")
	}
	v, ok := connMap[conn]
	if !ok {
		return QueryMetadata{}, fmt.Errorf("Connection key '%s' not found.", conn)
	}

	var split []string
	if piped {
		split = strings.Split(a, fmt.Sprintf("|%s>", conn))
	} else {
		split = strings.Split(a, fmt.Sprintf("%s>", conn))
	}

	query := strings.TrimLeft(split[1], " ")
	if len(split) < 2 || len(query) == 0 {
		return QueryMetadata{}, errors.New("No query on the right hand side of connection.")
	}

	var execType ExecType
	if isQuery(query) {
		execType = Query
	} else {
		execType = Command
	}

	connType, err := URI(v).ConnType()
	if err != nil {
		return QueryMetadata{}, err
	}
	return QueryMetadata{
		Conn:     v,
		ConnType: *connType,
		ExecLine: query,
		ExecType: execType,
	}, nil
}

func hasConnArg(a string) bool {
	return len(regexp.MustCompile(`\|.*>`).FindString(a)) > 0
}

func isQuery(s string) bool {
	for _, q := range QUERY_PREFIXES {
		if strings.HasPrefix(s, q) {
			return true
		}
	}
	return false
}
