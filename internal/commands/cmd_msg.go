package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/hay-kot/hive/pkg/randid"
	"github.com/urfave/cli/v3"
)

type MsgCmd struct {
	flags *Flags

	// pub flags
	pubTopic  string
	pubFile   string
	pubSender string

	// sub flags
	subTopic   string
	subTimeout string
	subLast    int
	subListen  bool
	subWait    bool
	subNew     bool

	// topic flags
	topicNew    bool
	topicPrefix string
}

// NewMsgCmd creates a new msg command.
func NewMsgCmd(flags *Flags) *MsgCmd {
	return &MsgCmd{flags: flags}
}

// Register adds the msg command to the application.
func (cmd *MsgCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "msg",
		Usage: "Publish and subscribe to inter-agent messages",
		Description: `Message commands for inter-agent communication.

Messages are stored in topic-based JSON files at $XDG_DATA_HOME/hive/messages/topics/.
Each topic is a separate file, allowing agents to communicate via named channels.

The sender is auto-detected from the current working directory's hive session.`,
		Commands: []*cli.Command{
			cmd.pubCmd(),
			cmd.subCmd(),
			cmd.listCmd(),
			cmd.topicCmd(),
		},
	})

	return app
}

func (cmd *MsgCmd) pubCmd() *cli.Command {
	return &cli.Command{
		Name:      "pub",
		Usage:     "Publish a message to a topic",
		UsageText: "hive msg pub --topic <topic> [message]",
		Description: `Publishes a message to the specified topic.

The message can be provided as:
- A command-line argument
- From a file with -f/--file
- From stdin if no argument is provided

The sender is auto-detected from the current hive session, or can be overridden with --sender.

Examples:
  hive msg pub --topic build.started "Build starting"
  echo "Hello" | hive msg pub --topic greetings
  hive msg pub --topic logs -f build.log`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "topic",
				Aliases:     []string{"t"},
				Usage:       "topic to publish to",
				Required:    true,
				Destination: &cmd.pubTopic,
			},
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "read message from file",
				Destination: &cmd.pubFile,
			},
			&cli.StringFlag{
				Name:        "sender",
				Aliases:     []string{"s"},
				Usage:       "override sender ID (default: auto-detect from session)",
				Destination: &cmd.pubSender,
			},
		},
		Action: cmd.runPub,
	}
}

func (cmd *MsgCmd) subCmd() *cli.Command {
	return &cli.Command{
		Name:      "sub",
		Usage:     "Read messages from a topic",
		UsageText: "hive msg sub [--topic <pattern>] [--last N] [--listen] [--new]",
		Description: `Reads messages from topics, optionally filtering by topic pattern.

By default, returns all messages as JSON and exits. Use --listen to poll for new messages,
or --wait to block until a single message arrives (useful for inter-agent handoff).

Use --new to filter messages since your last inbox read (only works for inbox topics).

Topic patterns:
- No topic or "*": all messages
- "exact.topic": exact topic match
- "prefix.*": wildcard match for topics starting with "prefix."

Examples:
  hive msg sub                          # all messages as JSON
  hive msg sub --topic agent.build      # specific topic
  hive msg sub --topic agent.*          # wildcard pattern
  hive msg sub --last 10                # last 10 messages
  hive msg sub --listen                 # poll for new messages
  hive msg sub --wait --topic handoff   # wait for single message (24h default timeout)
  hive msg sub -t agent.abc.inbox --new # only unread inbox messages`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "topic",
				Aliases:     []string{"t"},
				Usage:       "topic pattern to subscribe to (supports wildcards like agent.*)",
				Destination: &cmd.subTopic,
			},
			&cli.IntFlag{
				Name:        "last",
				Aliases:     []string{"n"},
				Usage:       "return only last N messages",
				Destination: &cmd.subLast,
			},
			&cli.BoolFlag{
				Name:        "listen",
				Aliases:     []string{"l"},
				Usage:       "poll for new messages instead of returning immediately",
				Destination: &cmd.subListen,
			},
			&cli.BoolFlag{
				Name:        "wait",
				Aliases:     []string{"w"},
				Usage:       "wait for a single message and exit (for inter-agent handoff)",
				Destination: &cmd.subWait,
			},
			&cli.BoolFlag{
				Name:        "new",
				Usage:       "only return messages since last inbox read (for inbox topics)",
				Destination: &cmd.subNew,
			},
			&cli.StringFlag{
				Name:        "timeout",
				Usage:       "timeout for --listen/--wait mode (e.g., 30s, 5m, 24h)",
				Value:       "30s",
				Destination: &cmd.subTimeout,
			},
		},
		Action: cmd.runSub,
	}
}

