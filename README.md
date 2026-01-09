# goru

**WASM-based code execution sandbox for Go applications.**

Run untrusted Python (and soon JavaScript) code in a secure, isolated environment — no Docker, no VMs, just a single Go binary.

```go
exec, _ := executor.New(registry)
result := exec.Run(ctx, python.New(), `print("Hello from sandbox!")`)
fmt.Println(result.Output) // "Hello from sandbox!"
```

## Features

- **🔒 Secure by default** — No filesystem, no network, no syscalls unless you explicitly allow them
- **⚡ Fast warm starts** — 120ms after initial compilation (vs 1.6s cold start)
- **📦 Single binary** — WASM runtime embedded, no external dependencies
- **🔌 Extensible** — Define custom host functions to expose your own APIs
- **🌍 Cross-platform** — Works everywhere Go runs (Linux, macOS, Windows, ARM)

## Installation

```bash
go get github.com/caffeineduck/goru
```

## Quick Start

### As a Library

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/caffeineduck/goru/executor"
    "github.com/caffeineduck/goru/hostfunc"
    "github.com/caffeineduck/goru/language/python"
)

func main() {
    // Create a registry for host functions
    registry := hostfunc.NewRegistry()
    
    // Create an executor (compiles WASM once, reuses for all runs)
    exec, _ := executor.New(registry)
    defer exec.Close()
    
    // Run Python code
    result := exec.Run(context.Background(), python.New(), `
import json
data = {"message": "Hello from Python!", "numbers": [1, 2, 3]}
print(json.dumps(data))
`)
    
    fmt.Println(result.Output)
    // {"message": "Hello from Python!", "numbers": [1, 2, 3]}
}
```

### As a CLI

```bash
# Build the CLI
go build -o goru ./cmd/goru

# Run Python code
./goru -c 'print("Hello!")'

# Run from file
./goru script.py

# With timeout
./goru -timeout 5s -c 'while True: pass'
```

## Host Functions

Sandboxed code can only interact with the outside world through host functions that you explicitly provide.

### Built-in Host Functions

#### Key-Value Store
```python
# In-memory key-value storage
kv_set("user:1", '{"name": "Alice"}')
data = kv_get("user:1")  # Returns '{"name": "Alice"}'
kv_delete("user:1")
```

#### HTTP Requests (with allowlist)
```go
// Go: Allow specific hosts
result := exec.Run(ctx, python.New(), code,
    executor.WithAllowedHosts([]string{"api.example.com", "httpbin.org"}),
)
```

```python
# Python: Make requests to allowed hosts only
response = http_get("https://api.example.com/data")
print(response["status"])  # 200
print(response["body"])    # Response body as string
```

### Custom Host Functions

```go
// Define your own host functions
registry := hostfunc.NewRegistry()

registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    userID := args["id"].(string)
    // Look up user in your database
    return map[string]any{
        "id":   userID,
        "name": "Alice",
    }, nil
})

registry.Register("send_email", func(ctx context.Context, args map[string]any) (any, error) {
    to := args["to"].(string)
    subject := args["subject"].(string)
    // Send email through your email service
    return "sent", nil
})

exec, _ := executor.New(registry)
```

```python
# Use custom functions from Python
user = _goru_call("get_user", {"id": "123"})
print(f"Hello, {user['name']}!")

_goru_call("send_email", {
    "to": "alice@example.com",
    "subject": "Welcome!"
})
```

## Performance

| Mode | Time | Notes |
|------|------|-------|
| Cold start | 1.6s | First run compiles WASM |
| Warm start (in-memory) | 120ms | Reuses compiled module |
| Warm start (disk cache) | 680ms | CLI repeated calls |
| Native Python | 14ms | For comparison (not isolated) |

### Parallel Execution

goru shines when running multiple executions in parallel — all share the same compiled runtime:

```go
exec, _ := executor.New(registry)
defer exec.Close()

// 10 parallel executions share one compiled runtime
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(n int) {
        defer wg.Done()
        exec.Run(ctx, python.New(), fmt.Sprintf("print(%d)", n))
    }(i)
}
wg.Wait()
```

| Parallel runs | goru | Docker |
|---------------|------|--------|
| 10 executions | 225ms | 1,044ms |

## Configuration

### Executor Options

```go
// Enable disk cache for faster CLI startup
exec, _ := executor.New(registry, executor.WithDiskCache())

// Custom cache directory
exec, _ := executor.New(registry, executor.WithCacheDir("/tmp/my-cache"))

// Precompile languages at startup
exec, _ := executor.New(registry, executor.WithPrecompile(python.New()))
```

### Run Options

```go
result := exec.Run(ctx, lang, code,
    executor.WithTimeout(30*time.Second),
    executor.WithAllowedHosts([]string{"api.example.com"}),
    executor.WithKVStore(sharedKV),  // Share KV across runs
)
```

## Adding New Languages

Implement the `Language` interface to add support for new languages:

```go
type Language interface {
    Name() string              // Unique identifier
    Module() []byte            // WASM binary
    WrapCode(code string) string   // Prepend stdlib
    Args(code string) []string     // CLI args for WASM
}
```

See `language/python/python.go` for a complete example.

## Security Model

1. **No capabilities by default** — Sandboxed code cannot access filesystem, network, or system resources
2. **Explicit host functions** — You define exactly what the sandbox can do
3. **Allowlists** — HTTP requests require explicit host allowlisting
4. **Timeouts** — Prevent infinite loops with configurable timeouts
5. **WASM isolation** — Memory-safe execution, no buffer overflows

## Comparison

| Feature | goru | Docker | Native |
|---------|------|--------|--------|
| Isolation | WASM sandbox | Container | None |
| Startup (warm) | 120ms | 180ms | 14ms |
| Dependencies | None | Docker daemon | Python |
| Binary size | 36MB | 144MB+ image | N/A |
| Fine-grained control | Per-function | Coarse | None |
| Platform | Anywhere Go runs | Linux (or VM) | Varies |

## Use Cases

- **AI agents** — Let LLMs write and execute code safely
- **Plugin systems** — Users provide custom logic in Python
- **Serverless** — Code execution without containers
- **Notebooks** — REPL environments with controlled access
- **CI/CD** — Run untrusted build scripts securely

## Roadmap

- [x] Python support (RustPython WASM)
- [x] Compilation caching (in-memory + disk)
- [x] HTTP host functions
- [x] Key-value storage
- [ ] JavaScript support (QuickJS WASM)
- [ ] Filesystem host functions with mount points
- [ ] Resource limits (memory, CPU time)
- [ ] Stdio streaming

## License

MIT
