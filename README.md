# hive

A CLI/TUI for managing multiple AI agent sessions as an alternative to git worktrees.

## Installation

```bash
go install github.com/hay-kot/hive@latest
```

## Overview

Hive creates isolated git environments for running multiple AI agents in parallel. Instead of managing worktrees manually, hive handles cloning, recycling, and spawning terminal sessions with your preferred AI tool.

## Usage

### `hive` (default)

Launches the TUI for managing sessions. Features:

- View all active and recycled sessions
- Configurable keybindings for actions (shell commands, open remote, trigger AI, mark recycled)
- Confirmation dialogs for destructive actions

### `hive new`

Creates a new agent session:

1. Prompts for session name and AI prompt
2. Either recycles an existing environment (clean tree, checkout main, pull) or clones fresh
3. Spawns a terminal with your configured command (supports Go templates)

### `hive ls`

Lists all sessions in a formatted table.

### `hive prune`

Removes recycled environments that are no longer in use.

## Configuration

Config file: `$XDG_CONFIG_HOME/hive/config.yaml` (default: `~/.config/hive/config.yaml`)

```yaml
# Command to spawn a new terminal session
# Available template variables: .Path, .Name, .Prompt
commands:
  spawn: # configurable spawn command
    - 'wezterm cli spawn --cwd "{{ .Path }}" -- claude --prompt "{{ .Prompt }}"'
  recycle: # configurable recycle command
    - git reset --hard
    - git checkout main
    - git pull

# Git executable (optional, defaults to "git")
git_path: git

# Keybindings for TUI actions
keybindings:
  # action: is a special property that allows the user to use more complex logic that we
  # have in the application
  r:
    action: recycle # Mark session as recycled
  d:
    action: delete # Delete session permanently
  o:
    help: open in finder
    sh: "open {{ .Path }}"
  O:
    help: open in finder
    sh: "open {{ .Remote }}"
  ctrl-o:
    help: open in zed
    sh: "zed {{ .Path }}"
  ctrl-d:
    help: trigger long running process
    confirm: Are you sure you want to do this scary thing? # when confirm is provided trigger a confirmation dialog before executing
    sh: "..." # Open git remote in browser
```

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
