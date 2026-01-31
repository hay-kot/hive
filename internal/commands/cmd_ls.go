package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/urfave/cli/v3"
)

type LsCmd struct {
	flags *Flags

	// flags
	jsonOutput bool
}

// NewLsCmd creates a new ls command
func NewLsCmd(flags *Flags) *LsCmd {
	return &LsCmd{flags: flags}
}

// Register adds the ls command to the application
func (cmd *LsCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:      "ls",
		Usage:     "List all sessions",
		UsageText: "hive ls [--json]",
		Description: `Displays a table of all sessions with their repo, name, state, and path.

Use --json for LLM-friendly output with additional fields like inbox topic and unread count.`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "json",
				Usage:       "output as JSON lines with inbox info",
				Destination: &cmd.jsonOutput,
			},
		},
		Action: cmd.run,
	})

	return app
}

func (cmd *LsCmd) run(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	sessions, err := cmd.flags.Service.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		if !cmd.jsonOutput {
			p.Infof("No sessions found")
		}
		return nil
	}

	// Separate normal and corrupted sessions
	var normal, corrupted []session.Session
	for _, s := range sessions {
		if s.State == session.StateCorrupted {
			corrupted = append(corrupted, s)
		} else {
			normal = append(normal, s)
		}
	}

	// Sort by repository name
	slices.SortFunc(normal, func(a, b session.Session) int {
		return strings.Compare(git.ExtractRepoName(a.Remote), git.ExtractRepoName(b.Remote))
	})

	out := c.Root().Writer

	// JSON output mode
	if cmd.jsonOutput {
		msgStore := cmd.getMsgStore()
		enc := json.NewEncoder(out)

		for _, s := range normal {
			info := cmd.buildSessionInfo(ctx, s, msgStore)
			if err := enc.Encode(info); err != nil {
				return fmt.Errorf("encode session: %w", err)
			}
		}
		return nil
	}

	// Table output mode
	if len(normal) > 0 {
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "REPO\tNAME\tSTATE\tPATH")

		for _, s := range normal {
			repo := git.ExtractRepoName(s.Remote)
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", repo, s.Name, s.State, s.Path)
		}

		_ = w.Flush()
	}

	if len(corrupted) > 0 {
		_, _ = fmt.Fprintln(out)
		p.Warnf("Found %d corrupted session(s) with invalid git repositories:", len(corrupted))
		for _, s := range corrupted {
			repo := git.ExtractRepoName(s.Remote)
			p.Infof("%s (%s)", repo, s.Path)
		}
		_, _ = fmt.Fprintln(out)
		p.Printf("Run 'hive prune' to clean up")
	}

	return nil
}

// sessionInfo is the JSON output format for hive ls --json.
type sessionInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Repo       string     `json:"repo"`
	Inbox      string     `json:"inbox"`
	LastActive *time.Time `json:"last_active,omitempty"`
	State      string     `json:"state"`
	Unread     int        `json:"unread"`
}

func (cmd *LsCmd) getMsgStore() *jsonfile.MsgStore {
	topicsDir := filepath.Join(cmd.flags.DataDir, "messages", "topics")
	return jsonfile.NewMsgStore(topicsDir)
}

func (cmd *LsCmd) buildSessionInfo(ctx context.Context, s session.Session, msgStore *jsonfile.MsgStore) sessionInfo {
	info := sessionInfo{
		ID:         s.ID,
		Name:       s.Name,
		Repo:       git.ExtractRepoName(s.Remote),
		Inbox:      s.InboxTopic(),
		LastActive: s.LastInboxRead,
		State:      string(s.State),
		Unread:     0,
	}

	// Count unread messages if we have a last read timestamp
	if s.LastInboxRead != nil {
		messages, err := msgStore.Subscribe(ctx, s.InboxTopic(), *s.LastInboxRead)
		if err == nil {
			info.Unread = len(messages)
		}
		// Silently ignore errors (e.g., topic not found means no inbox yet)
	}

	return info
}
