# Architecture

This document outlines the package structure, component interactions, and testing strategy for hive.

## Package Structure

```
hive/
├── main.go                     # Entry point, CLI setup
├── internal/
│   ├── commands/               # CLI command handlers (thin layer)
│   │   ├── cmd_new.go
│   │   ├── cmd_ls.go
│   │   ├── cmd_tui.go
│   │   └── cmd_prune.go
│   ├── core/
│   │   ├── config/             # Configuration loading and validation
│   │   │   └── config.go
│   │   ├── session/            # Session domain types and logic
│   │   │   ├── session.go      # Session struct, states, lifecycle
│   │   │   └── store.go        # SessionStore interface
│   │   └── git/                # Git operations abstraction
│   │       ├── git.go          # Git interface
│   │       └── executor.go     # Real git implementation
│   ├── hive/
│   │   ├── service.go          # Main business logic orchestrator
│   │   ├── spawner.go          # Terminal spawning logic
│   │   └── recycler.go         # Environment recycling logic
│   ├── store/
│   │   └── jsonfile/           # JSON file-based session storage
│   │       └── store.go
│   └── tui/                    # Bubble Tea TUI components
│       ├── app.go
│       ├── model.go
│       └── views/
├── pkg/
│   ├── executil/               # Shell execution utilities
│   │   └── exec.go
│   └── tmpl/                   # Template rendering utilities
│       └── tmpl.go
└── test/
    ├── integration/            # Integration tests (run in container)
    │   ├── Dockerfile
    │   └── *_test.go
    └── testutil/               # Shared test helpers
        └── helpers.go
```

## Layer Responsibilities

### `internal/commands/` - CLI Layer

Thin handlers that parse flags, construct dependencies, and delegate to business logic.

```go
// Commands receive dependencies, not create them
type NewCmd struct {
    service *hive.Service
}

func (c *NewCmd) Run(ctx context.Context) error {
    return c.service.CreateSession(ctx, c.name, c.prompt)
}
```

### `internal/core/` - Domain Types and Interfaces

Pure domain logic with no external dependencies. Defines interfaces that other packages implement.

**`core/session`** - Session domain:
```go
type State string

const (
    StateActive   State = "active"
    StateRecycled State = "recycled"
)

type Session struct {
    ID        string
    Name      string
    Path      string
    Remote    string
    State     State
    CreatedAt time.Time
    UpdatedAt time.Time
}

// Store defines persistence operations
type Store interface {
    List(ctx context.Context) ([]Session, error)
    Get(ctx context.Context, id string) (Session, error)
    Save(ctx context.Context, s Session) error
    Delete(ctx context.Context, id string) error
    FindRecyclable(ctx context.Context, remote string) (Session, error)
}
```

**`core/git`** - Git abstraction:
```go
// Git defines git operations needed by hive
type Git interface {
    Clone(ctx context.Context, url, dest string) error
    Checkout(ctx context.Context, dir, branch string) error
    Pull(ctx context.Context, dir string) error
    ResetHard(ctx context.Context, dir string) error
    RemoteURL(ctx context.Context, dir string) (string, error)
    IsClean(ctx context.Context, dir string) (bool, error)
}
```

**`core/config`** - Configuration:
```go
type Config struct {
    Commands    Commands
    GitPath     string
    Keybindings map[string]Keybinding
    DataDir     string  // resolved XDG path
}

type Commands struct {
    Spawn   []string
    Recycle []string
}

type Keybinding struct {
    Action  string  // built-in action name
    Help    string
    Sh      string  // shell command template
    Confirm string  // confirmation prompt
}
```

### `internal/hive/` - Business Logic

Orchestrates domain operations. This is where the application logic lives.

```go
type Service struct {
    sessions session.Store
    git      git.Git
    config   *config.Config
    executor executil.Executor
}

func (s *Service) CreateSession(ctx context.Context, name, prompt string) error {
    // 1. Check for recyclable session
    // 2. Clone or recycle
    // 3. Save session
    // 4. Spawn terminal
}

func (s *Service) RecycleSession(ctx context.Context, id string) error {
    // Update session state to recycled
}

func (s *Service) Prune(ctx context.Context) (int, error) {
    // Delete recycled sessions and their directories
}
```

### `internal/store/` - Storage Implementations

Concrete implementations of storage interfaces.

**`store/jsonfile`** - JSON file storage:
```go
type Store struct {
    path string
    mu   sync.RWMutex
}

func New(path string) *Store {
    return &Store{path: path}
}

func (s *Store) List(ctx context.Context) ([]session.Session, error) {
    // Read and parse JSON file
}
```

### `pkg/` - Reusable Utilities

Domain-agnostic utilities that could theoretically be extracted to separate modules.

**`pkg/executil`** - Shell execution:
```go
// Executor runs shell commands
type Executor interface {
    Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
    RunDir(ctx context.Context, dir, cmd string, args ...string) ([]byte, error)
}

// RealExecutor calls actual shell commands
type RealExecutor struct{}

// RecordingExecutor captures commands for testing
type RecordingExecutor struct {
    Commands []RecordedCommand
    Outputs  map[string][]byte
    Errors   map[string]error
}
```

**`pkg/tmpl`** - Template utilities:
```go
func Render(tmpl string, data any) (string, error) {
    // Parse and execute Go template
}
```

## Dependency Injection

All dependencies flow downward and are injected at construction time.

