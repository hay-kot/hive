package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	ctxpkg "github.com/hay-kot/hive/internal/core/context"
	"github.com/hay-kot/hive/internal/core/git"
	"github.com/hay-kot/hive/internal/printer"
	"github.com/hay-kot/hive/internal/store/jsonfile"
	"github.com/urfave/cli/v3"
)

type CtxCmd struct {
	flags *Flags

	// Shared flags
	repo   string
	shared bool

	// kv list flag
	jsonOut bool

	// kv watch flag
	timeout string

	// prune flag
	olderThan string
}

// NewCtxCmd creates a new ctx command.
func NewCtxCmd(flags *Flags) *CtxCmd {
	return &CtxCmd{flags: flags}
}

// Register adds the ctx command to the application.
func (cmd *CtxCmd) Register(app *cli.Command) *cli.Command {
	app.Commands = append(app.Commands, &cli.Command{
		Name:  "ctx",
		Usage: "Manage context directories for inter-agent communication",
		Description: `Context commands manage shared directories and KV storage for agents.

Each repository gets its own context directory at $XDG_DATA_HOME/hive/context/{owner}/{repo}/.
Use 'hive ctx init' in a git repository to create a .hive symlink pointing to this directory.

The KV store provides typed key-value storage with timestamps for inter-agent communication.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "repo",
				Aliases:     []string{"r"},
				Usage:       "target a specific repository context (owner/repo)",
				Destination: &cmd.repo,
			},
			&cli.BoolFlag{
				Name:        "shared",
				Aliases:     []string{"s"},
				Usage:       "use the shared context directory",
				Destination: &cmd.shared,
			},
		},
		Commands: []*cli.Command{
			cmd.initCmd(),
			cmd.openCmd(),
			cmd.lsCmd(),
			cmd.pruneCmd(),
			cmd.kvCmd(),
		},
	})

	return app
}

func (cmd *CtxCmd) initCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Create symlink to context directory",
		Description: `Creates a symlink in the current directory pointing to the context directory.

The symlink name is configured via context.symlink_name (default: .hive).
The target is $XDG_DATA_HOME/hive/context/{owner}/{repo}/.`,
		Action: cmd.runInit,
	}
}

func (cmd *CtxCmd) openCmd() *cli.Command {
	return &cli.Command{
		Name:        "open",
		Usage:       "Open context directory in $EDITOR",
		Description: "Opens the context directory in your default editor.",
		Action:      cmd.runOpen,
	}
}

func (cmd *CtxCmd) lsCmd() *cli.Command {
	return &cli.Command{
		Name:        "ls",
		Usage:       "List files in context directory",
		Description: "Lists files in the context directory with size and modification time.",
		Action:      cmd.runLs,
	}
}

func (cmd *CtxCmd) pruneCmd() *cli.Command {
	return &cli.Command{
		Name:  "prune",
		Usage: "Delete old files from context directory",
		Description: `Deletes files older than the specified duration.

Example: hive ctx prune --older-than 7d`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "older-than",
				Usage:       "delete files older than this duration (e.g., 7d, 24h)",
				Destination: &cmd.olderThan,
				Required:    true,
			},
		},
		Action: cmd.runPrune,
	}
}

func (cmd *CtxCmd) kvCmd() *cli.Command {
	return &cli.Command{
		Name:        "kv",
		Usage:       "Key-value store operations",
		Description: "Manage key-value pairs for inter-agent communication.",
		Commands: []*cli.Command{
			{
				Name:      "get",
				Usage:     "Get a value by key",
				UsageText: "hive ctx kv get <key>",
				Action:    cmd.runKvGet,
			},
			{
				Name:      "set",
				Usage:     "Set a key-value pair",
				UsageText: "hive ctx kv set <key> [value]",
				Description: `Sets a key to a value. If value is not provided, reads from stdin.

Example:
  hive ctx kv set mykey "my value"
  echo "piped value" | hive ctx kv set mykey`,
				Action: cmd.runKvSet,
			},
			{
				Name:      "list",
				Usage:     "List all keys",
				UsageText: "hive ctx kv list [prefix]",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:        "json",
						Usage:       "output as JSON",
						Destination: &cmd.jsonOut,
					},
				},
				Action: cmd.runKvList,
			},
			{
				Name:      "delete",
				Usage:     "Delete a key",
				UsageText: "hive ctx kv delete <key>",
				Action:    cmd.runKvDelete,
			},
			{
				Name:      "watch",
				Usage:     "Watch for key updates",
				UsageText: "hive ctx kv watch <key>",
				Description: `Blocks until the key is updated, then prints the new value.

If the key doesn't exist, waits for it to be created.`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "timeout",
						Aliases:     []string{"t"},
						Usage:       "timeout duration (e.g., 30s, 5m)",
						Value:       "30s",
						Destination: &cmd.timeout,
					},
				},
				Action: cmd.runKvWatch,
			},
		},
	}
}

