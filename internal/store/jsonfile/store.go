// Package jsonfile provides a JSON file-based session store.
package jsonfile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hay-kot/hive/internal/core/session"
)

// SessionFile is the root JSON structure stored on disk.
type SessionFile struct {
	Sessions []session.Session `json:"sessions"`
}

// Store implements session.Store using a JSON file for persistence.
type Store struct {
	path string
	mu   sync.RWMutex
}

// New creates a new JSON file store at the given path.
func New(path string) *Store {
	return &Store{path: path}
}

// List returns all sessions.
func (s *Store) List(ctx context.Context) ([]session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := s.load()
	if err != nil {
		return nil, err
	}

	return file.Sessions, nil
}

// Get returns a session by ID. Returns ErrNotFound if not found.
func (s *Store) Get(ctx context.Context, id string) (session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := s.load()
	if err != nil {
		return session.Session{}, err
	}

	for _, sess := range file.Sessions {
		if sess.ID == id {
			return sess, nil
		}
	}

	return session.Session{}, session.ErrNotFound
}

// Save creates or updates a session.
func (s *Store) Save(ctx context.Context, sess session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.load()
	if err != nil {
		return err
	}

	// Update existing or append new
	found := false
	for i, existing := range file.Sessions {
		if existing.ID == sess.ID {
			file.Sessions[i] = sess
			found = true
			break
		}
	}
	if !found {
		file.Sessions = append(file.Sessions, sess)
	}

	return s.save(file)
}

// Delete removes a session by ID. Returns ErrNotFound if not found.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := s.load()
	if err != nil {
		return err
	}

	for i, sess := range file.Sessions {
		if sess.ID == id {
			file.Sessions = append(file.Sessions[:i], file.Sessions[i+1:]...)
			return s.save(file)
		}
	}

	return session.ErrNotFound
}

// FindRecyclable returns a recyclable session for the given remote.
// Returns ErrNoRecyclable if none available.
func (s *Store) FindRecyclable(ctx context.Context, remote string) (session.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := s.load()
	if err != nil {
		return session.Session{}, err
	}

	for _, sess := range file.Sessions {
		if sess.State == session.StateRecycled && sess.Remote == remote {
			return sess, nil
		}
	}

	return session.Session{}, session.ErrNoRecyclable
}

// load reads the session file from disk.
// Returns empty SessionFile if file doesn't exist.
func (s *Store) load() (SessionFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionFile{}, nil
		}
		return SessionFile{}, fmt.Errorf("read sessions file: %w", err)
	}

	if len(data) == 0 {
		return SessionFile{}, nil
	}

	var file SessionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return SessionFile{}, fmt.Errorf("parse sessions file: %w", err)
	}

	return file, nil
}

// save writes the session file to disk atomically.
// Uses write-to-temp-then-rename to prevent corruption from interrupted writes.
func (s *Store) save(file SessionFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
