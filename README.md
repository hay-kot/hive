# hive

A CLI/TUI for managing multiple AI agent sessions as an alternative to git worktrees.

## Installation

```bash
go install github.com/hay-kot/hive@latest
```

## Overview

Hive creates isolated git environments for running multiple AI agents in parallel. Instead of managing worktrees manually, hive handles cloning, recycling, and spawning terminal sessions with your preferred AI tool.

## Usage

```
hive [global options] command [command options]
```

### Global Flags

| Flag           | Env Variable     | Default                      | Description                                        |
| -------------- | ---------------- | ---------------------------- | -------------------------------------------------- |
| `--log-level`  | `HIVE_LOG_LEVEL` | `info`                       | Log level (debug, info, warn, error, fatal, panic) |
| `--log-file`   | `HIVE_LOG_FILE`  | -                            | Path to log file (optional)                        |
| `--config, -c` | `HIVE_CONFIG`    | `~/.config/hive/config.yaml` | Path to config file                                |
| `--data-dir`   | `HIVE_DATA_DIR`  | `~/.local/share/hive`        | Path to data directory                             |

### Commands

#### `hive` (default)

Launches the interactive TUI for managing sessions.

**Features:**

- View all active and recycled sessions in a tree view grouped by repository
- Real-time terminal status monitoring (when tmux integration enabled)
- Git status display (branch, additions, deletions)
- Navigate with arrow keys or j/k
- Filter sessions with `/`
- Switch between Sessions and Messages views with `tab`
- Configurable keybindings for actions
- Confirmation dialogs for destructive actions

**Status Indicators** (with terminal integration):

| Indicator | Color | Meaning |
|-----------|-------|---------|
| `[●]` | Green (animated) | Agent actively working |
| `[!]` | Yellow | Agent needs approval/permission |
| `[>]` | Cyan | Agent ready for input |
| `[?]` | Dim | Terminal session not found |
| `[○]` | Gray | Session recycled |

**Default keybindings:**

- `r` - Mark session as recycled (with confirmation)
- `d` - Delete session permanently (with confirmation)
- `n` - Create new session (when repos discovered)
- `g` - Refresh git statuses
- `tab` - Switch between Sessions/Messages views
- `q` / `Ctrl+C` - Quit

#### `hive new`

Creates a new agent session.

**Flags:**
| Flag | Alias | Description |
|------|-------|-------------|
| `--name` | `-n` | Session name (used in directory path) |
| `--remote` | `-r` | Git remote URL (auto-detected from current directory if not specified) |
| `--prompt` | `-p` | AI prompt to pass to spawn command |

**Behavior:**

1. If `--name` is not provided, shows an interactive form prompting for session name and AI prompt
2. Checks for a recyclable session with the same remote
3. If recyclable found: runs recycle commands (reset, checkout main, pull)
4. If no recyclable: clones the repository fresh
5. Runs any configured hooks matching the remote
6. Saves session to the store
7. Spawns a terminal with the configured spawn command

**Examples:**

```bash
# Interactive mode - prompts for name and prompt
hive new

# Non-interactive with all options
hive new --name feature-auth --prompt "Implement OAuth2 login flow"

# Auto-detect remote from current directory
hive new -n bugfix-123
```

#### `hive tui`

Explicitly launches the interactive session manager (same as running `hive` with no command).

#### `hive ls`

Lists all sessions in a formatted table.

**Output columns:** ID, NAME, STATE, PATH

**Example:**

```
ID      NAME           STATE     PATH
abc123  feature-auth   active    ~/.local/share/hive/repos/myapp-feature-auth-abc123
def456  old-feature    recycled  ~/.local/share/hive/repos/myapp-old-feature-def456
```

#### `hive prune`

Removes recycled sessions exceeding the `max_recycled` limit.

**Flags:**
| Flag | Alias | Description |
|------|-------|-------------|
| `--all` | `-a` | Delete all recycled sessions (ignore max_recycled limit) |