func (cmd *CtxCmd) runInit(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	ctxDir, err := cmd.resolveContextDir(ctx)
	if err != nil {
		return err
	}

	// Create the context directory if it doesn't exist
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return fmt.Errorf("create context directory: %w", err)
	}

	symlinkName := cmd.flags.Config.Context.SymlinkName
	symlinkPath := filepath.Join(".", symlinkName)

	// Check if symlink already exists
	if info, err := os.Lstat(symlinkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(symlinkPath)
			if target == ctxDir {
				p.Infof("Symlink already exists: %s -> %s", symlinkName, ctxDir)
				return nil
			}
			return fmt.Errorf("symlink %s exists but points to %s, not %s", symlinkName, target, ctxDir)
		}
		return fmt.Errorf("%s already exists and is not a symlink", symlinkName)
	}

	if err := os.Symlink(ctxDir, symlinkPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	p.Successf("Created symlink: %s -> %s", symlinkName, ctxDir)
	return nil
}

func (cmd *CtxCmd) runOpen(ctx context.Context, c *cli.Command) error {
	ctxDir, err := cmd.resolveContextDir(ctx)
	if err != nil {
		return err
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	execCmd := exec.CommandContext(ctx, editor, ctxDir)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}

func (cmd *CtxCmd) runLs(ctx context.Context, c *cli.Command) error {
	ctxDir, err := cmd.resolveContextDir(ctx)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		if os.IsNotExist(err) {
			printer.Ctx(ctx).Infof("Context directory does not exist: %s", ctxDir)
			return nil
		}
		return fmt.Errorf("read directory: %w", err)
	}

	if len(entries) == 0 {
		printer.Ctx(ctx).Infof("Context directory is empty")
		return nil
	}

	out := c.Root().Writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSIZE\tMODIFIED")

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		size := formatSize(info.Size())
		modified := info.ModTime().Format("2006-01-02 15:04")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name(), size, modified)
	}

	return w.Flush()
}

func (cmd *CtxCmd) runPrune(ctx context.Context, c *cli.Command) error {
	p := printer.Ctx(ctx)

	ctxDir, err := cmd.resolveContextDir(ctx)
	if err != nil {
		return err
	}

	duration, err := parseDuration(cmd.olderThan)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	cutoff := time.Now().Add(-duration)
	count := 0

	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		if os.IsNotExist(err) {
			p.Infof("Context directory does not exist")
			return nil
		}
		return fmt.Errorf("read directory: %w", err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(ctxDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				p.Warnf("Failed to remove %s: %v", entry.Name(), err)
				continue
			}
			count++
		}
	}

	p.Successf("Removed %d file(s) older than %s", count, cmd.olderThan)
	return nil
}

func (cmd *CtxCmd) runKvGet(ctx context.Context, c *cli.Command) error {
	if c.NArg() < 1 {
		return fmt.Errorf("key argument required")
	}
	key := c.Args().Get(0)

	store, err := cmd.getKVStore(ctx)
	if err != nil {
		return err
	}

	entry, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ctxpkg.ErrKeyNotFound) {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("get key: %w", err)
	}

	_, _ = fmt.Fprintln(c.Root().Writer, entry.Value)
	return nil
}

