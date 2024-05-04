package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"simpanan/internal/common"
	"strings"
)

func HandleAddConnection(args []string) (string, error) {
	newConn := args[0]
	if newConn == "jq" {
		return "", fmt.Errorf("New connection name cannot be 'jq'.")
	}

	if newConn[len(newConn)-1] == '>' {
		return "", fmt.Errorf("Cannot use suffix character '>'.")
	}

	conns, err := GetConnectionList()
	if err != nil {
		return "", err
	}

	c := strings.Split(newConn, ">")
	if len(c) < 2 {
		return "", fmt.Errorf("Invalid connection syntax '%s'", newConn)
	}

	for _, conn := range conns {
		if c[0] == conn.Key {
			return "", fmt.Errorf("Connection with name '%s' already exists.", c[0])
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	conns = append(conns, common.KeyURIPair{
		Key: c[0],
		URI: common.URI(c[1]),
	})

	filePath := filepath.Join(homeDir, ".local/share/nvim/simpanan_connections.json")
	data, err := json.Marshal(conns)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", err
	}

	return "Success", nil
}