func (cmd *MsgCmd) listCmd() *cli.Command {
	return &cli.Command{
		Name:      "list",
		Usage:     "List all topics",
		UsageText: "hive msg list",
		Description: `Lists all topics with their message counts as JSON.

Examples:
  hive msg list`,
		Action: cmd.runList,
	}
}

func (cmd *MsgCmd) topicCmd() *cli.Command {
	return &cli.Command{
		Name:      "topic",
		Usage:     "Generate a random topic ID",
		UsageText: "hive msg topic [--prefix <prefix>]",
		Description: `Generates a random topic ID for inter-agent communication.

The generated topic ID follows the format "<prefix>.<4-char-alphanumeric>".
The prefix defaults to "agent" but can be configured via messaging.topic_prefix
in your config file, or overridden with --prefix.

Examples:
  hive msg topic              # outputs: agent.x7k2 (using config prefix)
  hive msg topic --prefix task   # outputs: task.x7k2
  hive msg topic --prefix ""     # outputs: x7k2 (no prefix)`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "new",
				Aliases:     []string{"n"},
				Usage:       "generate a new random topic ID (default behavior)",
				Destination: &cmd.topicNew,
			},
			&cli.StringFlag{
				Name:        "prefix",
				Aliases:     []string{"p"},
				Usage:       "topic prefix (overrides config, use empty string for no prefix)",
				Destination: &cmd.topicPrefix,
			},
		},
		Action: cmd.runTopic,
	}
}

func (cmd *MsgCmd) runTopic(_ context.Context, c *cli.Command) error {
	// Determine prefix: flag override > config > default "agent"
	prefix := cmd.flags.Config.Messaging.TopicPrefix
	if c.IsSet("prefix") {
		prefix = cmd.topicPrefix
	}

	// Generate topic ID
	id := randid.Generate(4)
	var topicID string
	if prefix != "" {
		topicID = prefix + "." + id
	} else {
		topicID = id
	}

	_, err := fmt.Fprintln(c.Root().Writer, topicID)
	return err
}

func (cmd *MsgCmd) runPub(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()

	// Determine message content
	var payload string
	switch {
	case c.NArg() >= 1:
		payload = c.Args().Get(0)
	case cmd.pubFile != "":
		data, err := os.ReadFile(cmd.pubFile)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		payload = string(data)
	default:
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		payload = string(data)
	}

	msg := messaging.Message{
		Topic:     cmd.pubTopic,
		Payload:   payload,
		Sender:    cmd.pubSender,
		SessionID: cmd.detectSessionID(ctx),
	}

	if err := store.Publish(ctx, msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	return nil
}

func (cmd *MsgCmd) runSub(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()

	topic := cmd.subTopic
	if topic == "" {
		topic = "*"
	}

	// Determine since timestamp for --new flag
	since := time.Time{}
	if cmd.subNew {
		since = cmd.getLastInboxRead(ctx, topic)
	}

	// Wait mode: wait for a single message and exit
	if cmd.subWait {
		return cmd.waitForMessage(ctx, c, store, topic, since)
	}

	// Listen mode: poll for new messages
	if cmd.subListen {
		return cmd.listenForMessages(ctx, c, store, topic, since)
	}

	// Default: return messages immediately
	messages, err := store.Subscribe(ctx, topic, since)
	if err != nil {
		if errors.Is(err, messaging.ErrTopicNotFound) {
			return nil // No messages, no output
		}
		return fmt.Errorf("subscribe: %w", err)
	}

	// Update inbox read timestamp if subscribing to own inbox
	cmd.updateInboxReadIfOwn(ctx, topic)

	// Apply --last N limit if specified
	if cmd.subLast > 0 && len(messages) > cmd.subLast {
		messages = messages[len(messages)-cmd.subLast:]
	}

	return cmd.printMessages(c.Root().Writer, messages)
}

func (cmd *MsgCmd) listenForMessages(ctx context.Context, c *cli.Command, store *jsonfile.MsgStore, topic string, initialSince time.Time) error {
	timeout, err := time.ParseDuration(cmd.subTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Update inbox read timestamp if subscribing to own inbox
	cmd.updateInboxReadIfOwn(ctx, topic)

	deadline := time.Now().Add(timeout)
	// Use initialSince if set (from --new flag), otherwise start from now
	since := initialSince
	if since.IsZero() {
		since = time.Now()
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil // Timeout reached, exit silently
			}

			messages, err := store.Subscribe(ctx, topic, since)
			if err != nil && !errors.Is(err, messaging.ErrTopicNotFound) {
				return fmt.Errorf("subscribe: %w", err)
			}

			if len(messages) > 0 {
				if err := cmd.printMessages(c.Root().Writer, messages); err != nil {
					return err
				}
				since = messages[len(messages)-1].CreatedAt
			}
		}
	}
}

