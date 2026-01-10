# goru

Run untrusted Python and JavaScript code safely, without Docker or VMs.

## The Problem

You need to execute user-submitted or AI-generated code. Your options:

- **Docker**: Requires daemon, orchestration, Linux features. Overkill for running a Python snippet.
- **Firecracker/gVisor**: Linux-only, complex setup, operational overhead.
- **Native execution**: Dangerous. One `os.system('rm -rf /')` and you're done.

goru is a Go library that sandboxes code execution using WebAssembly. No containers, no VMs, no external dependencies.

## Who It's For

- **Code execution platforms** - LeetCode clones, coding interviews, online judges
- **LLM tool execution** - Run AI-generated code without risking your infrastructure
- **Plugin systems** - Let users extend your app with custom scripts
- **Educational platforms** - Safe code playgrounds for students

## Install

```bash
go get github.com/caffeineduck/goru
```

## Usage

### Stateless Execution

```go
package main

import (
    "context"
    "fmt"

    "github.com/caffeineduck/goru/executor"
    "github.com/caffeineduck/goru/hostfunc"
    "github.com/caffeineduck/goru/language/python"
    "github.com/caffeineduck/goru/language/javascript"
)

func main() {
    exec, _ := executor.New(hostfunc.NewRegistry())
    defer exec.Close()

    // Python
    result := exec.Run(context.Background(), python.New(), `print("Hello from Python")`)
    fmt.Println(result.Output)

    // JavaScript
    result = exec.Run(context.Background(), javascript.New(), `console.log("Hello from JS")`)
    fmt.Println(result.Output)
}
```

### Sessions (Persistent State)

Sessions keep the interpreter alive between executions, allowing variables and functions to persist:

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

session, _ := exec.NewSession(python.New())
defer session.Close()

// State persists between runs
session.Run(ctx, `x = 42`)
session.Run(ctx, `def greet(name): return f"Hello, {name}!"`)
result := session.Run(ctx, `print(greet("World"), x)`)
// Output: Hello, World! 42
```

## Security Model

By default, sandboxed code can do nothing dangerous:

| Capability | Default | How to Enable |
|------------|---------|---------------|
| Filesystem | Blocked | `WithMount("/data", "./input", MountReadOnly)` |
| Network | Blocked | `WithAllowedHosts([]string{"api.example.com"})` |
| System calls | Blocked | N/A (WASM has no syscalls) |

Capabilities are explicitly granted, not revoked:

```go
// Allow reading from ./input, writing to ./output
// Allow HTTP only to api.openai.com
result := exec.Run(ctx, python.New(), code,
    executor.WithMount("/input", "./input", executor.MountReadOnly),
    executor.WithMount("/output", "./output", executor.MountReadWrite),
    executor.WithAllowedHosts([]string{"api.openai.com"}),
    executor.WithTimeout(10*time.Second),
)
```

## Features

**Languages**: Python 3.12, JavaScript ES2023

**Host Functions**: Sandboxed code can call back into Go through controlled interfaces:

```python
# HTTP (requires WithAllowedHosts)
resp = http.get("https://api.example.com/data")
print(resp.json())

# Filesystem (requires WithMount)
data = fs.read_text("/input/config.json")
fs.write_text("/output/result.txt", "done")
```

**Async Support**: True async/await with concurrent host function execution:

```python
import asyncio

async def main():
    # These run concurrently
    results = await asyncio.gather(
        http.async_get("https://api1.example.com"),
        http.async_get("https://api2.example.com"),
        http.async_get("https://api3.example.com"),
    )
    return results

asyncio.run(main())
```

**Custom Host Functions**: Expose your own Go functions to sandboxed code:

```go
registry := hostfunc.NewRegistry()
registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    userID := args["id"].(string)
    return map[string]any{"id": userID, "name": "Alice"}, nil
})

exec, _ := executor.New(registry)
```

```python
user = call("get_user", id="123")
print(user["name"])  # Alice
```

**CLI**: Run code from command line, REPL, or HTTP server:

```bash
goru -c 'print(1+1)'                    # Python (default)
goru -lang js -c 'console.log(1+1)'     # JavaScript
goru script.py                          # From file
goru repl                               # Interactive REPL with persistent state
goru serve -port 8080                   # HTTP API server

# Package management (Python only)
goru deps install pydantic requests     # Install packages
goru deps list                          # List installed packages
```

## Trade-offs

**Cold start is slow.** First execution compiles WASM, which takes time (especially Python). Use `WithDiskCache()` to cache compilation:

```go
exec, _ := executor.New(registry, executor.WithDiskCache())
```

**Not native performance.** Code runs in a WASM interpreter. Fine for scripts, not for compute-heavy workloads.

**Limited language support.** Only Python and JavaScript. Adding languages requires WASM-compiled runtimes.

## Documentation

- [API Reference](docs/api.md) - Go library usage
- [Host Functions](docs/host-functions.md) - HTTP, filesystem, custom functions
- [CLI Reference](docs/cli.md) - Command-line, REPL, and server mode
- [Languages](docs/languages.md) - Python/JS specifics and limitations

## License

MIT
