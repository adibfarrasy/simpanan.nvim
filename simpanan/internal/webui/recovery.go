package webui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const recoveryFileName = "simpanan_webui_recovery.json"

// recoveryPath returns the on-disk path of the recovery file. Lives
// alongside simpanan_connections.json so all simpanan state lives
// in one directory. The format is intentionally NOT in the spec
// (Excludes), so this is purely an implementation choice.
func recoveryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local/share/nvim", recoveryFileName), nil
}

// recoveryPayload is the on-disk shape of the recovery file. Versioned
// so future format changes can detect and skip stale files.
type recoveryPayload struct {
	Version int        `json:"version"`
	Active  string     `json:"active"`
	Files   []OpenFile `json:"files"`
}

const recoveryVersion = 1

// FlushRecovery writes every open file to the recovery file so the
// next launch can reconstruct the workspace. Best-effort — a write
// failure is returned but the caller may choose to ignore it (the
// shutdown should proceed regardless).
func (s *BufferStore) FlushRecovery() error {
	path, err := recoveryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	payload := recoveryPayload{
		Version: recoveryVersion,
		Active:  s.Active(),
		Files:   s.List(),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".recovery-*.tmp")
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

// LoadRecovery reads the recovery file and rehydrates the store.
// Missing file → no-op (clean start). Corrupt file → returned error
// but the store is left empty (the user gets a fresh workspace
// rather than a broken one).
//
// Per spec rule RestoreSession, OpenFile.status is recomputed from
// disk-vs-buffer at restore time so external edits between sessions
// are reflected (a file that was modified pre-restart and whose
// on-disk copy was edited externally still reads as modified).
//
// Files whose on-disk path no longer exists are restored anyway
// with empty disk_contents and modified status, so the user does
// not silently lose their pre-shutdown buffer.
func (s *BufferStore) LoadRecovery() error {
	path, err := recoveryPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var payload recoveryPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("recovery file %s is corrupt: %w", path, err)
	}
	if payload.Version != recoveryVersion {
		// Future-proofing: skip foreign-version files instead of
		// breaking. The user just gets a clean start.
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range payload.Files {
		f := payload.Files[i]
		// Re-derive disk_contents from the current on-disk state, so
		// status reflects reality rather than what was on disk last
		// session.
		current, err := os.ReadFile(f.Path)
		if err == nil {
			f.DiskContents = string(current)
		} else if os.IsNotExist(err) {
			// File vanished externally — keep buffer, mark as modified
			// against an empty disk so the user can save it back.
			f.DiskContents = ""
		}
		f.Recompute()
		s.files[f.Path] = &f
	}
	if payload.Active != "" {
		if _, ok := s.files[payload.Active]; ok {
			s.active = payload.Active
		}
	}
	return nil
}
