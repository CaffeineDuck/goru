# Host Functions

Host functions let sandboxed code call back into Go. They're the only way sandboxed code can interact with the outside world.

## Built-in Functions

### KV Store

Always available. In-memory key-value storage.

**Python:**
```python
kv_set("key", "value")  # returns "ok"
kv_get("key")           # returns "value" or None
kv_delete("key")        # returns "ok"
```

**JavaScript:**
```javascript
kv_set("key", "value")  // returns "ok"
kv_get("key")           // returns "value" or null
kv_delete("key")        // returns "ok"
```

### HTTP

Requires `WithAllowedHosts`. GET requests only.

**Go:**
```go
exec.Run(ctx, lang, code, executor.WithAllowedHosts([]string{"api.example.com"}))
```

**Python:**
```python
resp = http_get("https://api.example.com/data")
print(resp["status"])  # 200
print(resp["body"])    # response body string
```

**JavaScript:**
```javascript
const resp = http_get("https://api.example.com/data");
console.log(resp.status);  // 200
console.log(resp.body);    // response body string
```

Host matching: exact match or subdomain (e.g., `api.example.com` allows `sub.api.example.com`).

### Filesystem

Requires `WithMount`. Virtual paths mapped to host paths with explicit permissions.

**Go:**
```go
exec.Run(ctx, lang, code,
    executor.WithMount("/data", "./input", executor.MountReadOnly),
    executor.WithMount("/out", "./output", executor.MountReadWriteCreate),
)
```

| Function | Description | Min Mode |
|----------|-------------|----------|
| `fs_read(path)` | Read file contents | ro |
| `fs_write(path, content)` | Write to file | rw |
| `fs_list(path)` | List directory | ro |
| `fs_exists(path)` | Check if path exists | ro |
| `fs_stat(path)` | Get file info | ro |
| `fs_mkdir(path)` | Create directory | rwc |
| `fs_remove(path)` | Delete file/empty dir | rw |

**Return types:**

```python
# fs_list returns:
[{"name": "file.txt", "is_dir": False, "size": 123}, ...]

# fs_stat returns:
{"name": "file.txt", "size": 123, "is_dir": False, "mod_time": 1704067200}
```

Path traversal attacks (`../`) are blocked.

## Custom Functions

Register your own functions in Go:

```go
registry := hostfunc.NewRegistry()

registry.Register("get_user", func(ctx context.Context, args map[string]any) (any, error) {
    userID := args["id"].(string)
    // Look up user in database
    return map[string]any{
        "id":   userID,
        "name": "Alice",
        "email": "alice@example.com",
    }, nil
})

registry.Register("send_email", func(ctx context.Context, args map[string]any) (any, error) {
    to := args["to"].(string)
    subject := args["subject"].(string)
    body := args["body"].(string)
    // Send via your email service
    return "sent", nil
})

exec, _ := executor.New(registry)
```

Call from sandboxed code:

**Python:**
```python
user = _goru_call("get_user", {"id": "123"})
print(user["name"])  # Alice

_goru_call("send_email", {
    "to": "alice@example.com",
    "subject": "Hello",
    "body": "Welcome!"
})
```

**JavaScript:**
```javascript
const user = _goru_call("get_user", {id: "123"});
console.log(user.name);  // Alice

_goru_call("send_email", {
    to: "alice@example.com",
    subject: "Hello",
    body: "Welcome!"
});
```

## Type Stubs

For IDE autocomplete:

- Python: `language/python/goru.pyi`
- JavaScript: `language/javascript/goru.d.ts`