func (cmd *MsgCmd) waitForMessage(ctx context.Context, c *cli.Command, store *jsonfile.MsgStore, topic string, initialSince time.Time) error {
	// Use 24h default for --wait mode (essentially forever for handoff scenarios)
	timeout := 24 * time.Hour
	if cmd.subTimeout != "30s" { // User explicitly set a timeout
		var err error
		timeout, err = time.ParseDuration(cmd.subTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Update inbox read timestamp if subscribing to own inbox
	cmd.updateInboxReadIfOwn(ctx, topic)

	deadline := time.Now().Add(timeout)
	// Use initialSince if set (from --new flag), otherwise start from now
	since := initialSince
	if since.IsZero() {
		since = time.Now()
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for message on topic %q", topic)
			}

			messages, err := store.Subscribe(ctx, topic, since)
			if err != nil && !errors.Is(err, messaging.ErrTopicNotFound) {
				return fmt.Errorf("subscribe: %w", err)
			}

			if len(messages) > 0 {
				// Return only the first message and exit
				return cmd.printMessages(c.Root().Writer, messages[:1])
			}
		}
	}
}

func (cmd *MsgCmd) runList(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()

	topics, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("list topics: %w", err)
	}

	if len(topics) == 0 {
		return nil // No topics, no output
	}

	// Get message counts for each topic
	type topicInfo struct {
		Name         string `json:"name"`
		MessageCount int    `json:"message_count"`
	}

	var infos []topicInfo
	for _, t := range topics {
		messages, err := store.Subscribe(ctx, t, time.Time{})
		count := 0
		if err == nil {
			count = len(messages)
		}
		infos = append(infos, topicInfo{Name: t, MessageCount: count})
	}

	enc := json.NewEncoder(c.Root().Writer)
	for _, info := range infos {
		if err := enc.Encode(info); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *MsgCmd) getMsgStore() *jsonfile.MsgStore {
	topicsDir := filepath.Join(cmd.flags.DataDir, "messages", "topics")
	return jsonfile.NewMsgStore(topicsDir)
}

func (cmd *MsgCmd) detectSessionID(ctx context.Context) string {
	sessionsPath := filepath.Join(cmd.flags.DataDir, "sessions.json")
	sessStore := jsonfile.New(sessionsPath)
	detector := messaging.NewSessionDetector(sessStore)

	sessionID, _ := detector.DetectSession(ctx)
	return sessionID
}

func (cmd *MsgCmd) printMessages(w io.Writer, messages []messaging.Message) error {
	enc := json.NewEncoder(w)
	for _, msg := range messages {
		if err := enc.Encode(msg); err != nil {
			return err
		}
	}
	return nil
}

// updateInboxReadIfOwn updates the session's LastInboxRead timestamp if the
// subscribed topic matches the current session's inbox pattern (agent.<id>.inbox).
func (cmd *MsgCmd) updateInboxReadIfOwn(ctx context.Context, topic string) {
	// Only process exact inbox topics, not wildcards
	if !strings.HasSuffix(topic, ".inbox") {
		return
	}

	// Extract session ID from topic (agent.<id>.inbox)
	parts := strings.Split(topic, ".")
	if len(parts) != 3 || parts[0] != "agent" || parts[2] != "inbox" {
		return
	}
	topicSessionID := parts[1]

	// Get current session ID
	currentSessionID := cmd.detectSessionID(ctx)
	if currentSessionID == "" || currentSessionID != topicSessionID {
		return
	}

	// Update the session's LastInboxRead
	sessionsPath := filepath.Join(cmd.flags.DataDir, "sessions.json")
	sessStore := jsonfile.New(sessionsPath)

	sess, err := sessStore.Get(ctx, currentSessionID)
	if err != nil {
		return // Silently ignore errors
	}

	sess.UpdateLastInboxRead(time.Now())
	_ = sessStore.Save(ctx, sess) // Silently ignore errors
}

// getLastInboxRead returns the LastInboxRead timestamp for the session that owns
// the given inbox topic. Returns zero time if not found or not an inbox topic.
func (cmd *MsgCmd) getLastInboxRead(ctx context.Context, topic string) time.Time {
	// Only process exact inbox topics, not wildcards
	if !strings.HasSuffix(topic, ".inbox") {
		return time.Time{}
	}

	// Extract session ID from topic (agent.<id>.inbox)
	parts := strings.Split(topic, ".")
	if len(parts) != 3 || parts[0] != "agent" || parts[2] != "inbox" {
		return time.Time{}
	}
	topicSessionID := parts[1]

	// Get the session's LastInboxRead
	sessionsPath := filepath.Join(cmd.flags.DataDir, "sessions.json")
	sessStore := jsonfile.New(sessionsPath)

	sess, err := sessStore.Get(ctx, topicSessionID)
	if err != nil {
		return time.Time{}
	}

	if sess.LastInboxRead == nil {
		return time.Time{}
	}
	return *sess.LastInboxRead
}
