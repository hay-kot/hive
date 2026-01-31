package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/core/messaging"
	"github.com/hay-kot/hive/internal/core/session"
	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/hay-kot/hive/pkg/executil"
	"github.com/urfave/cli/v3"
)

type SessionCmd struct {
	flags *Flags
}

// NewSessionCmd creates a new session command.
func NewSessionCmd(flags *Flags) *SessionCmd {
	return &SessionCmd{flags: flags}
}

// Register adds the session command to the application.
func (cmd *SessionCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "session",
		Usage: "Session information and management",
		Description: `Session commands for getting information about the current hive session.

Use these commands when you need to identify which session you're running in,
get session metadata for inter-agent communication, or inspect session state.`,
		Commands: []*cli.Command{
			cmd.infoCmd(),
		},
	})

	return app
}

// SessionInfo represents the output of the session info command.
type SessionInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Path   string `json:"path"`
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	State  string `json:"state"`
}

func (cmd *SessionCmd) infoCmd() *cli.Command {
	return &cli.Command{
		Name:      "info",
		Usage:     "Display current session information",
		UsageText: "hive session info",
		Description: `Outputs information about the current hive session as JSON.

The session is detected from the current working directory. If not running
within a hive session, returns an error.

This command is useful for:
- LLM agents to identify themselves in inter-agent communication
- Scripts that need session metadata
- Debugging session state

Output fields:
  id      - Unique session identifier
  name    - Human-readable session name
  slug    - URL-safe session slug
  path    - Filesystem path to the session
  remote  - Git remote URL
  branch  - Current git branch
  state   - Session state (active, recycled, corrupted)`,
		Action: cmd.runInfo,
	}
}

func (cmd *SessionCmd) runInfo(ctx context.Context, c *cli.Command) error {
	store := cmd.getSessionStore()

	// Detect session from current directory
	detector := messaging.NewSessionDetector(store)
	sessionID, err := detector.DetectSession(ctx)
	if err != nil {
		return fmt.Errorf("detect session: %w", err)
	}
	if sessionID == "" {
		return fmt.Errorf("not running within a hive session")
	}

	// Get full session details
	sess, err := store.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	// Get current git branch
	gitExec := git.NewExecutor(cmd.flags.Config.GitPath, &executil.RealExecutor{})
	branch, err := gitExec.Branch(ctx, sess.Path)
	if err != nil {
		branch = "" // Non-fatal, just leave empty
	}

	info := SessionInfo{
		ID:     sess.ID,
		Name:   sess.Name,
		Slug:   sess.Slug,
		Path:   sess.Path,
		Remote: sess.Remote,
		Branch: branch,
		State:  string(sess.State),
	}

	enc := json.NewEncoder(c.Root().Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(info)
}

func (cmd *SessionCmd) getSessionStore() session.Store {
	sessionsPath := filepath.Join(cmd.flags.DataDir, "sessions.json")
	return jsonfile.New(sessionsPath)
}
