package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HandleRefreshSchemas drops one or all schema caches so the next
// completion request will re-introspect. A missing or empty label
// clears every cache; a non-empty label targets just that one.
//
// Silent on "cache did not exist" so the command is idempotent: the
// user does not need to check beforehand.
func HandleRefreshSchemas(args []string) (string, error) {
	label := ""
	if len(args) > 0 {
		label = strings.TrimSpace(args[0])
	}
	if label == "" {
		if err := dropAllSchemaCaches(); err != nil {
			return "", err
		}
		return "Success: refreshed all schema caches.", nil
	}
	if err := DeleteSchemaCacheFile(label); err != nil {
		return "", err
	}
	return fmt.Sprintf("Success: refreshed schema cache for %q.", label), nil
}

// dropAllSchemaCaches removes every file in the schema cache directory.
// A missing directory is treated as a no-op.
func dropAllSchemaCaches() error {
	dir, err := schemaCacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
