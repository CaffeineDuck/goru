# Python

**Runtime:** CPython 3.12.0 compiled to WASI
**Source:** [VMware Labs WebAssembly Language Runtimes](https://github.com/vmware-labs/webassembly-language-runtimes)

## Available Stdlib Modules

175 stdlib modules work. Key ones:

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
| Async | asyncio (custom WASI event loop) |
| Testing | unittest, doctest |

## Blocked Modules

| Module | Reason |
|--------|--------|
| `multiprocessing` | No process support in WASI |
| `threading` | Single-threaded only |
| `ssl` | Not compiled in |
| `socket` (raw) | No network stack |
| `subprocess` | No process spawning |
| `os.listdir`, `open()` | No filesystem (use `fs` module) |

## Goru APIs

```python
# KV Store (requires WithKV)
kv.set("key", {"nested": "value"})
kv.get("key")                      # returns value or None
kv.get("missing", default="x")     # returns "x"
kv.delete("key")
kv.keys()                          # ["key", ...]

# HTTP (requires WithAllowedHosts)
resp = http.get("https://api.example.com")
resp = http.post(url, body='{"x":1}', headers={"Content-Type": "application/json"})
resp.ok           # True if 2xx
resp.status_code  # 200
resp.text         # body string
resp.json()       # parsed JSON

# Filesystem (requires WithMount)
fs.read_text("/data/file.txt")
fs.read_json("/data/config.json")
fs.write_text("/out/result.txt", "content")
fs.write_json("/out/data.json", {"x": 1}, indent=2)
fs.listdir("/data")      # [{"name": "file.txt", "is_dir": False, "size": 123}]
fs.exists("/data/file")
fs.stat("/data/file")    # {"name", "size", "is_dir", "mod_time"}
fs.mkdir("/out/subdir")
fs.remove("/out/temp.txt")

# Runtime Package Install (requires WithAllowedPackages)
install_pkg("requests")
install_pkg("pydantic", ">=2.0")
import requests  # now available

# Time (monkey-patched stdlib)
import time
time.time()  # real host time

# Custom Host Functions
user = call("get_user", id="123")
```

## Async Support

```python
import asyncio

async def main():
    # Concurrent host function calls
    results = await asyncio.gather(
        http.async_get("https://api1.example.com"),
        http.async_get("https://api2.example.com"),
        kv.async_get("key"),
    )
    return results

asyncio.run(main())  # Patched to use WASI-compatible event loop
```

All sync methods have async variants: `async_get`, `async_set`, `async_read_text`, etc.

## Package Installation

**Pre-install (recommended):**
```bash
goru deps install requests pydantic
goru repl -packages .goru/python/packages
```

**Runtime install:**
```go
session, _ := exec.NewSession(python.New(),
    executor.WithAllowedPackages([]string{"requests>=2.32", "pydantic>=2.0"}))
```

```python
install_pkg("requests")  # Downloads from PyPI at runtime
import requests
```

### Package Limitations

Not all packages work in WASI:

| Works | Doesn't Work |
|-------|--------------|
| Pure Python packages | C extensions (numpy, pandas) |
| pydantic, requests | aiohttp (needs async sockets) |
| dataclasses, attrs | Django, Flask (need sockets) |
| json, yaml parsers | ML libraries (tensorflow, torch) |

Use `pip install --target` to pre-install, or `install_pkg()` for runtime install of allowlisted packages.

## Type Stubs

IDE autocomplete: `language/python/goru.pyi`
