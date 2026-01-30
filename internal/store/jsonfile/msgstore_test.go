package jsonfile

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
)

func TestMsgStore_PublishAndSubscribe(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	msg := messaging.Message{
		Topic:   "test.topic",
		Payload: "hello world",
		Sender:  "test-sender",
	}

	err := store.Publish(ctx, msg)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	messages, err := store.Subscribe(ctx, "test.topic", time.Time{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Subscribe returned %d messages, want 1", len(messages))
	}

	if messages[0].Payload != "hello world" {
		t.Errorf("Payload = %q, want %q", messages[0].Payload, "hello world")
	}
	if messages[0].Sender != "test-sender" {
		t.Errorf("Sender = %q, want %q", messages[0].Sender, "test-sender")
	}
	if messages[0].ID == "" {
		t.Error("ID should be auto-generated")
	}
	if messages[0].CreatedAt.IsZero() {
		t.Error("CreatedAt should be auto-set")
	}
}

func TestMsgStore_SubscribeNotFound(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	_, err := store.Subscribe(ctx, "nonexistent", time.Time{})
	if !errors.Is(err, messaging.ErrTopicNotFound) {
		t.Errorf("Subscribe error = %v, want ErrTopicNotFound", err)
	}
}

func TestMsgStore_SubscribeSince(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish first message
	_ = store.Publish(ctx, messaging.Message{Topic: "events", Payload: "first"})
	time.Sleep(10 * time.Millisecond)

	// Record time between messages
	midpoint := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Publish second message
	_ = store.Publish(ctx, messaging.Message{Topic: "events", Payload: "second"})

	// Subscribe since midpoint
	messages, err := store.Subscribe(ctx, "events", midpoint)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Subscribe returned %d messages, want 1", len(messages))
	}
	if messages[0].Payload != "second" {
		t.Errorf("Payload = %q, want %q", messages[0].Payload, "second")
	}
}

