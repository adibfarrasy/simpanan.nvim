package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"simpanan/internal/common"
)

func HandleGetConnections(args []string) ([]string, error) {
	res, err := GetConnectionList()
	if err != nil {
		return nil, err
	}
	data := []string{}
	for _, r := range res {
		data = append(data, r.String())
	}

	return data, nil
}

func GetConnectionList() ([]common.KeyURIPair, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(homeDir, ".local/share/nvim/simpanan_connections.json")
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var keyUriPairs []common.KeyURIPair
	err = json.Unmarshal(fileContent, &keyUriPairs)

	return keyUriPairs, err
}
