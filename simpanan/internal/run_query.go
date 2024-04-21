package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"simpanan/internal/common"
	"strings"
)

func HandleRunQuery(args []string) (string, error) {
	// remove the :: prefix and :: separators
	args = strings.Split(args[0], "::")[1:]

	conns, err := GetConnectionList()
	if err != nil {
		return "", err
	}

	connMap := common.KeyURIPairs(conns).Map()

	queries, err := parseQueries(args, connMap)
	if err != nil {
		return "", err
	}

	tmpRes := []byte{}
	for i, q := range queries {
		if i > 0 && len(tmpRes) == 0 {
			return "", fmt.Errorf("No arguments passed to one of the pipelines.")
		}

		res, err := execute(q, tmpRes)
		if err != nil {
			return "", err
		}

		tmpRes = res
	}

	return processPayload(tmpRes)
}

func parseQueries(args []string, connMap map[string]string) ([]common.QueryMetadata, error) {
	queries := []common.QueryMetadata{}

	tmpQueryMeta := common.QueryMetadata{}
	for i, a := range args {
		if i == 0 {
			q, err := parseQuery(a, connMap)
			if err != nil {
				return nil, err
			}
			tmpQueryMeta = q

			if len(args) == 1 {
				queries = append(queries, tmpQueryMeta)
			}
			continue
		}

		if len(strings.TrimLeft(a, " ")) == 0 {
			continue
		}

		if !hasConnArg(a) {
			tmpQueryMeta.QueryLine += fmt.Sprintf(" %s", a)
		} else {
			queries = append(queries, tmpQueryMeta)

			q, err := parseQuery(a, connMap)
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

func parseQuery(a string, connMap map[string]string) (common.QueryMetadata, error) {
	match := regexp.MustCompile(`^(.*?)>`).FindStringSubmatch(a)
	conn := ""
	if len(match) > 0 {
		conn = strings.TrimLeft(match[1], " ")
	}
	v, ok := connMap[conn]
	if !ok {
		return common.QueryMetadata{}, fmt.Errorf("Connection key '%s' not found.", conn)
	}

	split := strings.Split(a, fmt.Sprintf("%s>", conn))

	query := strings.TrimLeft(split[1], " ")
	if len(split) < 2 || len(query) == 0 {
		return common.QueryMetadata{}, fmt.Errorf("No query on the right hand side of connection.")
	}

	connType, err := common.URI(v).ConnType()
	if err != nil {
		return common.QueryMetadata{}, err
	}
	return common.QueryMetadata{
		Conn:      v,
		ConnType:  *connType,
		QueryLine: query,
	}, nil
}

func hasConnArg(a string) bool {
	return len(regexp.MustCompile(`^.*?>`).FindString(a)) > 0
}

func processPayload(res []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(prettyJSON.Bytes()), nil
}
