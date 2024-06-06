package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"simpanan/internal/common"
	"strings"
)

type debugObj struct {
	common.QueryMetadata
	Result *any `json:"result"`
}

func HandleRunQuery(args []string) (string, error) {
	// remove the :: prefix and :: separators
	argsToRun := strings.Split(args[0], "::")[1:]
	opts := strings.Split(args[1], "::")[1:]
	common.SetConfig(opts)

	conns, err := GetConnectionList()
	if err != nil {
		return processError(err)
	}

	connMap := common.KeyURIPairs(conns).Map()

	// add special faux connection
	connMap["jq"] = "jq://"

	queries, err := parseQueries(argsToRun, connMap)
	if err != nil {
		return processError(err)
	}

	dbgRes := []debugObj{}

	tmpRes := []byte{}
	for i, q := range queries {
		if i > 0 && len(tmpRes) == 0 {
			return processError(fmt.Errorf("No arguments passed to one of the pipelines."))
		}

		res, err := execute(q, tmpRes)
		if err != nil {
			return processError(err)
		}

		if common.GetConfig().DebugMode {
			var tmpRes any
			err := json.Unmarshal(res, &tmpRes)
			if err != nil {
				return processError(err)
			}
			dbgRes = append(dbgRes, debugObj{q, &tmpRes})
		}

		tmpRes = res
	}

	if common.GetConfig().DebugMode {
		return processPayloadDebug(dbgRes)
	}
	return processPayload(tmpRes)
}

func parseQueries(args []string, connMap map[string]string) ([]common.QueryMetadata, error) {
	queries := []common.QueryMetadata{}

	tmpQueryMeta := common.QueryMetadata{}
	firstQuery := true
	args = sanitizeArgs(args)
	for i, a := range args {
		if firstQuery {
			q, err := parseQuery(a, connMap)
			if err != nil {
				return nil, err
			}
			tmpQueryMeta = q

			if len(args) == 1 {
				queries = append(queries, tmpQueryMeta)
			}

			firstQuery = !firstQuery
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
	match := regexp.MustCompile(`^(\S+?)>`).FindStringSubmatch(a)
	conn := ""
	if len(match) > 0 {
		conn = strings.TrimSpace(match[1])
	}
	v, ok := connMap[conn]
	if !ok {
		return common.QueryMetadata{}, fmt.Errorf("Connection key '%s' not found.", conn)
	}

	split := strings.Split(a, fmt.Sprintf("%s>", conn))

	query := strings.TrimSpace(split[1])
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
	return len(regexp.MustCompile(`^\S+?>`).FindString(a)) > 0
}

func sanitizeArgs(args []string) (res []string) {
	for _, a := range args {
		a = strings.TrimSpace(a)
		if len(a) == 0 {
			continue
		}
		if _, found := strings.CutPrefix(a, "//"); found {
			continue
		}

		res = append(res, a)
	}
	return
}

func processPayload(res []byte) (string, error) {
	if len(res) == 0 {
		return "", nil
	}

	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(prettyJSON.Bytes()), nil
}

func processError(err error) (string, error) {
	return fmt.Sprintf("Error: %s", err.Error()), nil
}

func processPayloadDebug(dbgRes []debugObj) (string, error) {
	var prettyJSON bytes.Buffer
	prettyJSON.Write([]byte("// DEBUG MODE\n"))

	if len(dbgRes) == 0 {
		return "", nil
	}

	res, err := json.Marshal(dbgRes)
	if err != nil {
		return "", err
	}

	err = json.Indent(&prettyJSON, res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(prettyJSON.Bytes()), nil
}
