package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"simpanan/internal/common"
)

func HandleDeleteConnection(args []string) (string, error) {
	if len(args) == 0 || len(args[0]) == 0 {
		return "", fmt.Errorf("Empty connection label.")
	}
	toBeDeletedConn := args[0]

	conns, err := GetConnectionList()
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	newPairs := []common.KeyURIPair{}
	found := false
	for _, conn := range conns {
		if conn.Key == toBeDeletedConn {
			found = true
			continue
		}
		newPairs = append(newPairs, conn)
	}

	if !found {
		return "", fmt.Errorf("Connection with name '%s' does not exist.", toBeDeletedConn)
	}

	filePath := filepath.Join(homeDir, ".local/share/nvim/simpanan_connections.json")
	data, err := json.Marshal(newPairs)
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", err
	}

	return "Success", nil
}
