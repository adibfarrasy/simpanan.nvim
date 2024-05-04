package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"simpanan/internal/common"
)

func HandleDeleteConnection(args []string) (string, error) {
	toBeDeletedConn := args[0]

	conns, err := GetConnectionList()
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var newPairs []common.KeyURIPair
	for _, conn := range conns {
		// repopulate JSON
		if conn.Key != toBeDeletedConn {
			newPairs = append(newPairs, conn)
		}
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
