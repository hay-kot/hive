package jsonfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ctxpkg "github.com/hay-kot/hive/internal/core/context"
)

func TestKVStore_SetAndGet(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	err := store.Set(ctx, "foo", "bar")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entry, err := store.Get(ctx, "foo")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if entry.Key != "foo" {
		t.Errorf("Key = %q, want %q", entry.Key, "foo")
	}
	if entry.Value != "bar" {
		t.Errorf("Value = %q, want %q", entry.Value, "bar")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestKVStore_GetNotFound(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if !errors.Is(err, ctxpkg.ErrKeyNotFound) {
		t.Errorf("Get error = %v, want ErrKeyNotFound", err)
	}
}

func TestKVStore_UpdatePreservesCreatedAt(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	err := store.Set(ctx, "key", "value1")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entry1, _ := store.Get(ctx, "key")
	time.Sleep(10 * time.Millisecond)

	err = store.Set(ctx, "key", "value2")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entry2, _ := store.Get(ctx, "key")

	if entry2.Value != "value2" {
		t.Errorf("Value = %q, want %q", entry2.Value, "value2")
	}
	if !entry2.CreatedAt.Equal(entry1.CreatedAt) {
		t.Errorf("CreatedAt changed: %v -> %v", entry1.CreatedAt, entry2.CreatedAt)
	}
	if !entry2.UpdatedAt.After(entry1.UpdatedAt) {
		t.Errorf("UpdatedAt should be after original: %v <= %v", entry2.UpdatedAt, entry1.UpdatedAt)
	}
}

func TestKVStore_List(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	_ = store.Set(ctx, "app:config", "value1")
	_ = store.Set(ctx, "app:state", "value2")
	_ = store.Set(ctx, "other:key", "value3")

	entries, err := store.List(ctx, "app:")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("List returned %d entries, want 2", len(entries))
	}

	// List all
	all, _ := store.List(ctx, "")
	if len(all) != 3 {
		t.Errorf("List all returned %d entries, want 3", len(all))
	}
}

func TestKVStore_Delete(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	_ = store.Set(ctx, "key", "value")

	err := store.Delete(ctx, "key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, "key")
	if !errors.Is(err, ctxpkg.ErrKeyNotFound) {
		t.Errorf("Get after delete error = %v, want ErrKeyNotFound", err)
	}
}

func TestKVStore_DeleteNotFound(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if !errors.Is(err, ctxpkg.ErrKeyNotFound) {
		t.Errorf("Delete error = %v, want ErrKeyNotFound", err)
	}
}

func TestKVStore_Watch(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	// Set initial value
	_ = store.Set(ctx, "watch-key", "initial")
	entry1, _ := store.Get(ctx, "watch-key")

	// Start watching in background
	done := make(chan struct{})
	var watchedEntry ctxpkg.Entry
	var watchErr error

	go func() {
		watchedEntry, watchErr = store.Watch(ctx, "watch-key", entry1.UpdatedAt, 5*time.Second)
		close(done)
	}()

	// Wait a bit then update
	time.Sleep(100 * time.Millisecond)
	_ = store.Set(ctx, "watch-key", "updated")

	// Wait for watch to complete
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch timed out")
	}

	if watchErr != nil {
		t.Fatalf("Watch failed: %v", watchErr)
	}
	if watchedEntry.Value != "updated" {
		t.Errorf("Watched value = %q, want %q", watchedEntry.Value, "updated")
	}
}

func TestKVStore_WatchTimeout(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	_, err := store.Watch(ctx, "nonexistent", time.Now(), 100*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Watch error = %v, want DeadlineExceeded", err)
	}
}

func TestKVStore_WatchContextCancellation(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	var watchErr error

	go func() {
		_, watchErr = store.Watch(ctx, "key", time.Now(), 30*time.Second)
		close(done)
	}()

	// Cancel context after brief delay
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not respond to context cancellation")
	}

	if !errors.Is(watchErr, context.Canceled) {
		t.Errorf("Watch error = %v, want context.Canceled", watchErr)
	}
}

func TestKVStore_ConcurrentAccess(t *testing.T) {
	store := NewKVStore(filepath.Join(t.TempDir(), "kv.json"))
	ctx := context.Background()

	const goroutines = 10
	const iterations = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				if err := store.Set(ctx, key, "value"); err != nil {
					t.Errorf("Set failed: %v", err)
					return
				}
				if _, err := store.Get(ctx, key); err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}
				if _, err := store.List(ctx, ""); err != nil {
					t.Errorf("List failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	entries, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("Final List failed: %v", err)
	}
	if len(entries) != goroutines*iterations {
		t.Errorf("Expected %d entries, got %d", goroutines*iterations, len(entries))
	}
}

func TestKVStore_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kv.json")

	// Write invalid JSON
	if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	store := NewKVStore(path)
	ctx := context.Background()

	_, err := store.Get(ctx, "any")
	if err == nil {
		t.Error("Expected error for corrupted JSON, got nil")
	}

	// Verify Set also fails gracefully (can't read existing data)
	err = store.Set(ctx, "key", "value")
	if err == nil {
		t.Error("Expected error for Set with corrupted file, got nil")
	}
}
