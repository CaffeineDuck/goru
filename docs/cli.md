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
| `-kv-max-key` | Max KV key size | 1024 |
| `-kv-max-value` | Max KV value size | 1048576 |
| `-kv-max-entries` | Max KV entries | 10000 |
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
goru -allow-host api.example.com -c 'print(http_get("https://api.example.com/data"))'
goru -mount /data:./input:ro -c 'print(fs_read("/data/config.json"))'
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
| `-session-ttl` | Session expiry | 1h |
| `-allow-host` | Allow HTTP host (repeatable) | - |
| `-mount` | Mount spec (repeatable) | - |
| `-no-cache` | Disable disk cache | false |
| `-kv-max-key` | Max KV key size | 1024 |
| `-kv-max-value` | Max KV value size | 1048576 |
| `-kv-max-entries` | Max KV entries | 10000 |
| `-http-max-url` | Max HTTP URL length | 8192 |
| `-http-max-body` | Max HTTP response body | 1048576 |
| `-fs-max-file` | Max file read size | 10485760 |
| `-fs-max-write` | Max file write size | 10485760 |
| `-fs-max-path` | Max path length | 4096 |

**API:**

```
POST /execute
{
  "code": "print(1+1)",
  "lang": "python",        // optional, uses server default
  "session_id": "user-1",  // optional, for KV persistence
  "timeout": "5s"          // optional, overrides server default
}

Response:
{
  "output": "2\n",
  "duration_ms": 120,
  "error": null
}
```

```
GET /health
Response: ok
```

**Example:**

```bash
# Start server
goru serve -port 8080 -allow-host httpbin.org

# Execute code
curl -X POST localhost:8080/execute \
  -H "Content-Type: application/json" \
  -d '{"code": "print(1+1)"}'

# With session (KV persists across requests)
curl -X POST localhost:8080/execute \
  -d '{"code": "kv_set(\"x\", \"hello\")", "session_id": "user-1"}'
curl -X POST localhost:8080/execute \
  -d '{"code": "print(kv_get(\"x\"))", "session_id": "user-1"}'
```

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
