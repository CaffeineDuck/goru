# CLI Reference

## Installation

```bash
go install github.com/caffeineduck/goru/cmd/goru@latest
# or
go build -o goru ./cmd/goru
```

## Commands

### run (default)

Execute code directly.

```bash
goru [options] [file]
goru run [options] [file]
```

**Options:**

| Flag | Description | Default |
|------|-------------|---------|
| `-c` | Code string | - |
| `-lang` | Language (python, js) | auto-detect |
| `-timeout` | Execution timeout | 30s |
| `-allow-host` | Allow HTTP host (repeatable) | - |
| `-mount` | Mount spec (repeatable) | - |
| `-no-cache` | Disable disk cache | false |
| `-http-max-url` | Max HTTP URL length | 8192 |
| `-http-max-body` | Max HTTP response body | 1048576 |
| `-fs-max-file` | Max file read size | 10485760 |
| `-fs-max-write` | Max file write size | 10485760 |
| `-fs-max-path` | Max path length | 4096 |

**Examples:**

```bash
# Inline code
goru -c 'print(1+1)'
goru -lang js -c 'console.log(1+1)'

# From file (language auto-detected)
goru script.py
goru app.js

# From stdin
echo 'print("hello")' | goru

# With options
goru -timeout 5s -c 'while True: pass'
goru -allow-host api.example.com -c 'print(http.get("https://api.example.com/data"))'
goru -mount /data:./input:ro -c 'print(fs.read_text("/data/config.json"))'
```

### repl

Interactive REPL with persistent state.

```bash
goru repl [options]
```

**Options:**

| Flag | Description | Default |
|------|-------------|---------|
| `-lang` | Language (python, js) | python |
| `-packages` | Path to packages directory (Python only) | - |
| `-no-cache` | Disable disk cache | false |

**Example:**

```bash
goru repl
>>> x = 42
>>> def greet(name): return f"Hello, {name}!"
>>> print(greet("World"), x)
Hello, World! 42
>>> exit
```

### serve

Start HTTP server for code execution.

```bash
goru serve [options]
```

**Options:**

| Flag | Description | Default |
|------|-------------|---------|
| `-port` | Listen port | 8080 |
| `-lang` | Default language | python |
| `-timeout` | Default timeout | 30s |
| `-allow-host` | Allow HTTP host (repeatable) | - |
| `-mount` | Mount spec (repeatable) | - |
| `-no-cache` | Disable disk cache | false |
| `-http-max-url` | Max HTTP URL length | 8192 |
| `-http-max-body` | Max HTTP response body | 1048576 |
| `-fs-max-file` | Max file read size | 10485760 |
| `-fs-max-write` | Max file write size | 10485760 |
| `-fs-max-path` | Max path length | 4096 |

**API Endpoints:**

```
POST /execute        Execute code (stateless)
POST /sessions       Create a new session
POST /sessions/{id}/exec  Execute code in session
DELETE /sessions/{id}     Close session
GET /health          Health check
```

**Stateless Execution:**

```
POST /execute
{
  "code": "print(1+1)",
  "lang": "python",        // optional, uses server default
  "timeout": "5s"          // optional, overrides server default
}

Response:
{
  "output": "2\n",
  "duration_ms": 120,
  "error": null
}
```

**Session-based Execution (persistent state):**

```bash
# Create session
curl -X POST localhost:8080/sessions -d '{"lang": "python"}'
# Response: {"session_id": "abc123..."}

# Execute code (state persists)
curl -X POST localhost:8080/sessions/abc123/exec \
  -d '{"code": "x = 42"}'
curl -X POST localhost:8080/sessions/abc123/exec \
  -d '{"code": "print(x)"}'  # Output: 42

# Close session
curl -X DELETE localhost:8080/sessions/abc123
```

**Example:**

```bash
# Start server
goru serve -port 8080 -allow-host httpbin.org

# Stateless execution
curl -X POST localhost:8080/execute \
  -H "Content-Type: application/json" \
  -d '{"code": "print(1+1)"}'
```

### deps

Manage Python packages for sandboxed code.

```bash
goru deps <command> [options]
```

**Commands:**

| Command | Description |
|---------|-------------|
| `install <packages...>` | Install packages to .goru/packages |
| `list` | List installed packages |
| `remove <packages...>` | Remove packages |
| `cache clear` | Clear download cache |

**Options:**

| Flag | Description | Default |
|------|-------------|---------|
| `-dir` | Package directory | .goru/packages |

**Examples:**

```bash
# Install packages
goru deps install pydantic requests

# List installed
goru deps list

# Remove package
goru deps remove pydantic

# Use with REPL
goru deps install pydantic
goru repl -packages .goru/packages
>>> from pydantic import BaseModel
```

Note: JavaScript packages are not supported. Use bundling for JS.

## Mount Syntax

```
-mount virtual:host:mode
```

| Mode | Description |
|------|-------------|
| `ro` | Read only |
| `rw` | Read/write existing files |
| `rwc` | Read/write/create |

**Examples:**

```bash
-mount /data:./input:ro
-mount /output:./results:rw
-mount /workspace:./work:rwc
```
