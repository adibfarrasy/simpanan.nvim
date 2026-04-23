package webui

import (
	"errors"
	"os"
	"strings"
	"sync"
)

// OpenFileStatus mirrors the spec's enum of the same name.
type OpenFileStatus string

const (
	StatusClean    OpenFileStatus = "clean"
	StatusModified OpenFileStatus = "modified"
)

// OpenFile is the in-memory shape of a .simp file the user has opened
// in the webui. Mirrors the OpenFile entity in webui.allium.
//
// Status is derived from disk_contents vs buffer_contents per the
// ModifiedReflectsDivergence invariant; callers that mutate buffer
// or disk contents must also update Status via Recompute.
type OpenFile struct {
	Path             string         `json:"path"`
	DiskContents     string         `json:"disk_contents"`
	BufferContents   string         `json:"buffer_contents"`
	CursorByteOffset int            `json:"cursor_byte_offset"`
	Status           OpenFileStatus `json:"status"`
}

// Recompute updates Status to honour ModifiedReflectsDivergence.
func (f *OpenFile) Recompute() {
	if f.DiskContents == f.BufferContents {
		f.Status = StatusClean
	} else {
		f.Status = StatusModified
	}
}

// Errors surfaced by BufferStore. They map to spec rules
// OpenFileRejected (ErrInvalidPath, ErrPathNotFound) and to the
// requires-clauses of CloseFile / SaveFile / EditBuffer / SwitchActiveFile.
var (
	ErrNotSimpFile    = errors.New("path is not a .simp file")
	ErrPathNotFound   = errors.New("path does not exist on disk")
	ErrAlreadyOpen    = errors.New("file is already open; switch active file instead")
	ErrFileNotOpen    = errors.New("no open file at this path")
	ErrEmptyPath      = errors.New("path must not be empty")
)

// BufferStore is the in-memory registry of open files. M3 will plug
// load/flush hooks for the recovery file behind this same interface.
type BufferStore struct {
	mu     sync.RWMutex
	files  map[string]*OpenFile
	active string
}

// NewBufferStore returns an empty store.
func NewBufferStore() *BufferStore {
	return &BufferStore{files: map[string]*OpenFile{}}
}

// validatePath enforces RejectNonSimpFiles + the empty-path guard.
func validatePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}
	if !strings.HasSuffix(path, ".simp") {
		return ErrNotSimpFile
	}
	return nil
}

// Open implements rule OpenFileInWebui: reads the file from disk,
// creates an OpenFile in StatusClean, rejects an already-open path
// per the agent's design decision #4.
func (s *BufferStore) Open(path string) (*OpenFile, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, dup := s.files[path]; dup {
		return nil, ErrAlreadyOpen
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPathNotFound
		}
		return nil, err
	}
	contents := string(bytes)

	f := &OpenFile{
		Path:             path,
		DiskContents:     contents,
		BufferContents:   contents,
		CursorByteOffset: 0,
		Status:           StatusClean,
	}
	s.files[path] = f
	if s.active == "" {
		s.active = path
	}
	return f, nil
}

// Close implements rule CloseFile: drops the OpenFile silently.
// Closing a modified file does not prompt — the buffer state stays
// in the server's recovery file until shutdown.
func (s *BufferStore) Close(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[path]; !ok {
		return ErrFileNotOpen
	}
	delete(s.files, path)
	if s.active == path {
		s.active = ""
		// Promote any other open file to active so ActiveFileIsOpen
		// continues to hold. Map iteration order is fine — the user
		// will reselect with SwitchActive shortly.
		for p := range s.files {
			s.active = p
			break
		}
	}
	return nil
}

// Save implements rule SaveFile: writes buffer_contents to disk and
// updates disk_contents so the file becomes clean.
func (s *BufferStore) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[path]
	if !ok {
		return ErrFileNotOpen
	}
	if err := os.WriteFile(path, []byte(f.BufferContents), 0644); err != nil {
		return err
	}
	f.DiskContents = f.BufferContents
	f.Recompute()
	return nil
}

// Edit implements rule EditBuffer: updates buffer_contents +
// cursor_byte_offset and recomputes Status. Caller is responsible for
// broadcasting to other tabs (BroadcastBufferToTabs).
func (s *BufferStore) Edit(path, newContents string, newCursor int) (*OpenFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[path]
	if !ok {
		return nil, ErrFileNotOpen
	}
	f.BufferContents = newContents
	f.CursorByteOffset = newCursor
	f.Recompute()
	return f, nil
}

// Get returns a snapshot of the OpenFile (caller mustn't mutate).
func (s *BufferStore) Get(path string) (*OpenFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.files[path]
	if !ok {
		return nil, false
	}
	clone := *f
	return &clone, true
}

// List returns a snapshot of every open file. Order is unstable —
// the wire layer should sort if it needs determinism.
func (s *BufferStore) List() []OpenFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OpenFile, 0, len(s.files))
	for _, f := range s.files {
		out = append(out, *f)
	}
	return out
}

// Active returns the path of the active file, or "" if none.
func (s *BufferStore) Active() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// SwitchActive implements rule SwitchActiveFile: requires the file
// to already be in open_files so ActiveFileIsOpen never breaks.
func (s *BufferStore) SwitchActive(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[path]; !ok {
		return ErrFileNotOpen
	}
	s.active = path
	return nil
}