**Behavior:**

- By default, keeps the newest N recycled sessions per repository (based on `max_recycled` config)
- With `--all`: deletes all recycled sessions regardless of limit
- Always deletes corrupted sessions
- Reports how many sessions were pruned

**Examples:**

```bash
# Clean up sessions exceeding max_recycled limit
hive prune

# Delete ALL recycled sessions
hive prune --all
```

#### `hive doctor`

Runs diagnostic checks on configuration, environment, and dependencies.

**Flags:**
| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format (`text` or `json`) |

#### `hive batch`

Creates multiple agent sessions from a JSON specification. Useful for spawning parallel agents.

**Flags:**
| Flag | Alias | Description |
|------|-------|-------------|
| `--file` | `-f` | Path to JSON file (reads from stdin if not provided) |

**Input Schema:**
```json
{
  "sessions": [
    {
      "name": "session-name",
      "prompt": "optional task prompt",
      "remote": "optional git URL",
      "source": "optional source path"
    }
  ]
}
```

**Examples:**

```bash
# From stdin
echo '{"sessions":[{"name":"task1","prompt":"Fix the auth bug"}]}' | hive batch

# From file
hive batch -f sessions.json
```

#### `hive ctx`

Manages context directories for sharing files between sessions of the same repository.

##### `hive ctx init`

Creates a symlink (`.hive` by default) in the current directory pointing to the repository's context directory.

```bash
cd /path/to/session
hive ctx init
# Creates .hive -> ~/.local/share/hive/context/{owner}/{repo}/
```

##### `hive ctx prune`

Deletes files older than the specified duration from the context directory.

**Flags:**
| Flag | Required | Description |
|------|----------|-------------|
| `--older-than` | Yes | Duration (e.g., `7d`, `24h`, `1w`) |

```bash
hive ctx prune --older-than 7d
```

#### `hive msg`

Publish and subscribe to messages for inter-agent communication.

##### `hive msg pub`

Publishes a message to a topic.

**Flags:**
| Flag | Alias | Required | Description |
|------|-------|----------|-------------|
| `--topic` | `-t` | Yes | Topic to publish to |
| `--file` | `-f` | No | Read message from file |
| `--sender` | `-s` | No | Override sender ID |

```bash
hive msg pub -t build.status "Build completed"
echo "Hello" | hive msg pub -t greetings
```

##### `hive msg sub`

Reads messages from topics.

**Flags:**
| Flag | Alias | Default | Description |
|------|-------|---------|-------------|
| `--topic` | `-t` | `*` | Topic pattern (supports wildcards like `agent.*`) |
| `--last` | `-n` | - | Return only last N messages |
| `--listen` | `-l` | false | Poll for new messages continuously |
| `--wait` | `-w` | false | Wait for a single message and exit |
| `--new` | - | false | Only unread messages (for inbox topics) |
| `--timeout` | - | `30s` | Timeout for listen/wait mode |

```bash
hive msg sub                          # All messages
hive msg sub -t agent.build           # Specific topic
hive msg sub -t "agent.*" --last 10   # Wildcard, last 10
hive msg sub --listen                 # Continuous polling
```

##### `hive msg list`

Lists all topics with message counts.

##### `hive msg topic`

Generates a unique topic ID for inter-agent communication.

**Flags:**
| Flag | Alias | Description |
|------|-------|-------------|
| `--prefix` | `-p` | Topic prefix (default from config) |

```bash
hive msg topic           # outputs: agent.x7k2
hive msg topic -p task   # outputs: task.x7k2
```

#### `hive session`

Commands for inspecting sessions.

##### `hive session info`

Displays information about the current session based on working directory.

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

```bash
hive session info --json
# {"id":"abc123","name":"my-session","inbox":"agent.abc123.inbox",...}
```

#### `hive doc`

Access documentation and guides.

