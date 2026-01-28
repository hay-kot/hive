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

- View all active and recycled sessions
- Navigate with arrow keys or j/k
- Configurable keybindings for actions
- Confirmation dialogs for destructive actions

**Default keybindings:**

- `r` - Mark session as recycled (with confirmation)
- `d` - Delete session permanently (with confirmation)
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

Removes all recycled sessions and their directories.

**Behavior:**

- Only removes sessions with state `recycled`
- Deletes both the session record and the cloned repository directory
- Reports how many sessions were pruned

## Configuration

Config file: `$XDG_CONFIG_HOME/hive/config.yaml` (default: `~/.config/hive/config.yaml`)

```yaml
# Commands executed by hive
commands:
  # Spawn command runs after session creation
  # Available template variables: .Path, .Name, .Prompt
  spawn:
    - 'wezterm cli spawn --cwd "{{ .Path }}" -- claude --prompt "{{ .Prompt }}"'

  # Recycle commands run when reusing an existing session
  recycle:
    - git reset --hard
    - git checkout main
    - git pull

# Git executable (optional, defaults to "git")
git_path: git

# Rules for repository-specific setup
# Each rule can have commands (hooks) and/or copy patterns
# Pattern uses regex syntax matched against the remote URL
rules:
  - pattern: ".*/my-org/.*"
    commands:
      - npm install
      - npm run build
    copy:
      - .envrc
      - configs/*.yaml
  - pattern: ".*/hay-kot/.*"
    commands:
      - go mod download

# Keybindings for TUI actions
keybindings:
  # Built-in actions use the "action" property
  r:
    action: recycle # Mark session as recycled
    help: recycle
    confirm: Are you sure you want to recycle this session?
  d:
    action: delete # Delete session permanently
    help: delete
    confirm: Are you sure you want to delete this session?

  # Custom shell commands use the "sh" property
  # Available template variables: .Path, .Name, .Remote, .ID, .State
  o:
    help: open in finder
    sh: "open {{ .Path }}"
  O:
    help: open remote
    sh: "open {{ .Remote }}"
  ctrl+o:
    help: open in zed
    sh: "zed {{ .Path }}"
  ctrl+d:
    help: run custom script
    confirm: Are you sure? # Optional confirmation dialog
    sh: "my-script {{ .Path }}"
```

### Configuration Options

| Option             | Type                    | Default                                                 | Description                                                     |
| ------------------ | ----------------------- | ------------------------------------------------------- | --------------------------------------------------------------- |
| `commands.spawn`   | `[]string`              | `[]`                                                    | Commands to run after session creation (Go templates supported) |
| `commands.recycle` | `[]string`              | `["git reset --hard", "git checkout main", "git pull"]` | Commands to run when recycling a session                        |
| `git_path`         | `string`                | `git`                                                   | Path to git executable                                          |
| `rules`            | `[]Rule`                | `[]`                                                    | Repository-specific setup rules                                 |
| `keybindings`      | `map[string]Keybinding` | see below                                               | TUI keybinding configuration                                    |

### Rules

Rules run after cloning or recycling a session. Each rule has:

- `pattern`: Regex pattern matched against the remote URL (empty matches all)
- `commands`: Shell commands to execute in the session directory
- `copy`: Glob patterns for files to copy from the source directory

### Keybindings

Each keybinding can have:

- `action`: Built-in action (`recycle` or `delete`)
- `sh`: Shell command template (mutually exclusive with `action`)
- `help`: Help text shown in TUI status bar
- `confirm`: Confirmation prompt (optional, shows dialog before executing)

Default keybindings (`r` for recycle, `d` for delete) are provided and can be overridden.

## Data Storage

Session data and cloned repositories are stored at:

`$XDG_DATA_HOME/hive/` (default: `~/.local/share/hive/`)

```
~/.local/share/hive/
├── sessions.json          # Session state
└── repos/
    ├── myproject-feature1/
    ├── myproject-feature2/
    └── ...
```

## Session Lifecycle

1. **Active** - Session is in use, environment exists
2. **Recycled** - Marked for reuse, can be claimed by `hive new` or deleted by `hive prune`

## Dependencies

- Git (available in PATH or configured via `git_path`)
- Terminal emulator with CLI spawning support (e.g., wezterm, kitty, alacritty)
