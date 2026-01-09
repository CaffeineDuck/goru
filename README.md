# goru

WASM-based Python sandbox for Go. Run untrusted code without Docker or VMs.

## What it does

Executes Python code in a WebAssembly sandbox. The sandbox has no filesystem, network, or syscall access by default. You control what it can do through explicit host functions.

## Who it's for

- Running LLM-generated code safely
- Plugin systems where users provide Python logic
- Any case where you need `eval()` without the fear

## Install

```bash
go get github.com/caffeineduck/goru
```

## Usage

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
    registry := hostfunc.NewRegistry()
    exec, _ := executor.New(registry)
    defer exec.Close()

    result := exec.Run(context.Background(), python.New(), `print(sum(x**2 for x in range(10)))`)
    fmt.Print(result.Output) // 285
}
```

CLI:
```bash
go build -o goru ./cmd/goru
./goru -c 'print("hello")'
./goru script.py
echo 'print(1+1)' | ./goru
```

## Host Functions

Sandboxed code can only interact with the outside world through host functions you provide.

### Built-in

**KV Store** (always available):
```python
kv_set("key", "value")
kv_get("key")      # "value"
kv_delete("key")
```

**HTTP** (requires `WithAllowedHosts`):
```go
exec.Run(ctx, python.New(), code, executor.WithAllowedHosts([]string{"api.example.com"}))
```
```python
resp = http_get("https://api.example.com/data")
print(resp["status"], resp["body"])
```

**Filesystem** (requires `WithMount`):
```go
exec.Run(ctx, python.New(), code,
    executor.WithMount("/data", "./input", executor.MountReadOnly),
    executor.WithMount("/out", "./output", executor.MountReadWriteCreate),
)
```
```python
content = fs_read("/data/config.json")
fs_write("/out/result.txt", "done")
fs_list("/data")  # [{"name": "file.txt", "is_dir": false, "size": 123}]
```

### Custom

```go
registry := hostfunc.NewRegistry()
registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    return map[string]any{"id": args["id"], "name": "Alice"}, nil
})
```
```python
user = _goru_call("get_user", {"id": "123"})
```

## Configuration

```go
// Executor options (at creation)
exec, _ := executor.New(registry,
    executor.WithDiskCache(),                    // faster CLI startup
    executor.WithPrecompile(python.New()),       // compile at init
    executor.WithMemoryLimit(executor.MemoryLimit64MB),
)

// Run options (per execution)
exec.Run(ctx, python.New(), code,
    executor.WithTimeout(5*time.Second),
    executor.WithAllowedHosts([]string{"httpbin.org"}),
    executor.WithKVStore(sharedKV),
    executor.WithMount("/data", "./data", executor.MountReadOnly),
)
```

## Performance

| Scenario | Time |
|----------|------|
| Cold start (first run) | ~1.6s |
| Warm start (cached) | ~120ms |
| Native Python | ~14ms |

The value is isolation, not speed. If you need raw performance, don't use a sandbox.

## Python Runtime

**CPython 3.12.0** compiled to WASI by [VMware Labs](https://github.com/vmware-labs/webassembly-language-runtimes).

175 stdlib modules work: json, re, datetime, collections, itertools, math, random, hashlib, csv, dataclasses, typing, pathlib, asyncio, sqlite3 (in-memory), etc.

**Blocked** (WASI limitations): multiprocessing, ssl, raw sockets, subprocess, direct filesystem access.

Type stubs at `language/python/goru.pyi` for IDE autocomplete.

## Roadmap

- [x] Python (CPython 3.12)
- [x] Module caching
- [x] HTTP, KV, filesystem host functions
- [ ] JavaScript (QuickJS)
- [ ] Stdio streaming

## License

MIT
