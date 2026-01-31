package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
	"github.com/hay-kot/hive/internal/store/jsonfile"
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

	// activity store (lazy initialized)
	activityStore *jsonfile.ActivityStore
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
		UsageText: "hive msg sub [--topic <pattern>] [--last N] [--listen]",
		Description: `Reads messages from topics, optionally filtering by topic pattern.

By default, returns all messages as JSON and exits. Use --listen to poll for new messages,
or --wait to block until a single message arrives (useful for inter-agent handoff).

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
  hive msg sub --wait --topic handoff   # wait for single message (24h default timeout)`,
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

	sessionID := cmd.detectSessionID(ctx)
	msg := messaging.Message{
		Topic:     cmd.pubTopic,
		Payload:   payload,
		Sender:    cmd.pubSender,
		SessionID: sessionID,
	}

	if err := store.Publish(ctx, msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	// Record activity (best effort, don't fail on error)
	_ = cmd.getActivityStore().Record(messaging.Activity{
		Type:      messaging.ActivityPublish,
		Topic:     cmd.pubTopic,
		SessionID: sessionID,
		Sender:    cmd.pubSender,
		MessageID: msg.ID,
	})

	return nil
}

func (cmd *MsgCmd) runSub(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()

	topic := cmd.subTopic
	if topic == "" {
		topic = "*"
	}

	// Record subscribe activity (best effort)
	sessionID := cmd.detectSessionID(ctx)
	_ = cmd.getActivityStore().Record(messaging.Activity{
		Type:      messaging.ActivitySubscribe,
		Topic:     topic,
		SessionID: sessionID,
	})

	// Wait mode: wait for a single message and exit
	if cmd.subWait {
		return cmd.waitForMessage(ctx, c, store, topic)
	}

	// Listen mode: poll for new messages
	if cmd.subListen {
		return cmd.listenForMessages(ctx, c, store, topic)
	}

	// Default: return messages immediately
	messages, err := store.Subscribe(ctx, topic, time.Time{})
	if err != nil {
		if errors.Is(err, messaging.ErrTopicNotFound) {
			return nil // No messages, no output
		}
		return fmt.Errorf("subscribe: %w", err)
	}

	// Apply --last N limit if specified
	if cmd.subLast > 0 && len(messages) > cmd.subLast {
		messages = messages[len(messages)-cmd.subLast:]
	}

	return cmd.printMessages(c.Root().Writer, messages)
}

func (cmd *MsgCmd) listenForMessages(ctx context.Context, c *cli.Command, store *jsonfile.MsgStore, topic string) error {
	timeout, err := time.ParseDuration(cmd.subTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	deadline := time.Now().Add(timeout)
	since := time.Now()
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

func (cmd *MsgCmd) waitForMessage(ctx context.Context, c *cli.Command, store *jsonfile.MsgStore, topic string) error {
	// Use 24h default for --wait mode (essentially forever for handoff scenarios)
	timeout := 24 * time.Hour
	if cmd.subTimeout != "30s" { // User explicitly set a timeout
		var err error
		timeout, err = time.ParseDuration(cmd.subTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
	}

	deadline := time.Now().Add(timeout)
	since := time.Now()
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

func (cmd *MsgCmd) getActivityStore() *jsonfile.ActivityStore {
	if cmd.activityStore == nil {
		activityDir := filepath.Join(cmd.flags.DataDir, "messages")
		cmd.activityStore = jsonfile.NewActivityStore(activityDir)
	}
	return cmd.activityStore
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
