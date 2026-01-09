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

### Running Code

```go
result := exec.Run(ctx, language, code, opts...)
```

**Run options:**

| Option | Description |
|--------|-------------|
| `WithTimeout(dur)` | Execution timeout (default 30s) |
| `WithAllowedHosts([]string)` | Hosts for `http.get` |
| `WithKVStore(*KVStore)` | Share KV across runs |
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

### KV Store

```go
kv := hostfunc.NewKVStore()
// Pass to executor:
executor.WithKVStore(kv)
```

### Filesystem

```go
fs := hostfunc.NewFS([]hostfunc.Mount{
    {VirtualPath: "/data", HostPath: "./input", Mode: hostfunc.MountReadOnly},
})
```
