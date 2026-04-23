package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const schemaCacheDirName = "simpanan_schema_cache"

// schemaCacheDir returns the on-disk directory that holds per-label
// SchemaCache JSON files. The location mirrors the connection
// registry's ($HOME/.local/share/nvim/) so both live together.
func schemaCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".local/share/nvim", schemaCacheDirName), nil
}

// schemaCachePath returns the JSON file path for a single connection
// label. Rejects path-traversal characters so a crafted label cannot
// escape the schema-cache directory.
func schemaCachePath(label string) (string, error) {
	if label == "" {
		return "", fmt.Errorf("schema cache: label must not be empty")
	}
	if strings.ContainsAny(label, "/\\") || label == "." || label == ".." {
		return "", fmt.Errorf("schema cache: label %q contains unsafe characters", label)
	}
	dir, err := schemaCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, label+".json"), nil
}

// SaveSchemaCache writes a cache to disk, creating the parent directory
// if needed. Writes are atomic via temp-file-and-rename so a partial
// write cannot leave a corrupted file in place.
func SaveSchemaCache(cache *SchemaCache) error {
	if cache == nil {
		return fmt.Errorf("schema cache: cannot save nil cache")
	}
	path, err := schemaCachePath(cache.ConnectionLabel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// LoadSchemaCache reads the on-disk cache for a single label. Returns
// (nil, nil) when no file exists for this label — callers treat that
// as "cache absent, populate from scratch".
func LoadSchemaCache(label string) (*SchemaCache, error) {
	path, err := schemaCachePath(label)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache SchemaCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("schema cache: decoding %s: %w", path, err)
	}
	return &cache, nil
}

// LoadAllSchemaCaches reads every .json file under the schema cache
// directory. A missing directory is treated as "no caches yet". A
// single malformed file does not poison the rest — it's skipped and
// the error is returned alongside the caches successfully loaded.
func LoadAllSchemaCaches() ([]*SchemaCache, error) {
	dir, err := schemaCacheDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var caches []*SchemaCache
	var firstErr error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		label := strings.TrimSuffix(e.Name(), ".json")
		c, err := LoadSchemaCache(label)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if c != nil {
			caches = append(caches, c)
		}
	}
	return caches, firstErr
}

// DeleteSchemaCacheFile removes the on-disk cache for a label. A
// missing file is not an error (idempotent delete).
func DeleteSchemaCacheFile(label string) error {
	path, err := schemaCachePath(label)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
