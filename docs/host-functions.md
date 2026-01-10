# Host Functions

Host functions let sandboxed code call back into Go. They're the only way sandboxed code can interact with the outside world.

## Built-in Modules

### http - HTTP Client

Requires `WithAllowedHosts`. Supports all HTTP methods with headers and body.

**Go:**
```go
exec.Run(ctx, lang, code, executor.WithAllowedHosts([]string{"api.example.com"}))
```

**Python:**
```python
# GET
resp = http.get("https://api.example.com/data")
resp = http.get("https://api.example.com/data", headers={"Authorization": "Bearer token"})

# POST, PUT, PATCH, DELETE
resp = http.post("https://api.example.com/data", body='{"key": "value"}', headers={"Content-Type": "application/json"})
resp = http.put("https://api.example.com/data/1", body='{"key": "updated"}')
resp = http.patch("https://api.example.com/data/1", body='{"key": "patched"}')
resp = http.delete("https://api.example.com/data/1")

# Generic request
resp = http.request("OPTIONS", "https://api.example.com/data")

# Response
resp.ok           # True if 2xx
resp.status_code  # 200
resp.text         # response body string
resp.headers      # response headers dict
resp.json()       # parse JSON

# Async
resp = await http.async_get("https://api.example.com/data")
resp = await http.async_post("https://api.example.com/data", body='{}')
```

**JavaScript:**
```javascript
// GET
const resp = http.get("https://api.example.com/data");
const resp = http.get("https://api.example.com/data", {headers: {"Authorization": "Bearer token"}});

// POST, PUT, PATCH, DELETE
const resp = http.post("https://api.example.com/data", {body: '{"key": "value"}', headers: {"Content-Type": "application/json"}});
const resp = http.put("https://api.example.com/data/1", {body: '{"key": "updated"}'});
const resp = http.patch("https://api.example.com/data/1", {body: '{"key": "patched"}'});
const resp = http.delete("https://api.example.com/data/1");

// Generic request
const resp = http.request("OPTIONS", "https://api.example.com/data");

// Response
resp.ok          // true if 2xx
resp.statusCode  // 200
resp.text        // response body string
resp.headers     // response headers object
resp.json()      // parse JSON

// Async
const resp = await http.asyncGet("https://api.example.com/data");
const resp = await http.asyncPost("https://api.example.com/data", {body: '{}'});
```

Host matching: exact match or subdomain (e.g., `api.example.com` allows `sub.api.example.com`).

### fs - Filesystem

Requires `WithMount`. Virtual paths mapped to host paths with explicit permissions.

**Go:**
```go
exec.Run(ctx, lang, code,
    executor.WithMount("/data", "./input", executor.MountReadOnly),
    executor.WithMount("/out", "./output", executor.MountReadWriteCreate),
)
```

**Python:**
```python
fs.read_text("/data/file.txt")
fs.read_json("/data/config.json")
fs.write_text("/out/result.txt", "content")
fs.write_json("/out/data.json", {"key": "value"}, indent=2)
fs.listdir("/data")
fs.exists("/data/file.txt")
fs.stat("/data/file.txt")
fs.mkdir("/out/subdir")
fs.remove("/out/temp.txt")

# Async versions
await fs.async_read_text("/data/file.txt")
await fs.async_write_text("/out/result.txt", "content")
# ... etc
```

**JavaScript:**
```javascript
fs.readText("/data/file.txt");
fs.readJson("/data/config.json");
fs.writeText("/out/result.txt", "content");
fs.writeJson("/out/data.json", {key: "value"}, 2);
fs.listdir("/data");
fs.exists("/data/file.txt");
fs.stat("/data/file.txt");
fs.mkdir("/out/subdir");
fs.remove("/out/temp.txt");

// Async versions
await fs.asyncReadText("/data/file.txt");
await fs.asyncWriteText("/out/result.txt", "content");
// ... etc
```

| Method | Description | Min Mode |
|--------|-------------|----------|
| `read_text` / `readText` | Read file contents | ro |
| `read_json` / `readJson` | Read and parse JSON file | ro |
| `write_text` / `writeText` | Write string to file | rw |
| `write_json` / `writeJson` | Write data as JSON | rw |
| `listdir` | List directory | ro |
| `exists` | Check if path exists | ro |
| `stat` | Get file info | ro |
| `mkdir` | Create directory | rwc |
| `remove` | Delete file/empty dir | rw |

**Return types:**

```python
# fs.listdir returns:
[{"name": "file.txt", "is_dir": False, "size": 123}, ...]

# fs.stat returns:
{"name": "file.txt", "size": 123, "is_dir": False, "mod_time": 1704067200}
```

Path traversal attacks (`../`) are blocked.

### time

**Python:**
```python
import time
time.time()  # Returns real host time (monkey-patched)
```

**JavaScript:**
```javascript
time_now()  // Returns Unix timestamp
```

### install_pkg - Runtime Package Installation

Requires `WithPackageInstall` or `WithAllowedPackages`. Python only.

**Go:**
```go
// Allow any package
session, _ := exec.NewSession(python.New(), executor.WithPackageInstall(true))

// Restrict to specific packages
session, _ := exec.NewSession(python.New(),
    executor.WithAllowedPackages([]string{"requests", "pydantic"}))
```

**Python:**
```python
install_pkg("requests")
install_pkg("pydantic", ">=2.0")

import requests
resp = requests.get("https://example.com")
```

Packages are installed to `.goru/python/packages` using pip. Version specifiers (`>=`, `<=`, `==`, `~=`) are supported.

## Async Support

Both Python and JavaScript support true async/await with concurrent host function execution.

**Python:**
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

`asyncio.run` is patched to use goru's WASM-compatible event loop. You can also use `run_async(coro)` directly.

**JavaScript:**
```javascript
const results = await runAsync(
    http.asyncGet("https://api1.example.com"),
    http.asyncGet("https://api2.example.com"),
    http.asyncGet("https://api3.example.com"),
);
```

## Custom Functions

Register your own functions in Go:

```go
registry := hostfunc.NewRegistry()

registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    userID := args["id"].(string)
    return map[string]any{
        "id":   userID,
        "name": "Alice",
    }, nil
})

exec, _ := executor.New(registry)
```

Call from sandboxed code:

**Python:**
```python
user = call("get_user", id="123")
print(user["name"])  # Alice

# Async
user = await async_call("get_user", id="123")
```

**JavaScript:**
```javascript
const user = call("get_user", {id: "123"});
console.log(user.name);  // Alice

// Async
const user = await asyncCall("get_user", {id: "123"});
```

## Type Stubs

For IDE autocomplete:

- Python: `language/python/goru.pyi`
- JavaScript: `language/javascript/goru.d.ts`
