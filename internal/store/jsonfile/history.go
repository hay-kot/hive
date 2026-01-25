package jsonfile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hay-kot/hive/internal/core/history"
)

// historyFile is the root JSON structure stored on disk.
type historyFile struct {
	Entries []history.Entry `json:"entries"`
}

// HistoryStore implements history.Store using a JSON file for persistence.
type HistoryStore struct {
	path       string
	maxEntries int
	mu         sync.RWMutex
}

// NewHistoryStore creates a new JSON file history store at the given path.
// maxEntries limits stored entries (0 means unlimited).
func NewHistoryStore(path string, maxEntries int) *HistoryStore {
	return &HistoryStore{path: path, maxEntries: maxEntries}
}

// List returns all history entries, newest first.
func (s *HistoryStore) List(ctx context.Context) ([]history.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, err := s.load()
	if err != nil {
		return nil, err
	}

	return f.Entries, nil
}

// Get returns a history entry by ID. Returns ErrNotFound if not found.
func (s *HistoryStore) Get(ctx context.Context, id string) (history.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, err := s.load()
	if err != nil {
		return history.Entry{}, err
	}

	for _, entry := range f.Entries {
		if entry.ID == id {
			return entry, nil
		}
	}

	return history.Entry{}, history.ErrNotFound
}

// Save adds a new history entry, pruning old entries to stay within maxEntries.
func (s *HistoryStore) Save(ctx context.Context, entry history.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := s.load()
	if err != nil {
		return err
	}

	f.Entries = append([]history.Entry{entry}, f.Entries...)

	if s.maxEntries > 0 && len(f.Entries) > s.maxEntries {
		f.Entries = f.Entries[:s.maxEntries]
	}

	return s.save(f)
}

// Clear removes all history entries.
func (s *HistoryStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.save(historyFile{Entries: []history.Entry{}})
}

// LastFailed returns the most recent failed entry. Returns ErrNotFound if none.
func (s *HistoryStore) LastFailed(ctx context.Context) (history.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, err := s.load()
	if err != nil {
		return history.Entry{}, err
	}

	for _, entry := range f.Entries {
		if entry.Failed() {
			return entry, nil
		}
	}

	return history.Entry{}, history.ErrNotFound
}

// load reads the history file from disk.
// Returns empty historyFile if file doesn't exist.
func (s *HistoryStore) load() (historyFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return historyFile{}, nil
		}
		return historyFile{}, fmt.Errorf("read history file: %w", err)
	}

	if len(data) == 0 {
		return historyFile{}, nil
	}

	var f historyFile
	if err := json.Unmarshal(data, &f); err != nil {
		return historyFile{}, fmt.Errorf("history file corrupted (run 'hive run --clear-history' to reset): %w", err)
	}

	return f, nil
}

// save writes the history file to disk atomically.
func (s *HistoryStore) save(f historyFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write history temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename history file: %w", err)
	}

	return nil
}