##### `hive doc migrate`

Shows configuration migration information between versions.

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Show all migrations |

##### `hive doc messaging`

Outputs messaging conventions documentation for LLMs.

## Configuration

Config file: `$XDG_CONFIG_HOME/hive/config.yaml` (default: `~/.config/hive/config.yaml`)

```yaml
# Directories to scan for repositories (enables 'n' key in TUI)
repo_dirs:
  - ~/code/repos

# Terminal integration for real-time agent status
integrations:
  terminal:
    enabled: [tmux]
    poll_interval: 500ms

# Commands executed by hive
commands:
  # Spawn command runs after session creation (see Template Variables below)
  spawn:
    - 'wezterm cli spawn --cwd "{{ .Path }}" -- claude'

  # Batch spawn command - same as spawn but also has .Prompt
  batch_spawn:
    - 'wezterm cli spawn --cwd "{{ .Path }}" -- claude "{{ .Prompt }}"'

  # Recycle commands run when reusing an existing session
  recycle:
    - git fetch origin
    - git checkout {{ .DefaultBranch }}
    - git reset --hard origin/{{ .DefaultBranch }}
    - git clean -fd

# Git executable (optional, defaults to "git")
git_path: git

# Rules for repository-specific setup
rules:
  # Catch-all rule sets the default max_recycled
  - pattern: ""
    max_recycled: 5
    commands:
      - hive ctx init  # Create .hive symlink

  - pattern: ".*/my-org/.*"
    commands:
      - npm install
    copy:
      - .envrc
      - configs/*.yaml

  # Override max_recycled for large repos
  - pattern: ".*/my-org/large-repo"
    max_recycled: 2

# TUI settings
tui:
  refresh_interval: 15s

# Keybindings for TUI actions
keybindings:
  r:
    action: recycle
    help: recycle
    confirm: Are you sure you want to recycle this session?
  d:
    action: delete
    help: delete
    confirm: Are you sure you want to delete this session?

  # Custom shell commands (see Template Variables below)
  o:
    help: open in finder
    sh: "open {{ .Path }}"
    silent: true
  O:
    help: open remote
    sh: "open {{ .Remote }}"
```

### Template Variables

Commands support Go templates. Use `{{ .Variable }}` syntax, and `{{ .Variable | shq }}` for shell-safe quoting.

**Spawn commands** (`commands.spawn`):

| Variable | Description |
|----------|-------------|
| `.Path` | Absolute path to session directory |
| `.Name` | Session name |
| `.Slug` | URL-safe session name |
| `.ContextDir` | Path to context directory |
| `.Owner` | Repository owner |
| `.Repo` | Repository name |

**Batch spawn commands** (`commands.batch_spawn`): Same as spawn, plus:

| Variable | Description |
|----------|-------------|
| `.Prompt` | User-provided prompt |

**Recycle commands** (`commands.recycle`):

| Variable | Description |
|----------|-------------|
| `.DefaultBranch` | Default branch name (e.g., `main`) |

**Keybinding commands** (`keybindings.*.sh`):

| Variable | Description |
|----------|-------------|
| `.Path` | Absolute path to session directory |
| `.Name` | Session name |
| `.Remote` | Git remote URL |
| `.ID` | Session ID |

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `commands.spawn` | `[]string` | `[]` | Commands after session creation |
| `commands.batch_spawn` | `[]string` | `[]` | Commands after batch session creation (has `.Prompt`) |
| `commands.recycle` | `[]string` | git fetch/checkout/reset/clean | Commands when recycling a session |
| `git_path` | `string` | `git` | Path to git executable |
| `repo_dirs` | `[]string` | `[]` | Directories to scan for repositories (enables `n` key in TUI) |
| `rules` | `[]Rule` | `[]` | Repository-specific setup rules |
| `keybindings` | `map[string]Keybinding` | see below | TUI keybinding configuration |
| `tui.refresh_interval` | `duration` | `15s` | Auto-refresh interval (0 to disable) |
| `integrations.terminal.enabled` | `[]string` | `[]` | Terminal integrations to enable (e.g., `["tmux"]`) |
| `integrations.terminal.poll_interval` | `duration` | `500ms` | How often to check terminal status |
| `messaging.topic_prefix` | `string` | `agent` | Default prefix for generated topic IDs |
| `context.symlink_name` | `string` | `.hive` | Name of symlink created by `hive ctx init` |

