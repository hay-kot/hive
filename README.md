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
| `--template` | `-t` | Use a session template (see Templates section) |
| `--set` | - | Set template field value (name=value), use commas for multi-select |

**Behavior:**

1. If `--template` is provided, uses the template to generate the prompt
2. If `--name` is not provided, shows an interactive form prompting for session name and AI prompt
3. Checks for a recyclable session with the same remote
4. If recyclable found: runs recycle commands (reset, checkout main, pull)
5. If no recyclable: clones the repository fresh
6. Runs any configured hooks matching the remote
7. Saves session to the store
8. Spawns a terminal with the configured spawn command

**Examples:**

```bash
# Interactive mode - prompts for name and prompt
hive new

# Non-interactive with all options
hive new --name feature-auth --prompt "Implement OAuth2 login flow"

# Auto-detect remote from current directory
hive new -n bugfix-123

# Using a template (interactive form for fields)
hive new --template pr-review

# Using a template with --set flags (non-interactive)
hive new --template pr-review --set pr_number=123 --set focus_areas=security,performance
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

#### `hive templates list`

Lists all available templates defined in your config file.

**Output columns:** NAME, DESCRIPTION, FIELDS

#### `hive templates show <name>`

Displays detailed information about a specific template including all fields and the prompt template.

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

# Repository-specific setup hooks
# Pattern uses glob syntax (supports ** for path matching)
hooks:
  - pattern: "**/my-org/**"
    commands:
      - npm install
      - npm run build
  - pattern: "**github.com/hay-kot/**"
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

# Session templates for common workflows
templates:
  pr-review:
    description: "Review a GitHub pull request"
    name: "pr-{{ .pr_number }}"  # Optional session name template
    fields:
      - name: pr_number
        label: "PR Number"
        type: string
        required: true
        placeholder: "e.g., 523"
      - name: review_depth
        label: "Review Depth"
        type: select
        default: "standard"
        options:
          - value: quick
            label: "Quick (surface-level)"
          - value: standard
            label: "Standard"
          - value: thorough
            label: "Thorough (deep dive)"
      - name: focus_areas
        label: "Focus Areas"
        type: multi-select
        options:
          - value: security
            label: "Security vulnerabilities"
          - value: performance
            label: "Performance issues"
          - value: style
            label: "Code style"
      - name: context
        label: "Additional Context"
        type: text
        required: false
    prompt: |
      Review PR #{{ .pr_number }} with {{ .review_depth }} depth.
      {{ if .focus_areas }}
      Focus on: {{ .focus_areas | join ", " }}
      {{ end }}
      {{ if .context }}
      Additional context: {{ .context }}
      {{ end }}
```

### Configuration Options

| Option             | Type                    | Default                                                 | Description                                                     |
| ------------------ | ----------------------- | ------------------------------------------------------- | --------------------------------------------------------------- |
| `commands.spawn`   | `[]string`              | `[]`                                                    | Commands to run after session creation (Go templates supported) |
| `commands.recycle` | `[]string`              | `["git reset --hard", "git checkout main", "git pull"]` | Commands to run when recycling a session                        |
| `git_path`         | `string`                | `git`                                                   | Path to git executable                                          |
| `hooks`            | `[]Hook`                | `[]`                                                    | Repository-specific setup commands                              |
| `keybindings`      | `map[string]Keybinding` | see below                                               | TUI keybinding configuration                                    |
| `templates`        | `map[string]Template`   | `{}`                                                    | Session templates for common workflows                          |

### Hooks

Hooks run after cloning or recycling a session. Each hook has:

- `pattern`: Glob pattern matched against the remote URL (supports `**` for path matching)
- `commands`: Shell commands to execute in the session directory

### Keybindings

Each keybinding can have:

- `action`: Built-in action (`recycle` or `delete`)
- `sh`: Shell command template (mutually exclusive with `action`)
- `help`: Help text shown in TUI status bar
- `confirm`: Confirmation prompt (optional, shows dialog before executing)

Default keybindings (`r` for recycle, `d` for delete) are provided and can be overridden.

### Templates

Templates define reusable session configurations with interactive forms. Each template has:

- `description`: Brief description of what the template is for
- `name`: Optional Go template for generating the session name
- `prompt`: Go template for generating the AI prompt (required)
- `fields`: List of form fields to collect user input

**Field Types:**

| Type | Description | Form Element |
|------|-------------|--------------|
| `string` | Single-line text input | Text input |
| `text` | Multi-line text input | Text area |
| `select` | Single choice from options | Dropdown/radio |
| `multi-select` | Multiple choices from options | Checkboxes |

**Field Properties:**

| Property | Type | Description |
|----------|------|-------------|
| `name` | `string` | Variable name used in templates (required) |
| `label` | `string` | Display label in form |
| `type` | `string` | Field type (see above) |
| `required` | `bool` | Whether field must have a value |
| `default` | `string` | Default value |
| `placeholder` | `string` | Placeholder text for input fields |
| `options` | `[]Option` | Options for select/multi-select |

**Template Functions:**

| Function | Description | Example |
|----------|-------------|---------|
| `join` | Join slice with separator | `{{ .items \| join ", " }}` |
| `default` | Provide default if empty | `{{ .opt \| default "none" }}` |
| `shq` | Shell-quote a string | `{{ .msg \| shq }}` |

**Usage:**

```bash
# Interactive - shows form for all fields
hive new --template pr-review

# Non-interactive - set field values directly
hive new --template pr-review --set pr_number=123 --set focus_areas=security,performance
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
