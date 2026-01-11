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

## Who It's NOT For

- **High-performance computing** - WASM adds overhead; use native execution if you trust the code
- **NumPy/Pandas workloads** - C extensions don't work in WASI; pure Python only
- **Long-running processes** - Designed for short scripts, not daemons or servers
- **Multi-threaded code** - WASI doesn't support threads

## Limitations

**Performance**: WASM interpretation adds ~2-10x overhead vs native. Cold starts are slow (~500ms Python, ~100ms JS). We mitigate with disk caching and session reuse.

**WASI constraints**: No threads, no raw sockets, no FFI. Python's `multiprocessing`, `threading`, `socket`, `ssl` modules are blocked. C extensions (numpy, pandas) won't load.

**Abstraction layer**: `http`, `fs`, `kv` modules are wrappers over host functions, not real stdlib. They're injected into the runtime and call back to Go. This means:
- No `requests` library (use built-in `http` module instead)
- No direct filesystem access (only mounted paths via `fs`)
- Behavior may differ slightly from native equivalents

## Features

- **Bidirectional host-guest protocol** - Sandboxed code calls Go functions via `call()`, Go receives structured responses ([docs](docs/sandbox-api.md#call))
- **Extensible** - Register custom Go functions callable from sandbox via `Registry.Register()` ([docs](https://pkg.go.dev/github.com/caffeineduck/goru/hostfunc#Registry))
- **Async batching** - Concurrent host calls with `asyncio.gather()` / `Promise.all()` batched into single round-trip ([docs](docs/sandbox-api.md#async-batching))
- **Capability-based security** - Zero permissions by default, opt-in HTTP/filesystem/KV per-session ([docs](https://pkg.go.dev/github.com/caffeineduck/goru/executor#SessionOption))
- **Session state** - Variables persist across executions, define functions once and reuse ([docs](https://pkg.go.dev/github.com/caffeineduck/goru/executor#Session))
- **Built-in modules** - `http`, `fs`, `kv` available in sandbox without imports when enabled ([docs](docs/sandbox-api.md#modules))
- **Python packages** - Pre-install via CLI or allow runtime `install_pkg()` from sandboxed code ([docs](docs/sandbox-api.md#install_pkg-python-only))

## Security Model

By default, sandboxed code has **zero capabilities**:

| Capability | Default | Enable With |
|------------|---------|-------------|
| Filesystem | Blocked | `WithMount()` |
| HTTP | Blocked | `WithAllowedHosts()` |
| KV Store | Blocked | `WithKV()` |
| Host Functions | Blocked | `Registry.Register()` |

## Install

```bash
# CLI
curl -fsSL https://raw.githubusercontent.com/caffeineduck/goru/main/install.sh | bash

# Go library
go get github.com/caffeineduck/goru
```

## CLI Quick Start

```bash
goru script.py                              # Run file
goru --lang python -c 'print(1+1)'          # Inline code
echo 'print(1+1)' | goru -l python          # Pipe code via stdin
echo 'console.log(1+1)' | goru -l js        # Works with JavaScript too
goru repl --lang python                     # Interactive REPL
goru --help                                 # Full options
```

## Go API

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

session, _ := exec.NewSession(python.New(),
    executor.WithSessionAllowedHosts([]string{"api.example.com"}),
    executor.WithSessionMount("/data", "./input", hostfunc.MountReadOnly),
    executor.WithSessionKV(),
)
defer session.Close()

session.Run(ctx, `x = 42`)
session.Run(ctx, `resp = http.get("https://api.example.com/data")`)
result := session.Run(ctx, `print(x, resp["status"])`)
```

Full API: [pkg.go.dev/github.com/caffeineduck/goru](https://pkg.go.dev/github.com/caffeineduck/goru)

## Documentation

- [Sandbox API](docs/sandbox-api.md) - APIs for Python/JavaScript code running in the sandbox
- [Go API](https://pkg.go.dev/github.com/caffeineduck/goru) - Go embedding documentation on pkg.go.dev

## License

MIT