### Rules

Rules run after cloning or recycling a session. Each rule has:

- `pattern`: Regex pattern matched against the remote URL (empty = catch-all)
- `commands`: Shell commands to execute in the session directory
- `copy`: Glob patterns for files to copy from the source directory
- `max_recycled`: Max recycled sessions for matching repos (0 = unlimited)

### Max Recycled Sessions

The `max_recycled` rule setting controls how many recycled sessions to keep per repository. When a session is recycled and the limit is exceeded, the oldest recycled sessions are automatically deleted.

**Behavior:**
- Rules are processed in order; last matching rule with `max_recycled` set wins
- Use an empty pattern (`""`) as a catch-all to set the default
- If no rule sets `max_recycled`, the code default is 5
- `0` means unlimited (no automatic deletion)

**Example:**
```yaml
rules:
  # Catch-all sets the default for all repos
  - pattern: ""
    max_recycled: 5

  # Large repos: keep fewer sessions
  - pattern: "github.com/my-org/large-repo"
    max_recycled: 2

  # Explicitly unlimited for specific repos
  - pattern: "github.com/my-org/unlimited-repo"
    max_recycled: 0
```

### Keybindings

Each keybinding can have:

- `action`: Built-in action (`recycle` or `delete`)
- `sh`: Shell command template (mutually exclusive with `action`)
- `help`: Help text shown in TUI status bar
- `confirm`: Confirmation prompt (optional, shows dialog before executing)
- `silent`: Skip loading indicator for fast commands (default: false)
- `exit`: Exit hive after command completes (see below)

Default keybindings (`r` for recycle, `d` for delete) are provided and can be overridden.

#### Conditional Exit

The `exit` field controls whether hive exits after executing a keybinding. This is useful for popup/ephemeral terminal scenarios (e.g., tmux popup).

```yaml
keybindings:
  # Static exit - always exit after this command
  enter:
    sh: "wezterm cli spawn --cwd {{ .Path }}"
    exit: true

  # Conditional exit - exit only when HIVE_POPUP=true
  enter:
    sh: "wezterm cli spawn --cwd {{ .Path }}"
    exit: $HIVE_POPUP
```

The `exit` field accepts:
- Boolean values: `true`, `false`, `1`, `0`
- Environment variable reference: `$VAR_NAME` - exits if the var is set to a truthy value

This allows the same config to work in both persistent and popup modes:

```bash
# Persistent session - select project, stay in hive
hive

# Popup mode - select project, exit so popup closes
HIVE_POPUP=true hive
```

## Data Storage

All data is stored at `$XDG_DATA_HOME/hive/` (default: `~/.local/share/hive/`):

```
~/.local/share/hive/
├── sessions.json              # Session state
├── repos/                     # Cloned repositories
│   ├── myproject-feature1-abc123/
│   └── myproject-feature2-def456/
├── context/                   # Per-repo context directories
│   ├── {owner}/{repo}/        # Linked via .hive symlink
│   └── shared/                # Shared context
└── messages/
    └── topics/                # Pub/sub message storage
        ├── agent.abc123.inbox.json
        └── build.status.json
```

## Session Lifecycle

1. **Active** - Session is in use, environment exists
2. **Recycled** - Marked for reuse, can be claimed by `hive new` or deleted by `hive prune`

## Acknowledgments

