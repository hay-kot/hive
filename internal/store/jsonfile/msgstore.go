package jsonfile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
)

const defaultMaxMessages = 100

// MsgStore implements messaging.Store using per-topic JSON files.
type MsgStore struct {
	topicsDir   string
	maxMessages int
	mu          sync.RWMutex
}

// NewMsgStore creates a new message store at the given directory.
// The topicsDir should be the full path to the topics directory
// (e.g., $XDG_DATA_HOME/hive/messages/topics).
func NewMsgStore(topicsDir string) *MsgStore {
	return &MsgStore{
		topicsDir:   topicsDir,
		maxMessages: defaultMaxMessages,
	}
}

// WithMaxMessages sets the maximum number of messages to retain per topic.
func (s *MsgStore) WithMaxMessages(max int) *MsgStore {
	s.maxMessages = max
	return s
}

// topicPath returns the file path for a topic.
func (s *MsgStore) topicPath(topic string) string {
	// Sanitize topic name for filesystem safety
	safe := strings.ReplaceAll(topic, "/", "_")
	return filepath.Join(s.topicsDir, safe+".json")
}

// lockPath returns the lock file path for a topic.
func (s *MsgStore) lockPath(topic string) string {
	return s.topicPath(topic) + ".lock"
}

// withSharedLock executes fn while holding a shared (read) file lock.
func (s *MsgStore) withSharedLock(topic string, fn func() error) error {
	return s.withFileLock(topic, syscall.LOCK_SH, fn)
}

// withExclusiveLock executes fn while holding an exclusive (write) file lock.
func (s *MsgStore) withExclusiveLock(topic string, fn func() error) error {
	return s.withFileLock(topic, syscall.LOCK_EX, fn)
}

// withFileLock acquires a file lock, executes fn, then releases the lock.
func (s *MsgStore) withFileLock(topic string, lockType int, fn func() error) error {
	if err := os.MkdirAll(s.topicsDir, 0o755); err != nil {
		return fmt.Errorf("create topics directory: %w", err)
	}

	f, err := os.OpenFile(s.lockPath(topic), os.O_CREATE|os.O_RDWR, 0o644)
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

// generateID creates a unique message ID.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Publish adds a message to a topic, creating the topic if it doesn't exist.
func (s *MsgStore) Publish(ctx context.Context, msg messaging.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.withExclusiveLock(msg.Topic, func() error {
		topic, err := s.loadTopic(msg.Topic)
		if err != nil {
			return err
		}

		// Set ID and timestamp if not provided
		if msg.ID == "" {
			msg.ID = generateID()
		}
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = time.Now()
		}

		topic.Messages = append(topic.Messages, msg)
		topic.UpdatedAt = time.Now()

		// Enforce retention limit
		if len(topic.Messages) > s.maxMessages {
			topic.Messages = topic.Messages[len(topic.Messages)-s.maxMessages:]
		}

		return s.saveTopic(topic)
	})
}

// Subscribe returns all messages for a topic pattern, optionally filtered by since timestamp.
// The topic parameter supports wildcards:
//   - "*" or "" returns messages from all topics
//   - "prefix.*" matches topics starting with "prefix."
//
// Returns ErrTopicNotFound if no matching topics exist.
func (s *MsgStore) Subscribe(ctx context.Context, topic string, since time.Time) ([]messaging.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics, err := s.matchingTopics(topic)
	if err != nil {
		return nil, err
	}

	if len(topics) == 0 {
		return nil, messaging.ErrTopicNotFound
	}

	var messages []messaging.Message
	for _, t := range topics {
		err := s.withSharedLock(t, func() error {
			topicData, err := s.loadTopic(t)
			if err != nil {
				return err
			}

			for _, msg := range topicData.Messages {
				if since.IsZero() || msg.CreatedAt.After(since) {
					messages = append(messages, msg)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Sort by creation time
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	return messages, nil
}

// List returns all topic names.
func (s *MsgStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.topicsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read topics directory: %w", err)
	}

	var topics []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".lock") {
			// Remove .json suffix and restore slashes
			topic := strings.TrimSuffix(name, ".json")
			topic = strings.ReplaceAll(topic, "_", "/")
			topics = append(topics, topic)
		}
	}

	sort.Strings(topics)
	return topics, nil
}

// Prune removes messages older than the given duration across all topics.
// Returns the number of messages removed.
func (s *MsgStore) Prune(ctx context.Context, olderThan time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topics, err := s.listTopicsUnsafe()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	var removed int

	for _, t := range topics {
		err := s.withExclusiveLock(t, func() error {
			topic, err := s.loadTopic(t)
			if err != nil {
				return err
			}

			var kept []messaging.Message
			for _, msg := range topic.Messages {
				if msg.CreatedAt.After(cutoff) {
					kept = append(kept, msg)
				} else {
					removed++
				}
			}

			if len(kept) != len(topic.Messages) {
				topic.Messages = kept
				topic.UpdatedAt = time.Now()
				return s.saveTopic(topic)
			}
			return nil
		})
		if err != nil {
			return removed, err
		}
	}

	return removed, nil
}

// matchingTopics returns topic names matching the given pattern.
func (s *MsgStore) matchingTopics(pattern string) ([]string, error) {
	topics, err := s.listTopicsUnsafe()
	if err != nil {
		return nil, err
	}

	// Empty pattern or "*" matches all topics
	if pattern == "" || pattern == "*" {
		return topics, nil
	}

	// Wildcard pattern like "prefix.*"
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, "*")
		var matched []string
		for _, t := range topics {
			if strings.HasPrefix(t, prefix) {
				matched = append(matched, t)
			}
		}
		return matched, nil
	}

	// Exact match
	if slices.Contains(topics, pattern) {
		return []string{pattern}, nil
	}

	return nil, nil
}

// listTopicsUnsafe returns all topic names without locking.
// Caller must hold s.mu.
func (s *MsgStore) listTopicsUnsafe() ([]string, error) {
	entries, err := os.ReadDir(s.topicsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read topics directory: %w", err)
	}

	var topics []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".lock") {
			topic := strings.TrimSuffix(name, ".json")
			topic = strings.ReplaceAll(topic, "_", "/")
			topics = append(topics, topic)
		}
	}

	return topics, nil
}

// loadTopic reads a topic file from disk.
// Returns empty topic if file doesn't exist.
func (s *MsgStore) loadTopic(name string) (messaging.Topic, error) {
	path := s.topicPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return messaging.Topic{
				Name:     name,
				Messages: nil,
			}, nil
		}
		return messaging.Topic{}, fmt.Errorf("read topic file: %w", err)
	}

	if len(data) == 0 {
		return messaging.Topic{
			Name:     name,
			Messages: nil,
		}, nil
	}

	var topic messaging.Topic
	if err := json.Unmarshal(data, &topic); err != nil {
		return messaging.Topic{}, fmt.Errorf("parse topic file: %w", err)
	}

	return topic, nil
}

// saveTopic writes a topic file to disk atomically.
func (s *MsgStore) saveTopic(topic messaging.Topic) error {
	if err := os.MkdirAll(s.topicsDir, 0o755); err != nil {
		return fmt.Errorf("create topics directory: %w", err)
	}

	data, err := json.MarshalIndent(topic, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal topic: %w", err)
	}

	path := s.topicPath(topic.Name)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