```
main.go
    │
    ├── config.Load()
    │
    ├── store/jsonfile.New(dataDir)
    │
    ├── core/git.NewExecutor(config.GitPath, executor)
    │
    ├── hive.NewService(store, git, config, executor)
    │
    └── commands.New*(service)
```

## Testing Strategy

### Unit Tests

For pure logic that doesn't touch the filesystem or shell.

```go
// session/session_test.go
func TestSession_CanRecycle(t *testing.T) {
    s := Session{State: StateActive}
    assert.True(t, s.CanRecycle())

    s.State = StateRecycled
    assert.False(t, s.CanRecycle())
}
```

### Interface-Based Testing

Mock interfaces for testing business logic in isolation.

```go
// hive/service_test.go
func TestService_CreateSession_RecyclesExisting(t *testing.T) {
    store := &mockStore{
        recyclable: session.Session{ID: "old", Path: "/tmp/old"},
    }
    git := &mockGit{}
    exec := &executil.RecordingExecutor{}

    svc := NewService(store, git, cfg, exec)
    err := svc.CreateSession(ctx, "new-feature", "implement X")

    require.NoError(t, err)
    assert.True(t, git.ResetHardCalled)
    assert.Contains(t, exec.Commands[0].Cmd, "wezterm")
}
```

### Integration Tests (Containerized)

For testing actual git operations and shell commands in an isolated environment.

```
test/integration/
├── Dockerfile          # Alpine + git + test dependencies
├── docker-compose.yml  # Test orchestration
├── git_test.go         # Real git operations
└── session_test.go     # Full session lifecycle
```

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine

RUN apk add --no-cache git bash

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Run integration tests
CMD ["go", "test", "-tags=integration", "-v", "./test/integration/..."]
```

**Test with build tag:**
```go
//go:build integration

package integration

func TestGitClone_RealRepository(t *testing.T) {
    // Create temp directory
    // Clone a real repository
    // Verify files exist
    // Clean up
}

func TestSessionLifecycle_EndToEnd(t *testing.T) {
    // Create session (clones repo)
    // Verify session in store
    // Mark as recycled
    // Create new session (should recycle)
    // Verify reused same directory
    // Prune
    // Verify directory deleted
}
```

**Running integration tests:**
```bash
# Via Taskfile
task test:integration

# Or directly
docker compose -f test/integration/docker-compose.yml up --build --abort-on-container-exit
```

### Test Utilities

Shared helpers in `test/testutil/`:

```go
package testutil

// TempGitRepo creates a temporary git repository for testing
func TempGitRepo(t *testing.T) string {
    t.Helper()
    dir := t.TempDir()
    // git init, add file, commit
    return dir
}

// AssertFileExists fails if path doesn't exist
func AssertFileExists(t *testing.T, path string) {
    t.Helper()
    _, err := os.Stat(path)
    require.NoError(t, err, "file should exist: %s", path)
}
```

## Data Flow Examples

### Creating a New Session

```
User: hive new
         │
         ▼
┌─────────────────┐
│  cmd_new.go     │  Parse flags, prompt for input
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  hive/service   │  Orchestrate creation
│                 │
│  1. FindRecyclable(remote)
│  2. if found: recycle()
│     else: clone()
│  3. Save(session)
│  4. Spawn(terminal)
└────────┬────────┘
         │
    ┌────┴────┬──────────┐
    ▼         ▼          ▼
┌───────┐ ┌───────┐ ┌─────────┐
│ store │ │  git  │ │executil │
└───────┘ └───────┘ └─────────┘
```

### TUI Session Management

```
User: hive (launches TUI)
         │
         ▼
┌─────────────────┐
│  cmd_tui.go     │  Initialize TUI
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  tui/app.go     │  Bubble Tea program
│                 │
│  - List sessions from service
│  - Handle keybindings
│  - Delegate actions to service
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  hive/service   │  Execute actions
└─────────────────┘
```

## Error Handling

Errors are wrapped with context as they propagate up:

```go
// Low level
func (e *Executor) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
    out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("exec %s: %w", cmd, err)
    }
    return out, nil
}

// Mid level
func (g *GitExecutor) Clone(ctx context.Context, url, dest string) error {
    _, err := g.exec.Run(ctx, g.gitPath, "clone", url, dest)
    if err != nil {
        return fmt.Errorf("clone %s to %s: %w", url, dest, err)
    }
    return nil
}

// High level
func (s *Service) CreateSession(ctx context.Context, name, prompt string) error {
    // ...
    if err := s.git.Clone(ctx, remote, path); err != nil {
        return fmt.Errorf("create session %s: %w", name, err)
    }
    // ...
}
```

## Configuration Loading

```go
func Load() (*Config, error) {
    // 1. Determine config path (XDG_CONFIG_HOME/hive/config.yaml)
    // 2. Read file if exists
    // 3. Apply defaults
    // 4. Validate
    // 5. Resolve data directory (XDG_DATA_HOME/hive)
    return cfg, nil
}
```

Default resolution order:
1. Explicit flag (`--config`)
2. Environment variable (`HIVE_CONFIG`)
3. XDG config path
4. Built-in defaults

## Future Considerations

- **Plugin system**: Keybindings already support arbitrary shell commands
- **Multiple remotes**: Session could track which remote it was cloned from
- **Session groups**: Group related sessions for batch operations
- **Hooks**: Pre/post hooks for session lifecycle events