This project was heavily inspired by [agent-deck](https://github.com/asheshgoplani/agent-deck) by Ashesh Goplani. Several concepts and code patterns were adapted from their work. Thanks to the agent-deck team for open-sourcing their project under the MIT license.

## Dependencies

- Git (available in PATH or configured via `git_path`)
- Terminal emulator with CLI spawning support (e.g., wezterm, kitty, alacritty, tmux)

## Tmux Integration

Hive works well with tmux for managing AI agent sessions. Here's a complete example setup.

### Example Configuration

```yaml
version: 0.2.3
repo_dirs:
  - ~/code/repos

integrations:
  terminal:
    enabled: [tmux]
    poll_interval: 500ms

tui:
  refresh_interval: 15s

commands:
  spawn:
    - ~/.config/tmux/layouts/hive.sh "{{ .Name }}" "{{ .Path }}"
  batch_spawn:
    - ~/.config/tmux/layouts/hive.sh -b "{{ .Name }}" "{{ .Path }}" "{{ .Prompt }}"

rules:
  - pattern: ""
    max_recycled: 3
    commands:
      - hive ctx init

keybindings:
  enter:
    help: open/create tmux
    sh: ~/.config/tmux/layouts/hive.sh "{{ .Name }}" "{{ .Path }}"
    exit: $HIVE_POPUP
    silent: true
  p:
    help: popup
    sh: tmux display-popup -E -w 80% -h 80% "tmux new-session -s hive-popup -t '{{ .Name }}'"
    silent: true
  ctrl+d:
    help: kill session
    sh: tmux kill-session -t "{{ .Name }}" 2>/dev/null || true
  t:
    help: send /tidy
    sh: claude-send "{{ .Name }}:claude" "/tidy"
    silent: true
```

### Helper Script: hive.sh

Creates a tmux session with two windows: `claude` (running the AI) and `shell`.

```bash
#!/bin/bash
# Usage: hive.sh [-b] [session-name] [working-dir] [prompt]
#   -b: background mode (create session without attaching)

BACKGROUND=false
if [ "$1" = "-b" ]; then
    BACKGROUND=true
    shift
fi

SESSION="${1:-hive}"
WORKDIR="${2:-$PWD}"
PROMPT="${3:-}"

if [ -n "$PROMPT" ]; then
    CLAUDE_CMD="claude '$PROMPT'"
else
    CLAUDE_CMD="claude"
fi

if tmux has-session -t "$SESSION" 2>/dev/null; then
    [ "$BACKGROUND" = true ] && exit 0
    if [ -n "$TMUX" ]; then
        tmux switch-client -t "$SESSION"
    else
        tmux attach-session -t "$SESSION"
    fi
else
    tmux new-session -d -s "$SESSION" -n claude -c "$WORKDIR" "$CLAUDE_CMD"
    tmux new-window -t "$SESSION" -n shell -c "$WORKDIR"
    tmux select-window -t "$SESSION:claude"

    [ "$BACKGROUND" = true ] && exit 0
    if [ -n "$TMUX" ]; then
        tmux switch-client -t "$SESSION"
    else
        tmux attach-session -t "$SESSION"
    fi
fi
```

### Helper Script: claude-send

Sends text to a Claude session in tmux (useful for remote commands like `/tidy`).

```bash
#!/bin/bash
# Usage: claude-send <target> <text>
TARGET="${1:?Usage: claude-send <target> <text>}"
TEXT="${2:?Usage: claude-send <target> <text>}"

tmux send-keys -t "$TARGET" "$TEXT"
sleep 0.5
tmux send-keys -t "$TARGET" C-m
```

### Tmux Config Additions

```bash
# Quick access to hive TUI as popup (prefix + Space)
bind Space display-popup -E -w 85% -h 85% "HIVE_POPUP=true hive"

# Quick switch to hive session
bind l switch-client -t hive
```

### Quick Alias

```bash
# Start or attach to a persistent hive session
alias hv="tmux new-session -As hive hive"
```
