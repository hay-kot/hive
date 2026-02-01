# Agent Instructions

## Project Overview

**Hive** is a CLI/TUI for managing multiple AI agent sessions in isolated git environments. Instead of manually managing worktrees, hive handles cloning, recycling, and spawning terminal sessions with your preferred AI tool.

Key capabilities:
- **Session Management** - Create, recycle, and prune isolated git clones
- **Terminal Integration** - Real-time status monitoring of AI agents in tmux
- **Inter-agent Messaging** - Pub/sub communication between sessions
- **Context Directories** - Shared storage per repository via `.hive` symlinks

## Architecture

```
internal/
├── commands/       # CLI command handlers (urfave/cli/v3)
├── core/
│   ├── config/     # Configuration loading, validation, defaults
│   ├── git/        # Git operations (clone, pull, status)
│   └── session/    # Session model and Store interface
├── hive/           # Service layer - orchestrates all operations
├── integration/
│   └── terminal/   # Terminal status monitoring (tmux)
├── store/
│   └── jsonfile/   # JSON file session storage implementation
├── tui/            # Bubble Tea TUI (tree view, modals, keybindings)
├── messaging/      # Pub/sub messaging between agents
├── printer/        # Output formatting utilities
└── styles/         # Shared lipgloss styles
```

### Key Files

| File | Purpose |
|------|---------|
| `main.go` | CLI entry point, global flags, command registration |
| `internal/hive/service.go` | Service layer - coordinates sessions, git, rules |
| `internal/core/config/config.go` | Config structs, loading, defaults |
| `internal/core/config/validate.go` | Template data structs, validation |
| `internal/tui/model.go` | TUI model, update loop, view rendering |
| `internal/tui/tree_view.go` | Session tree with status indicators |
| `internal/integration/terminal/detector.go` | AI agent status detection patterns |

## Development

### Commands

```bash
task run              # Run with dev config (supports CLI_ARGS)
task run -- new       # Example: run 'hive new'
task build            # Build with goreleaser
task test             # Run tests with gotestsum
task test:watch       # Watch mode
task lint             # Run golangci-lint
task check            # tidy + lint + test (full validation)
task coverage         # Generate coverage report
```

### Environment

Dev environment uses `config.dev.yaml` and `.data/` for isolation:

```bash
HIVE_LOG_LEVEL=debug
HIVE_LOG_FILE=./dev.log
HIVE_CONFIG=./config.dev.yaml
HIVE_DATA_DIR=./.data
```

## Code Patterns

### Bubble Tea (TUI)

Standard Model/Update/View pattern. Key messages:
- `sessionsLoadedMsg` - Sessions fetched from store
- `gitStatusBatchCompleteMsg` - Git status for all sessions
- `terminalPollTickMsg` - Terminal status polling tick
- `actionCompleteMsg` - Keybinding action finished

### Configuration

Two validation phases:
1. **Basic** (`Validate()`) - Struct validation, required fields
2. **Deep** (`ValidateDeep()`) - File access, template syntax, regex patterns

### Templates

Commands support Go templates with `shq` function for shell quoting:

```yaml
spawn:
  - my-script {{ .Name | shq }} {{ .Path | shq }}
```

Available variables vary by context - see `internal/core/config/validate.go` for `*TemplateData` structs.

### Session States

```
(new) ──► active ──► recycled ──► (deleted)
              │           │
              └──► corrupted ──► (deleted)
```

## Issue Tracking

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

