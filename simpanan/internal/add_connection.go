package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"simpanan/internal/common"
	"strings"
)

// HandleAddConnection is the rplugin-shaped entry: args[0] is
// "label>uri". Thin shim over AddConnection so both clients share a
// single canonical implementation.
func HandleAddConnection(args []string) (string, error) {
	if len(args) == 0 || len(args[0]) == 0 {
		return "", fmt.Errorf("Empty connection input.")
	}
	c := strings.SplitN(args[0], ">", 2)
	if len(c) < 2 {
		return "", fmt.Errorf("Invalid connection syntax '%s'; expected 'label>uri'.", args[0])
	}
	if err := AddConnection(strings.TrimSpace(c[0]), strings.TrimSpace(c[1])); err != nil {
		return "", err
	}
	return "Success", nil
}

// AddConnection is the structured entry point used by the webui and
// reused under the rplugin shim. Validates the inputs, persists the
// new pair to the registry, and best-effort warms the schema cache
// for eligible types.
func AddConnection(label, uri string) error {
	if label == "" {
		return fmt.Errorf("Empty connection label.")
	}
	if label == "jq" {
		return fmt.Errorf("New connection name cannot be 'jq'.")
	}
	if strings.Contains(label, ">") {
		return fmt.Errorf("Connection label cannot contain '>'.")
	}
	if uri == "" {
		return fmt.Errorf("Empty connection uri.")
	}
	if _, err := common.URI(uri).ConnType(); err != nil {
		return fmt.Errorf("Unrecognised uri scheme: '%s'.", uri)
	}

	conns, err := GetConnectionList()
	if err != nil {
		return err
	}
	for _, conn := range conns {
		if label == conn.Key {
			return fmt.Errorf("Connection with name '%s' already exists.", label)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	conns = append(conns, common.KeyURIPair{
		Key: label,
		URI: common.URI(uri),
	})

	filePath := filepath.Join(homeDir, ".local/share/nvim/simpanan_connections.json")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(conns, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	// Best-effort schema warm-up. Failures do not abort AddConnection —
	// the cache will be populated lazily at first completion request.
	if ct, _ := common.URI(uri).ConnType(); ct != nil {
		PopulateSchemaCacheForConnection(label, uri, *ct)
	}
	return nil
}
