package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	ctxpkg "github.com/hay-kot/hive/internal/core/context"
)

// KVFile is the root JSON structure stored on disk for KV data.
type KVFile struct {
	Entries map[string]ctxpkg.Entry `json:"entries"`
}

// KVStore implements ctxpkg.Store using a JSON file for persistence.
type KVStore struct {
	path string
	mu   sync.RWMutex
}

// NewKVStore creates a new JSON file KV store at the given path.
func NewKVStore(path string) *KVStore {
	return &KVStore{path: path}
}

// lockPath returns the path to the lock file.
func (s *KVStore) lockPath() string {
	return s.path + ".lock"
}

// withSharedLock executes fn while holding a shared (read) file lock.
// Multiple processes can hold shared locks simultaneously.
func (s *KVStore) withSharedLock(fn func() error) error {
	return s.withFileLock(syscall.LOCK_SH, fn)
}

// withExclusiveLock executes fn while holding an exclusive (write) file lock.
// Only one process can hold an exclusive lock at a time.
func (s *KVStore) withExclusiveLock(fn func() error) error {
	return s.withFileLock(syscall.LOCK_EX, fn)
}

// withFileLock acquires a file lock, executes fn, then releases the lock.
func (s *KVStore) withFileLock(lockType int, fn func() error) error {
	// Ensure parent directory exists for lock file
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}

	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if err := syscall.Flock(int(f.Fd()), lockType); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

// Get returns an entry by key. Returns ErrKeyNotFound if not found.
func (s *KVStore) Get(ctx context.Context, key string) (ctxpkg.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entry ctxpkg.Entry
	var found bool

	err := s.withSharedLock(func() error {
		file, err := s.load()
		if err != nil {
			return err
		}

		entry, found = file.Entries[key]
		return nil
	})
	if err != nil {
		return ctxpkg.Entry{}, err
	}

	if !found {
		return ctxpkg.Entry{}, ctxpkg.ErrKeyNotFound
	}

	return entry, nil
}

// Set creates or updates an entry.
func (s *KVStore) Set(ctx context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withExclusiveLock(func() error {
		file, err := s.load()
		if err != nil {
			return err
		}

		now := time.Now()
		entry, exists := file.Entries[key]
		if exists {
			entry.Value = value
			entry.UpdatedAt = now
		} else {
			entry = ctxpkg.Entry{
				Key:       key,
				Value:     value,
				CreatedAt: now,
				UpdatedAt: now,
			}
		}

		file.Entries[key] = entry
		return s.save(file)
	})
}

// Delete removes an entry by key. Returns ErrKeyNotFound if not found.
func (s *KVStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var notFound bool

	err := s.withExclusiveLock(func() error {
		file, err := s.load()
		if err != nil {
			return err
		}

		if _, ok := file.Entries[key]; !ok {
			notFound = true
			return nil
		}

		delete(file.Entries, key)
		return s.save(file)
	})
	if err != nil {
		return err
	}

	if notFound {
		return ctxpkg.ErrKeyNotFound
	}

	return nil
}

// List returns all entries matching the prefix.
func (s *KVStore) List(ctx context.Context, prefix string) ([]ctxpkg.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []ctxpkg.Entry

	err := s.withSharedLock(func() error {
		file, err := s.load()
		if err != nil {
			return err
		}

		for _, entry := range file.Entries {
			if prefix == "" || strings.HasPrefix(entry.Key, prefix) {
				entries = append(entries, entry)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
}

// Watch polls for key updates until UpdatedAt > after or timeout.
func (s *KVStore) Watch(ctx context.Context, key string, after time.Time, timeout time.Duration) (ctxpkg.Entry, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctxpkg.Entry{}, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return ctxpkg.Entry{}, context.DeadlineExceeded
			}

			entry, err := s.Get(ctx, key)
			if errors.Is(err, ctxpkg.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return ctxpkg.Entry{}, err
			}

			if entry.UpdatedAt.After(after) {
				return entry, nil
			}
		}
	}
}

// load reads the KV file from disk.
// Returns empty KVFile if file doesn't exist.
func (s *KVStore) load() (KVFile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return KVFile{Entries: make(map[string]ctxpkg.Entry)}, nil
		}
		return KVFile{}, err
	}

	if len(data) == 0 {
		return KVFile{Entries: make(map[string]ctxpkg.Entry)}, nil
	}

	var file KVFile
	if err := json.Unmarshal(data, &file); err != nil {
		return KVFile{}, fmt.Errorf("parse %s: %w", s.path, err)
	}

	if file.Entries == nil {
		file.Entries = make(map[string]ctxpkg.Entry)
	}

	return file, nil
}

// save writes the KV file to disk atomically.
func (s *KVStore) save(file KVFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp) // best effort cleanup
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
