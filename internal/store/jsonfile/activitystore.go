package jsonfile

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
)

const (
	defaultMaxActivities = 1000
	activityFilename     = "activity.jsonl"
)

// ActivityStore implements messaging.ActivityStore using a JSONL file.
type ActivityStore struct {
	dir           string
	maxActivities int
	mu            sync.Mutex
}

// NewActivityStore creates a new activity store at the given directory.
func NewActivityStore(dir string) *ActivityStore {
	return &ActivityStore{
		dir:           dir,
		maxActivities: defaultMaxActivities,
	}
}

// WithMaxActivities sets the maximum number of activities to retain.
func (s *ActivityStore) WithMaxActivities(max int) *ActivityStore {
	s.maxActivities = max
	return s
}

func (s *ActivityStore) filePath() string {
	return filepath.Join(s.dir, activityFilename)
}

func (s *ActivityStore) lockPath() string {
	return s.filePath() + ".lock"
}

// withExclusiveLock executes fn while holding an exclusive file lock.
func (s *ActivityStore) withExclusiveLock(fn func() error) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create activity directory: %w", err)
	}

	f, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire file lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

// generateActivityID creates a unique activity ID.
func generateActivityID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Record records an activity event.
func (s *ActivityStore) Record(activity messaging.Activity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withExclusiveLock(func() error {
		// Set ID and timestamp if not provided
		if activity.ID == "" {
			activity.ID = generateActivityID()
		}
		if activity.Timestamp.IsZero() {
			activity.Timestamp = time.Now()
		}

		// Read existing activities
		activities, err := s.readActivitiesUnsafe()
		if err != nil {
			return err
		}

		// Append new activity
		activities = append(activities, activity)

		// Enforce retention limit
		if len(activities) > s.maxActivities {
			activities = activities[len(activities)-s.maxActivities:]
		}

		// Write back
		return s.writeActivitiesUnsafe(activities)
	})
}

// List returns recent activity events, newest first.
func (s *ActivityStore) List(limit int) ([]messaging.Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []messaging.Activity
	err := s.withExclusiveLock(func() error {
		activities, err := s.readActivitiesUnsafe()
		if err != nil {
			return err
		}

		// Reverse to get newest first
		for i := len(activities) - 1; i >= 0; i-- {
			result = append(result, activities[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return nil
	})
	return result, err
}

// ListSince returns activity events since the given time, newest first.
func (s *ActivityStore) ListSince(since time.Time, limit int) ([]messaging.Activity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []messaging.Activity
	err := s.withExclusiveLock(func() error {
		activities, err := s.readActivitiesUnsafe()
		if err != nil {
			return err
		}

		// Reverse to get newest first, filter by time
		for i := len(activities) - 1; i >= 0; i-- {
			if activities[i].Timestamp.After(since) {
				result = append(result, activities[i])
				if limit > 0 && len(result) >= limit {
					break
				}
			}
		}
		return nil
	})
	return result, err
}

// readActivitiesUnsafe reads all activities from the file.
// Caller must hold lock.
func (s *ActivityStore) readActivitiesUnsafe() ([]messaging.Activity, error) {
	f, err := os.Open(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open activity file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var activities []messaging.Activity
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var activity messaging.Activity
		if err := json.Unmarshal(scanner.Bytes(), &activity); err != nil {
			// Skip malformed lines
			continue
		}
		activities = append(activities, activity)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read activity file: %w", err)
	}

	return activities, nil
}

// writeActivitiesUnsafe writes all activities to the file.
// Caller must hold lock.
func (s *ActivityStore) writeActivitiesUnsafe(activities []messaging.Activity) error {
	tmpPath := s.filePath() + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	enc := json.NewEncoder(f)
	for _, a := range activities {
		if err := enc.Encode(a); err != nil {
			f.Close() //nolint:errcheck
			_ = os.Remove(tmpPath)
			return fmt.Errorf("write activity: %w", err)
		}
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath()); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
