# Agent Instructions

This is **goru**, a Go library that sandboxes Python and JavaScript code execution using WebAssembly.

## Project Structure

```
goru/
  cmd/goru/         CLI (run, repl, serve, deps commands)
  executor/         Core WASM execution engine
  hostfunc/         Host function implementations (HTTP, filesystem, KV)
  language/         Language runtimes (python/, javascript/)
  internal/tools/   Build tools (WASM downloader)
  docs/             Documentation
```

## Build Commands

```bash
# First time setup: download WASM runtimes (~28MB total)
go generate ./...

# Build the CLI binary
go build -o goru ./cmd/goru

# Run the CLI
./goru -c 'print(1+1)'              # Python (default)
./goru -lang js -c 'console.log(1)' # JavaScript
```

Note: `go generate` downloads Python (~26MB) and JavaScript (~1.4MB) WASM runtimes from GitHub releases. Files are cached locally after first download.

## Test Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./executor
go test ./hostfunc

# Run a single test by name
go test ./executor -run TestPythonBasicExecution
go test ./executor -run TestFilesystem       # Runs all TestFilesystem* tests
go test ./hostfunc -run TestHTTPRequest

# Run tests with verbose output
go test -v ./executor -run TestPythonCustomHostFunction
```

## Lint/Format Commands

```bash
# Format code
go fmt ./...
gofmt -w .

# Vet for common issues
go vet ./...
```

## Code Style Guidelines

### Imports

Group imports in this order, separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages

```go
import (
    "context"
    "fmt"
    "sync"

    "github.com/tetratelabs/wazero"

    "github.com/caffeineduck/goru/hostfunc"
)
```

### Naming Conventions

- **Types**: PascalCase (`Executor`, `Registry`, `SessionOption`)
- **Functions/Methods**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase (`sharedExec`, `httpHandler`)
- **Constants**: PascalCase for exported (`DefaultMaxBodySize`), camelCase for unexported
- **Interfaces**: Describe behavior, often end in `-er` (`Language`)
- **Options pattern**: `WithXxx` functions returning `Option` or `XxxOption`

### Error Handling

- Always wrap errors with context using `fmt.Errorf("action: %w", err)`
- Define sentinel errors with `errors.New()` for common cases
- Return errors immediately, don't accumulate them
- Use early returns for error cases

```go
// Good
if err != nil {
    return nil, fmt.Errorf("compile %s: %w", name, err)
}

// Sentinel errors
var (
    ErrSessionClosed = errors.New("session closed")
    ErrSessionBusy   = errors.New("session busy")
)
```

### Struct Design

- Use the functional options pattern for configurable types
- Provide sensible defaults via `defaultXxxConfig()` functions
- Keep config structs unexported when possible

```go
type Option func(*runConfig)

func WithTimeout(d time.Duration) Option {
    return func(c *runConfig) {
        c.timeout = d
    }
}
```

### Concurrency

- Protect shared state with `sync.Mutex` or `sync.RWMutex`
- Use channels for signaling completion
- Name mutex fields `mu` and place them near the fields they protect
- Always unlock with `defer` immediately after locking

```go
type Executor struct {
    mu       sync.RWMutex
    compiled map[string]wazero.CompiledModule
    closed   bool
}
```

### Documentation

- Write doc comments for all exported types, functions, and methods
- First sentence should be a complete sentence starting with the name
- Include examples for non-obvious usage

```go
// Result holds the output and metadata from code execution.
type Result struct { ... }

// WithMount adds a filesystem mount point with the specified permissions.
// The virtual path is what sandboxed code sees; host path is the actual location.
func WithMount(virtualPath, hostPath string, mode MountMode) Option { ... }
```

### Testing

- Use table-driven tests for multiple similar cases
- Use `t.TempDir()` for tests that need temporary files
- Share expensive resources (like WASM compilation) via `TestMain`
- Use descriptive test names: `TestXxx_scenario`

```go
func TestMain(m *testing.M) {
    sharedExec, _ = executor.New(hostfunc.NewRegistry())
    defer sharedExec.Close()
    os.Exit(m.Run())
}
```

### File Organization

- One primary type per file (e.g., `session.go` for `Session`)
- Put options in `options.go`
- Put tests in `*_test.go` files in the same package (or `_test` package for integration tests)

## Issue Tracking with bd (beads)

This project uses **bd** for issue tracking. Run `bd onboard` to get started.

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Session Completion Checklist

When ending a work session, complete ALL steps:

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed):
   ```bash
   go fmt ./...
   go vet ./...
   go test ./...
   ```
3. **Update issue status** - Close finished work, update in-progress items
4. **Clean up** - Clear stashes, prune remote branches
5. **Verify** - All changes committed AND pushed
6. **Hand off** - Provide context for next session
