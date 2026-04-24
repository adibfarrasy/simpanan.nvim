package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"simpanan/internal/common"
)

// HandleDeleteConnection is the rplugin-shaped entry: args[0] is the
// label. Thin shim over DeleteConnection.
func HandleDeleteConnection(args []string) (string, error) {
	if len(args) == 0 || len(args[0]) == 0 {
		return "", fmt.Errorf("Empty connection label.")
	}
	if err := DeleteConnection(args[0]); err != nil {
		return "", err
	}
	return "Success", nil
}

// DeleteConnection removes a registered label and drops its schema
// cache file (idempotent). Used by both the rplugin shim and the
// webui HTTP handler.
func DeleteConnection(label string) error {
	if label == "" {
		return fmt.Errorf("Empty connection label.")
	}
	conns, err := GetConnectionList()
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	newPairs := []common.KeyURIPair{}
	found := false
	for _, conn := range conns {
		if conn.Key == label {
			found = true
			continue
		}
		newPairs = append(newPairs, conn)
	}

	if !found {
		return fmt.Errorf("Connection with name '%s' does not exist.", label)
	}

	filePath := filepath.Join(homeDir, ".local/share/nvim/simpanan_connections.json")
	data, err := json.MarshalIndent(newPairs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	// Drop any persisted schema cache for this connection. Idempotent;
	// a missing file is not an error.
	_ = DeleteSchemaCacheFile(label)
	return nil
}
