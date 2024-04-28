package common

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func ProcessPayload(res []byte) (string, error) {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(prettyJSON.Bytes()), nil
}

func ProcessError(err error) (string, error) {
	return fmt.Sprintf("Error: %s", err.Error()), nil
}
