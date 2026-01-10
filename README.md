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

# Or build from source
git clone https://github.com/caffeineduck/goru && cd goru
go generate ./... && go build -o goru ./cmd/goru

# Go library
go get github.com/caffeineduck/goru
```

## Quick Start

```bash
# Run code (language auto-detected from file extension)
goru script.py
goru script.js

# Inline code (--lang required)
goru --lang python -c 'print(1+1)'
goru --lang js -c 'console.log(1+1)'

# Interactive REPL
goru repl --lang python
```

## Security Model

By default, sandboxed code has **zero capabilities** - no filesystem, no network, no system calls.

| Capability | Default | Enable With |
|------------|---------|-------------|
| Filesystem | Blocked | `--mount /data:./local:ro` |
| HTTP | Blocked | `--allow-host api.example.com` |
| KV Store | Blocked | `--kv` |
| Memory | 256MB | `--memory 64mb` |

## Enabling Capabilities

```bash
# HTTP access (sandboxed code uses goru's http module, not requests/urllib)
goru --lang python -c 'print(http.get("https://api.example.com").json())' \
    --allow-host api.example.com

# Filesystem access
goru --lang python -c 'print(fs.read_text("/data/config.json"))' \
    --mount /data:./mydata:ro

# KV store
goru --lang python -c 'kv.set("x", 42); print(kv.get("x"))' --kv
```

## Go API

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

// Session with persistent state
session, _ := exec.NewSession(python.New())
session.Run(ctx, `x = 42`)
session.Run(ctx, `print(x)`)  // 42

// With capabilities
session, _ := exec.NewSession(python.New(),
    executor.WithSessionAllowedHosts([]string{"api.example.com"}),
    executor.WithSessionMount("/data", "./input", hostfunc.MountReadOnly),
    executor.WithSessionKV(),
)
```

## Python Packages

Only **pure Python** packages work (no C extensions, no sockets).

```bash
# Install packages (downloads directly from PyPI, no pip required)
goru deps install pydantic python-dateutil

# Use in REPL
goru repl --lang python --packages .goru/python/packages
```

Works: `pydantic`, `attrs`, `python-dateutil`, `pyyaml`, `toml`, `jinja2`
Blocked: `numpy`, `pandas`, `requests`, `flask` (C extensions or sockets)

## Documentation

- [CLI Reference](docs/cli.md)
- [Go API](docs/api.md)
- [Host Functions](docs/host-functions.md)
- [Python](docs/python.md) | [JavaScript](docs/javascript.md)

## License

MIT
