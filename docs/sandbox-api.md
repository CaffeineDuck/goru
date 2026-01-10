# Sandbox API

APIs available inside sandboxed Python and JavaScript code.

## Modules

All modules require explicit opt-in via Go options or CLI flags.

| Module | Enable With | Description |
|--------|-------------|-------------|
| `http` | `WithSessionAllowedHosts()` | HTTP client |
| `fs` | `WithSessionMount()` | Filesystem access |
| `kv` | `WithSessionKV()` | Key-value store |
| `install_pkg` | `WithAllowedPackages()` | Package installation (Python only) |

## http

HTTP client for allowed hosts only.

**Python:**
```python
resp = http.get("https://api.example.com/data")
resp = http.post(url, body='{"x":1}', headers={"Content-Type": "application/json"})
resp = http.put(url, body='...')
resp = http.patch(url, body='...')
resp = http.delete(url)

resp.ok           # True if 2xx
resp.status_code  # 200
resp.text         # body string
resp.headers      # dict
resp.json()       # parsed JSON

# Async
resp = await http.async_get(url)
```

**JavaScript:**
```javascript
const resp = http.get("https://api.example.com/data");
const resp = http.post(url, {body: '{"x":1}', headers: {"Content-Type": "application/json"}});

resp.ok          // true if 2xx
resp.statusCode  // 200
resp.text        // body string
resp.headers     // object
resp.json()      // parsed JSON

// Async
const resp = await http.asyncGet(url);
```

## fs

Filesystem access to mounted paths only.

**Python:**
```python
fs.read_text("/data/file.txt")
fs.read_json("/data/config.json")
fs.write_text("/out/result.txt", "content")
fs.write_json("/out/data.json", {"x": 1}, indent=2)
fs.listdir("/data")      # [{"name": "file.txt", "is_dir": False, "size": 123}]
fs.exists("/data/file")
fs.stat("/data/file")    # {"name", "size", "is_dir", "mod_time"}
fs.mkdir("/out/subdir")
fs.remove("/out/temp.txt")

# Async
content = await fs.async_read_text("/data/file.txt")
```

**JavaScript:**
```javascript
fs.readText("/data/file.txt");
fs.readJson("/data/config.json");
fs.writeText("/out/result.txt", "content");
fs.writeJson("/out/data.json", {x: 1}, 2);
fs.listdir("/data");
fs.exists("/data/file");
fs.stat("/data/file");
fs.mkdir("/out/subdir");
fs.remove("/out/temp.txt");

// Async
const content = await fs.asyncReadText("/data/file.txt");
```

| Operation | Required Mode |
|-----------|---------------|
| read, listdir, exists, stat | `MountReadOnly` |
| write, remove | `MountReadWrite` |
| mkdir | `MountReadWriteCreate` |

## kv

In-memory key-value store. Values are JSON-serializable.

**Python:**
```python
kv.set("key", {"nested": "value"})
kv.get("key")                      # returns value or None
kv.get("missing", default="x")     # returns "x"
kv.delete("key")
kv.keys()                          # ["key", ...]

# Async
await kv.async_set("key", "value")
value = await kv.async_get("key")
```

**JavaScript:**
```javascript
kv.set("key", {nested: "value"});
kv.get("key");                     // returns value or null
kv.get("missing", "default");      // returns "default"
kv.delete("key");
kv.keys();                         // ["key", ...]

// Async
await kv.asyncSet("key", "value");
const value = await kv.asyncGet("key");
```

## call

Call custom host functions registered in Go.

**Python:**
```python
user = call("get_user", id="123")
user = await async_call("get_user", id="123")
```

**JavaScript:**
```javascript
const user = call("get_user", {id: "123"});
const user = await asyncCall("get_user", {id: "123"});
```

## Async Batching

Concurrent host calls are batched into a single round-trip.

**Python:**
```python
import asyncio

async def main():
    results = await asyncio.gather(
        http.async_get("https://api1.example.com"),
        http.async_get("https://api2.example.com"),
        kv.async_get("key"),
    )
    return results

asyncio.run(main())
```

**JavaScript:**
```javascript
const results = await runAsync(
    http.asyncGet("https://api1.example.com"),
    http.asyncGet("https://api2.example.com"),
    kv.asyncGet("key"),
);
```

## install_pkg (Python only)

Runtime package installation. Requires `WithAllowedPackages()`.

```python
install_pkg("pydantic")
install_pkg("python-dateutil", ">=2.8")
from pydantic import BaseModel
```

Pre-install packages for faster startup:
```bash
goru deps install pydantic python-dateutil
goru repl --lang python --packages .goru/python/packages
```

## time

**Python:**
```python
import time
time.time()  # real host time (monkey-patched)
```

**JavaScript:**
```javascript
time_now()  // Unix timestamp
```

---

## Python

**Runtime:** CPython 3.12 (WASI)

### Stdlib

175 modules work including: json, csv, re, collections, itertools, math, random, datetime, sqlite3 (in-memory), asyncio, typing, dataclasses.

**Blocked:** multiprocessing, threading, ssl, socket, subprocess, raw filesystem access.

### Packages

Pure Python packages work. C extensions (numpy, pandas) don't.

| Works | Doesn't Work |
|-------|--------------|
| pydantic, attrs, pyyaml | numpy, pandas |
| python-dateutil, jinja2 | requests (use `http`) |
| zod, toml | Django, Flask |

### Type Stubs

Copy `language/python/goru.pyi` to your project for IDE autocomplete.

---

## JavaScript

**Runtime:** QuickJS-NG (ES2023)

### Features

ES2023 support, async/await, faster cold start than Python.

**Blocked:** fetch (use `http`), setTimeout, require/import, Node.js APIs.

### Packages

Bundle dependencies with esbuild/webpack:
```bash
npx esbuild your-code.js --bundle --outfile=bundled.js --format=esm
goru bundled.js
```

### Type Definitions

Copy `language/javascript/goru.d.ts` to your project for IDE autocomplete.
