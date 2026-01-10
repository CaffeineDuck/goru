# goru

Run untrusted Python and JavaScript safely. No Docker, no VMs, just WebAssembly.

## The Problem

You need to execute user-submitted or AI-generated code. Your options:

- **Docker**: Requires daemon, orchestration, Linux features. Overkill for running a Python snippet.
- **Firecracker/gVisor**: Linux-only, complex setup, operational overhead.
- **Native execution**: Dangerous. One `os.system('rm -rf /')` and you're done.

## Who It's For

- **Code execution platforms** - LeetCode clones, coding interviews, online judges
- **LLM tool execution** - Run AI-generated code without risking your infrastructure
- **Plugin systems** - Let users extend your app with custom scripts
- **Educational platforms** - Safe code playgrounds for students

## Install

```bash
# CLI (from releases)
curl -fsSL https://github.com/caffeineduck/goru/releases/latest/download/goru-$(uname -s)-$(uname -m).tar.gz | tar xz
sudo mv goru /usr/local/bin/

# Go library
go get github.com/caffeineduck/goru
```

## CLI Quick Start

```bash
goru script.py                              # Run file
goru --lang python -c 'print(1+1)'          # Inline code
goru repl --lang python                     # Interactive REPL
goru --help                                 # Full options
```

## Go API

### Basic Execution

```go
import (
    "github.com/caffeineduck/goru/executor"
    "github.com/caffeineduck/goru/hostfunc"
    "github.com/caffeineduck/goru/language/python"
)

// Create executor
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

// Run code (stateless)
result := exec.Run(ctx, python.New(), `print("hello")`)
fmt.Println(result.Output)  // "hello\n"
fmt.Println(result.Error)   // nil
```

### Sessions (Persistent State)

```go
session, _ := exec.NewSession(python.New())
defer session.Close()

session.Run(ctx, `x = 42`)
session.Run(ctx, `y = x * 2`)
result := session.Run(ctx, `print(y)`)  // Output: "84\n"
```

### Session with Capabilities

```go
session, _ := exec.NewSession(python.New(),
    executor.WithSessionTimeout(10*time.Second),
    executor.WithSessionAllowedHosts([]string{"api.example.com"}),
    executor.WithSessionMount("/data", "./input", hostfunc.MountReadOnly),
    executor.WithSessionKV(),
)
defer session.Close()

// Now sandboxed code can use http, fs, kv modules
session.Run(ctx, `resp = http.get("https://api.example.com/users")`)
session.Run(ctx, `config = fs.read_json("/data/config.json")`)
session.Run(ctx, `kv.set("key", "value")`)
```

### Custom Host Functions

```go
registry := hostfunc.NewRegistry()
registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    id := args["id"].(string)
    return map[string]any{"id": id, "name": "Alice"}, nil
})

exec, _ := executor.New(registry)
session, _ := exec.NewSession(python.New())
session.Run(ctx, `user = call("get_user", id="123")`)  // user["name"] = "Alice"
```

### Executor Options

```go
exec, _ := executor.New(registry,
    executor.WithDiskCache(),                      // Cache compiled WASM (default)
    executor.WithMemoryLimit(executor.MemoryLimit64MB),
)
```

## Security Model

By default, sandboxed code has **zero capabilities**:

| Capability | Default | Enable With |
|------------|---------|-------------|
| Filesystem | Blocked | `WithMount()` |
| HTTP | Blocked | `WithAllowedHosts()` |
| KV Store | Blocked | `WithKV()` |
| Host Functions | Blocked | `Registry.Register()` |

## Python Packages

Only **pure Python** packages work (no C extensions, no sockets).

```bash
goru deps install pydantic python-dateutil
goru repl --lang python --packages .goru/python/packages
```

Works: `pydantic`, `attrs`, `python-dateutil`, `pyyaml`, `toml`, `jinja2`
Blocked: `numpy`, `pandas`, `requests`, `flask` (C extensions or sockets)

## Documentation

- [Go API](docs/api.md)
- [Host Functions](docs/host-functions.md)
- [Python](docs/python.md) | [JavaScript](docs/javascript.md)

## License

MIT