func TestMsgStore_SubscribeWildcard(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish to multiple topics
	_ = store.Publish(ctx, messaging.Message{Topic: "agent.build", Payload: "build started"})
	_ = store.Publish(ctx, messaging.Message{Topic: "agent.test", Payload: "tests running"})
	_ = store.Publish(ctx, messaging.Message{Topic: "other.topic", Payload: "unrelated"})

	// Subscribe with wildcard
	messages, err := store.Subscribe(ctx, "agent.*", time.Time{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("Subscribe returned %d messages, want 2", len(messages))
	}

	payloads := make(map[string]bool)
	for _, m := range messages {
		payloads[m.Payload] = true
	}
	if !payloads["build started"] || !payloads["tests running"] {
		t.Errorf("Missing expected payloads in %v", messages)
	}
}

func TestMsgStore_SubscribeAll(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish to multiple topics
	_ = store.Publish(ctx, messaging.Message{Topic: "topic1", Payload: "msg1"})
	_ = store.Publish(ctx, messaging.Message{Topic: "topic2", Payload: "msg2"})

	// Subscribe to all with empty pattern
	messages, err := store.Subscribe(ctx, "", time.Time{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("Subscribe returned %d messages, want 2", len(messages))
	}

	// Subscribe to all with "*" pattern
	messages, err = store.Subscribe(ctx, "*", time.Time{})
	if err != nil {
		t.Fatalf("Subscribe with * failed: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("Subscribe with * returned %d messages, want 2", len(messages))
	}
}

func TestMsgStore_List(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	_ = store.Publish(ctx, messaging.Message{Topic: "topic.a", Payload: "a"})
	_ = store.Publish(ctx, messaging.Message{Topic: "topic.b", Payload: "b"})

	topics, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("List returned %d topics, want 2", len(topics))
	}

	// Topics should be sorted
	if topics[0] != "topic.a" || topics[1] != "topic.b" {
		t.Errorf("Topics = %v, want [topic.a topic.b]", topics)
	}
}

func TestMsgStore_ListEmpty(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	topics, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(topics) != 0 {
		t.Errorf("List returned %v, want empty", topics)
	}
}

func TestMsgStore_Retention(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics")).WithMaxMessages(3)
	ctx := context.Background()

	// Publish 5 messages
	for i := range 5 {
		err := store.Publish(ctx, messaging.Message{
			Topic:   "test",
			Payload: fmt.Sprintf("msg%d", i),
		})
		if err != nil {
			t.Fatalf("Publish %d failed: %v", i, err)
		}
	}

	messages, err := store.Subscribe(ctx, "test", time.Time{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Should only keep last 3 messages
	if len(messages) != 3 {
		t.Fatalf("Subscribe returned %d messages, want 3", len(messages))
	}

	// Verify oldest messages were dropped
	if messages[0].Payload != "msg2" {
		t.Errorf("First message payload = %q, want %q", messages[0].Payload, "msg2")
	}
	if messages[2].Payload != "msg4" {
		t.Errorf("Last message payload = %q, want %q", messages[2].Payload, "msg4")
	}
}

func TestMsgStore_Prune(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish messages
	_ = store.Publish(ctx, messaging.Message{Topic: "events", Payload: "old"})
	time.Sleep(50 * time.Millisecond)
	_ = store.Publish(ctx, messaging.Message{Topic: "events", Payload: "new"})

	// Prune messages older than 25ms
	removed, err := store.Prune(ctx, 25*time.Millisecond)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("Prune removed %d messages, want 1", removed)
	}

	messages, _ := store.Subscribe(ctx, "events", time.Time{})
	if len(messages) != 1 {
		t.Fatalf("Subscribe returned %d messages after prune, want 1", len(messages))
	}
	if messages[0].Payload != "new" {
		t.Errorf("Remaining message payload = %q, want %q", messages[0].Payload, "new")
	}
}

func TestMsgStore_ConcurrentAccess(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	const goroutines = 10
	const iterations = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				topic := fmt.Sprintf("topic.%d", id)
				err := store.Publish(ctx, messaging.Message{
					Topic:   topic,
					Payload: fmt.Sprintf("msg-%d-%d", id, j),
				})
				if err != nil {
					t.Errorf("Publish failed: %v", err)
					return
				}

				_, err = store.Subscribe(ctx, topic, time.Time{})
				if err != nil {
					t.Errorf("Subscribe failed: %v", err)
					return
				}

				_, err = store.List(ctx)
				if err != nil {
					t.Errorf("List failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	topics, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Final List failed: %v", err)
	}
	if len(topics) != goroutines {
		t.Errorf("Expected %d topics, got %d", goroutines, len(topics))
	}

	// Each topic should have its messages (up to retention limit)
	for _, topic := range topics {
		messages, err := store.Subscribe(ctx, topic, time.Time{})
		if err != nil {
			t.Errorf("Subscribe to %s failed: %v", topic, err)
			continue
		}
		if len(messages) != iterations {
			t.Errorf("Topic %s has %d messages, want %d", topic, len(messages), iterations)
		}
	}
}

func TestMsgStore_MessageOrdering(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish messages with slight delays to ensure different timestamps
	for i := range 5 {
		_ = store.Publish(ctx, messaging.Message{
			Topic:   "ordered",
			Payload: fmt.Sprintf("msg%d", i),
		})
		time.Sleep(time.Millisecond)
	}

	messages, _ := store.Subscribe(ctx, "ordered", time.Time{})

	// Verify messages are in chronological order
	for i := 0; i < len(messages)-1; i++ {
		if !messages[i].CreatedAt.Before(messages[i+1].CreatedAt) {
			t.Errorf("Messages not in order: %v >= %v", messages[i].CreatedAt, messages[i+1].CreatedAt)
		}
	}
}

func TestMsgStore_WildcardOrdering(t *testing.T) {
	store := NewMsgStore(filepath.Join(t.TempDir(), "topics"))
	ctx := context.Background()

	// Publish messages across topics with explicit ordering via delays
	_ = store.Publish(ctx, messaging.Message{Topic: "ns.a", Payload: "first"})
	time.Sleep(5 * time.Millisecond)
	_ = store.Publish(ctx, messaging.Message{Topic: "ns.b", Payload: "second"})
	time.Sleep(5 * time.Millisecond)
	_ = store.Publish(ctx, messaging.Message{Topic: "ns.a", Payload: "third"})

	messages, _ := store.Subscribe(ctx, "ns.*", time.Time{})

	if len(messages) != 3 {
		t.Fatalf("Subscribe returned %d messages, want 3", len(messages))
	}

	// Should be chronologically sorted across all topics
	expected := []string{"first", "second", "third"}
	for i, msg := range messages {
		if msg.Payload != expected[i] {
			t.Errorf("Message %d payload = %q, want %q", i, msg.Payload, expected[i])
		}
	}
}
