# goru

WASM code sandbox for Go. Execute untrusted Python/JavaScript without Docker or VMs.

## Install

```bash
go get github.com/caffeineduck/goru
```

## Quick Start

```go
exec, _ := executor.New(hostfunc.NewRegistry())
defer exec.Close()

// Python
exec.Run(ctx, python.New(), `print(1+1)`)

// JavaScript
exec.Run(ctx, javascript.New(), `console.log(1+1)`)
```

CLI:
```bash
goru -c 'print(1+1)'           # Python (default)
goru -lang js -c 'console.log(1)'  # JavaScript
goru script.py                  # Auto-detect from extension
goru serve -port 8080           # HTTP server mode
```

## Features

| Feature | Description |
|---------|-------------|
| Languages | Python 3.12, JavaScript ES2023 |
| Isolation | No fs/network/syscalls by default |
| Host functions | KV store, HTTP (allowlist), filesystem (mounts) |
| Server mode | HTTP API with session-based state |
| Performance | JS: ~60ms, Python: ~120ms (warm) |

## Documentation

- [API Reference](docs/api.md) - Go library usage
- [CLI Reference](docs/cli.md) - Command-line usage
- [Host Functions](docs/host-functions.md) - Built-in and custom functions
- [Languages](docs/languages.md) - Python/JS specifics and limitations

## License

MIT
