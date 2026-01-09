# Languages

## Python

**Runtime:** CPython 3.12.0 compiled to WASI
**Source:** [VMware Labs WebAssembly Language Runtimes](https://github.com/vmware-labs/webassembly-language-runtimes)
**Size:** ~25MB
**Performance:** ~1.5s cold, ~120ms warm

### Available Modules

175 stdlib modules work:

| Category | Modules |
|----------|---------|
| Data | json, csv, pickle, struct, base64, hashlib |
| Text | re, string, textwrap, difflib |
| Collections | collections, itertools, functools, heapq, bisect |
| Math | math, random, decimal, fractions, statistics |
| Time | datetime, calendar, time, zoneinfo |
| Types | typing, dataclasses, enum, abc |
| Parsing | ast, tokenize, argparse, configparser |
| Database | sqlite3 (in-memory only) |
| Async | asyncio (with custom WASI event loop) |
| Testing | unittest, doctest |

### Blocked (WASI limitations)

| Module | Reason |
|--------|--------|
| `multiprocessing` | No process support |
| `threading` | Single-threaded only |
| `ssl` | Not compiled in |
| `socket` (raw) | No network stack |
| `subprocess` | No process spawning |
| `os.listdir`, `open()` | No filesystem (use `fs` module) |

### Built-in Modules

```python
# Key-value store
kv.set("key", "value")
kv.get("key", default="fallback")
kv.delete("key")

# HTTP client (requires WithAllowedHosts)
resp = http.get(url)
resp.ok, resp.status_code, resp.text, resp.json()

# Filesystem (requires WithMount)
fs.read_text(path)
fs.read_json(path)
fs.write_text(path, content)
fs.write_json(path, data, indent=2)
fs.listdir(path)
fs.exists(path)
fs.stat(path)
fs.mkdir(path)
fs.remove(path)

# Time (monkey-patched)
import time
time.time()  # Real host time

# Async support
import asyncio
async def main():
    results = await asyncio.gather(
        kv.async_get("a"),
        kv.async_get("b"),
    )
run_async(main())
```

### Type Stubs

`language/python/goru.pyi` - see file for full API.

---

## JavaScript

**Runtime:** QuickJS-NG compiled to WASI
**Source:** [nicholasareed/nicholasareed.github.io](https://github.com/nicholasareed/nicholasareed.github.io)
**Size:** ~1.5MB
**Performance:** ~200ms cold, ~60ms warm

### Features

- ES2023 support
- `std` and `os` modules (via `--std` flag)
- Single-threaded

### Blocked

| Feature | Reason |
|---------|--------|
| `fetch` | No network (use `http.get`) |
| `require`/`import` | No module loading |
| `setTimeout` | No event loop |
| File I/O | No filesystem (use `fs` module) |

### Built-in Modules

```javascript
// Key-value store
kv.set("key", "value");
kv.get("key", "fallback");
kv.delete("key");

// HTTP client (requires WithAllowedHosts)
const resp = http.get(url);
resp.ok, resp.statusCode, resp.text, resp.json()

// Filesystem (requires WithMount)
fs.readText(path);
fs.readJson(path);
fs.writeText(path, content);
fs.writeJson(path, data, 2);
fs.listdir(path);
fs.exists(path);
fs.stat(path);
fs.mkdir(path);
fs.remove(path);

// Time
time_now()  // Unix timestamp

// Async support
const results = await runAsync(
    kv.asyncGet("a"),
    kv.asyncGet("b"),
);
```

### Type Definitions

`language/javascript/goru.d.ts` - see file for full API.

---

## Performance Comparison

| Language | Cold Start | Warm Start | Binary Size |
|----------|------------|------------|-------------|
| JavaScript | ~200ms | ~60ms | 1.5MB |
| Python | ~1.5s | ~120ms | 25MB |

JavaScript is recommended for latency-sensitive applications. Python is better for data processing and when stdlib access matters.
