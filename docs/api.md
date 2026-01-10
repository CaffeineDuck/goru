# API Reference

## Executor

```go
import "github.com/caffeineduck/goru/executor"
```

### Creating an Executor

```go
registry := hostfunc.NewRegistry()
exec, err := executor.New(registry, opts...)
defer exec.Close()
```

**Options:**

| Option | Description |
|--------|-------------|
| `WithDiskCache(dir...)` | Cache compiled WASM to disk. Default: `~/.cache/goru` |
| `WithPrecompile(lang)` | Compile language at startup |
| `WithMemoryLimit(pages)` | Max WASM memory (64KB per page) |

**Memory constants:** `MemoryLimit1MB`, `MemoryLimit16MB`, `MemoryLimit64MB`, `MemoryLimit256MB`, `MemoryLimit1GB`

### Running Code (Stateless)

```go
result := exec.Run(ctx, language, code, opts...)
```

**Run options:**

| Option | Description |
|--------|-------------|
| `WithTimeout(dur)` | Execution timeout (default 30s) |
| `WithAllowedHosts([]string)` | Hosts for HTTP requests |
| `WithHTTPTimeout(dur)` | HTTP request timeout |
| `WithMount(virtual, host, mode)` | Mount filesystem |

**Mount modes:** `MountReadOnly`, `MountReadWrite`, `MountReadWriteCreate`

### Result

```go
type Result struct {
    Output   string        // stdout + stderr
    Duration time.Duration
    Error    error
}
```

## Sessions

Sessions keep the interpreter alive between executions, allowing variables and functions to persist.

### Creating a Session

```go
session, err := exec.NewSession(language, opts...)
defer session.Close()
```

**Session options:**

| Option | Description |
|--------|-------------|
| `WithSessionTimeout(dur)` | Timeout per execution (default 30s) |
| `WithPackages(path)` | Mount packages directory |

### Running Code in Session

```go
result := session.Run(ctx, code)
```

State persists between runs:

```go
session.Run(ctx, `x = 42`)
session.Run(ctx, `def greet(name): return f"Hello, {name}!"`)
result := session.Run(ctx, `print(greet("World"), x)`)
// Output: Hello, World! 42
```

### Example

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

session, _ := exec.NewSession(python.New())
defer session.Close()

// Define a function
session.Run(ctx, `
def fibonacci(n):
    if n <= 1: return n
    return fibonacci(n-1) + fibonacci(n-2)
`)

// Use it in subsequent runs
result := session.Run(ctx, `print(fibonacci(10))`)
fmt.Println(result.Output)  // 55
```

## Languages

```go
import "github.com/caffeineduck/goru/language/python"
import "github.com/caffeineduck/goru/language/javascript"
```

```go
python.New()      // Python 3.12
javascript.New()  // JavaScript ES2023
```

## Host Functions

```go
import "github.com/caffeineduck/goru/hostfunc"
```

### Registry

```go
registry := hostfunc.NewRegistry()
registry.Register("name", func(ctx context.Context, args map[string]any) (any, error) {
    return result, nil
})
```

### Filesystem

```go
fs := hostfunc.NewFS([]hostfunc.Mount{
    {VirtualPath: "/data", HostPath: "./input", Mode: hostfunc.MountReadOnly},
})
```