func (cmd *CtxCmd) runKvSet(ctx context.Context, c *cli.Command) error {
	if c.NArg() < 1 {
		return fmt.Errorf("key argument required")
	}
	key := c.Args().Get(0)

	var value string
	if c.NArg() >= 2 {
		value = c.Args().Get(1)
	} else {
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		value = strings.TrimSuffix(string(data), "\n")
	}

	store, err := cmd.getKVStore(ctx)
	if err != nil {
		return err
	}

	if err := store.Set(ctx, key, value); err != nil {
		return fmt.Errorf("set key: %w", err)
	}

	return nil
}

func (cmd *CtxCmd) runKvList(ctx context.Context, c *cli.Command) error {
	prefix := ""
	if c.NArg() >= 1 {
		prefix = c.Args().Get(0)
	}

	store, err := cmd.getKVStore(ctx)
	if err != nil {
		return err
	}

	entries, err := store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list keys: %w", err)
	}

	if len(entries) == 0 {
		if !cmd.jsonOut {
			printer.Ctx(ctx).Infof("No keys found")
		} else {
			_, _ = fmt.Fprintln(c.Root().Writer, "[]")
		}
		return nil
	}

	if cmd.jsonOut {
		enc := json.NewEncoder(c.Root().Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	out := c.Root().Writer
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tUPDATED")

	for _, entry := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", entry.Key, entry.UpdatedAt.Format("2006-01-02 15:04:05"))
	}

	return w.Flush()
}

func (cmd *CtxCmd) runKvDelete(ctx context.Context, c *cli.Command) error {
	if c.NArg() < 1 {
		return fmt.Errorf("key argument required")
	}
	key := c.Args().Get(0)

	store, err := cmd.getKVStore(ctx)
	if err != nil {
		return err
	}

	if err := store.Delete(ctx, key); err != nil {
		if errors.Is(err, ctxpkg.ErrKeyNotFound) {
			return fmt.Errorf("key not found: %s", key)
		}
		return fmt.Errorf("delete key: %w", err)
	}

	return nil
}

func (cmd *CtxCmd) runKvWatch(ctx context.Context, c *cli.Command) error {
	if c.NArg() < 1 {
		return fmt.Errorf("key argument required")
	}
	key := c.Args().Get(0)

	timeout, err := time.ParseDuration(cmd.timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	store, err := cmd.getKVStore(ctx)
	if err != nil {
		return err
	}

	// Get current state to determine "after" time
	after := time.Now()
	if entry, err := store.Get(ctx, key); err == nil {
		after = entry.UpdatedAt
	}

	entry, err := store.Watch(ctx, key, after, timeout)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timeout waiting for key update")
		}
		return fmt.Errorf("watch key: %w", err)
	}

	_, _ = fmt.Fprintln(c.Root().Writer, entry.Value)
	return nil
}

func (cmd *CtxCmd) resolveContextDir(ctx context.Context) (string, error) {
	if cmd.shared {
		return cmd.flags.Config.SharedContextDir(), nil
	}

	if cmd.repo != "" {
		parts := strings.SplitN(cmd.repo, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid repo format, expected owner/repo: %s", cmd.repo)
		}
		return cmd.flags.Config.RepoContextDir(parts[0], parts[1]), nil
	}

	// Detect from current directory
	remote, err := cmd.flags.Service.DetectRemote(ctx, ".")
	if err != nil {
		return "", fmt.Errorf("detect remote (are you in a git repository?): %w", err)
	}

	owner, repo := git.ExtractOwnerRepo(remote)
	if owner == "" || repo == "" {
		return "", fmt.Errorf("could not extract owner/repo from remote: %s", remote)
	}

	return cmd.flags.Config.RepoContextDir(owner, repo), nil
}

func (cmd *CtxCmd) getKVStore(ctx context.Context) (*jsonfile.KVStore, error) {
	ctxDir, err := cmd.resolveContextDir(ctx)
	if err != nil {
		return nil, err
	}

	// Ensure directory exists
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return nil, fmt.Errorf("create context directory: %w", err)
	}

	kvPath := filepath.Join(ctxDir, "kv.json")
	return jsonfile.NewKVStore(kvPath), nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func parseDuration(s string) (time.Duration, error) {
	// Handle day suffix (not supported by time.ParseDuration)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
