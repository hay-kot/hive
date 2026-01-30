package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/hay-kot/hive/internal/core/messaging"
	"github.com/hay-kot/hive/internal/printer"
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

	// list flags
	jsonOut bool
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
		Usage:     "Subscribe to messages",
		UsageText: "hive msg sub [--topic <pattern>] [--last N] [--timeout duration]",
		Description: `Subscribes to messages, optionally filtering by topic pattern.

Topic patterns:
- No topic or "*": all messages
- "exact.topic": exact topic match
- "prefix.*": wildcard match for topics starting with "prefix."

Options:
- --last N returns the last N messages immediately
- Without --last, polls for new messages until timeout

Examples:
  hive msg sub                          # all messages
  hive msg sub --topic agent.build      # specific topic
  hive msg sub --topic agent.*          # wildcard pattern
  hive msg sub --last 10                # last 10 messages`,
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
				Usage:       "return last N messages immediately",
				Destination: &cmd.subLast,
			},
			&cli.StringFlag{
				Name:        "timeout",
				Usage:       "timeout for waiting for new messages (e.g., 30s, 5m)",
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
		UsageText: "hive msg list [--json]",
		Description: `Lists all topics with their message counts.

Examples:
  hive msg list
  hive msg list --json`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "json",
				Usage:       "output as JSON",
				Destination: &cmd.jsonOut,
			},
		},
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

	// Determine sender
	sender := cmd.pubSender
	if sender == "" {
		sender = cmd.detectSender(ctx)
	}

	msg := messaging.Message{
		Topic:   cmd.pubTopic,
		Payload: payload,
		Sender:  sender,
	}

	if err := store.Publish(ctx, msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	return nil
}

func (cmd *MsgCmd) runSub(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()
	p := printer.Ctx(ctx)

	topic := cmd.subTopic
	if topic == "" {
		topic = "*"
	}

	// If --last is specified, return last N messages immediately
	if cmd.subLast > 0 {
		messages, err := store.Subscribe(ctx, topic, time.Time{})
		if err != nil {
			if errors.Is(err, messaging.ErrTopicNotFound) {
				p.Infof("No messages found")
				return nil
			}
			return fmt.Errorf("subscribe: %w", err)
		}

		// Get last N messages
		start := 0
		if len(messages) > cmd.subLast {
			start = len(messages) - cmd.subLast
		}
		messages = messages[start:]

		return cmd.printMessages(c.Root().Writer, messages)
	}

	// Poll for new messages
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

func (cmd *MsgCmd) runList(ctx context.Context, c *cli.Command) error {
	store := cmd.getMsgStore()
	p := printer.Ctx(ctx)

	topics, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("list topics: %w", err)
	}

	if len(topics) == 0 {
		if !cmd.jsonOut {
			p.Infof("No topics found")
		} else {
			_, _ = fmt.Fprintln(c.Root().Writer, "[]")
		}
		return nil
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

	if cmd.jsonOut {
		enc := json.NewEncoder(c.Root().Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}

	out := c.Root().Writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TOPIC\tMESSAGES")

	for _, info := range infos {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", info.Name, info.MessageCount)
	}

	return w.Flush()
}

func (cmd *MsgCmd) getMsgStore() *jsonfile.MsgStore {
	topicsDir := filepath.Join(cmd.flags.DataDir, "messages", "topics")
	return jsonfile.NewMsgStore(topicsDir)
}

func (cmd *MsgCmd) detectSender(ctx context.Context) string {
	sessionsPath := filepath.Join(cmd.flags.DataDir, "sessions.json")
	sessStore := jsonfile.New(sessionsPath)
	detector := messaging.NewSessionDetector(sessStore)

	sender, _ := detector.DetectSession(ctx)
	return sender
}

func (cmd *MsgCmd) printMessages(w io.Writer, messages []messaging.Message) error {
	for _, msg := range messages {
		var senderPart string
		if msg.Sender != "" {
			senderPart = fmt.Sprintf(" [%s]", msg.Sender)
		}
		_, err := fmt.Fprintf(w, "[%s] %s%s: %s\n",
			msg.CreatedAt.Format("15:04:05"),
			msg.Topic,
			senderPart,
			msg.Payload,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
