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

**CLI** (from [releases](https://github.com/caffeineduck/goru/releases)):
```bash
# macOS/Linux
curl -fsSL https://github.com/caffeineduck/goru/releases/latest/download/goru-$(uname -s)-$(uname -m).tar.gz | tar xz
sudo mv goru /usr/local/bin/

# Or build from source
git clone https://github.com/caffeineduck/goru && cd goru
go generate ./... && go build -o goru ./cmd/goru
```

**Go Library**:
```bash
go get github.com/caffeineduck/goru
```

## CLI Usage

```bash
goru -c 'print(1+1)'                     # Run Python
goru -lang js -c 'console.log(1+1)'      # Run JavaScript
goru script.py                           # Run file
goru repl                                # Interactive REPL
goru repl -kv                            # REPL with KV store
```

**Python Packages** - Install PyPI packages for sandboxed use:
```bash
goru deps install requests pydantic      # Install to .goru/python/packages
goru repl -packages .goru/python/packages
```

**Capabilities** - Enable features explicitly:
```bash
goru -c 'print(http.get("https://api.example.com").text)' \
    -allow-host api.example.com          # Allow HTTP

goru -c 'print(fs.read_text("/data/in.txt"))' \
    -mount /data:./mydata:ro             # Mount filesystem (ro/rw/rwc)

goru -c 'kv.set("x", 42); print(kv.get("x"))' \
    -kv                                  # Enable KV store
```

## Go API

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

// Stateless execution
result := exec.Run(ctx, python.New(), `print("hello")`)

// Session with persistent state
session, _ := exec.NewSession(python.New())
session.Run(ctx, `x = 42`)
session.Run(ctx, `print(x)`)  // 42
```

**Enable capabilities**:
```go
result := exec.Run(ctx, python.New(), code,
    executor.WithAllowedHosts([]string{"api.example.com"}),
    executor.WithMount("/data", "./input", executor.MountReadOnly),
    executor.WithKV(),
    executor.WithTimeout(10*time.Second),
)
```

## Security Model

By default, sandboxed code can do nothing dangerous:

| Capability | Default | How to Enable |
|------------|---------|---------------|
| Filesystem | Blocked | `WithMount("/data", "./input", MountReadOnly)` |
| Network | Blocked | `WithAllowedHosts([]string{"api.example.com"})` |
| KV Store | Blocked | `WithKV()` |
| Memory | 256MB | `WithMemoryLimit(executor.MemoryLimit64MB)` |
| System calls | Blocked | N/A (WASM has no syscalls) |

## Sandboxed APIs

Available to sandboxed code when enabled:

```python
# HTTP (requires -allow-host or WithAllowedHosts)
resp = http.get("https://api.example.com/data")
print(resp.json())

# Filesystem (requires -mount or WithMount)
data = fs.read_text("/data/config.json")
fs.write_text("/output/result.txt", "done")

# KV Store (requires -kv or WithKV)
kv.set("key", {"nested": "value"})
print(kv.get("key"))

# Custom host functions
user = call("get_user", id="123")  # Calls Go function
```

**Async support** for concurrent operations:
```python
import asyncio
results = await asyncio.gather(
    http.async_get("https://api1.example.com"),
    http.async_get("https://api2.example.com"),
)
```

## Trade-offs

- **Cold start is slow** - WASM compilation takes time. Use `WithDiskCache()` or `-no-cache=false` (default).
- **Not native performance** - Fine for scripts, not for compute-heavy workloads.
- **Python and JavaScript only** - Adding languages requires WASM-compiled runtimes.

## Documentation

- [CLI Reference](docs/cli.md)
- [Go API](docs/api.md)
- [Host Functions](docs/host-functions.md)
- [Language Details](docs/languages.md)

## License

MIT
