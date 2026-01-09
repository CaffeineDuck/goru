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
| Async | asyncio (single-threaded) |
| Testing | unittest, doctest |

### Blocked (WASI limitations)

| Module | Reason |
|--------|--------|
| `multiprocessing` | No process support |
| `threading` | Single-threaded only |
| `ssl` | Not compiled in |
| `socket` (raw) | No network stack |
| `subprocess` | No process spawning |
| `os.listdir`, `open()` | No filesystem (use mounts) |

### Type Stubs

`language/python/goru.pyi`:

```python
from typing import TypedDict

class HTTPResponse(TypedDict):
    status: int
    body: str

class FSEntry(TypedDict):
    name: str
    is_dir: bool
    size: int

class FSStatResult(TypedDict):
    name: str
    size: int
    is_dir: bool
    mod_time: int

def kv_get(key: str) -> str | None: ...
def kv_set(key: str, value: str) -> str: ...
def kv_delete(key: str) -> str: ...
def http_get(url: str) -> HTTPResponse: ...
def fs_read(path: str) -> str: ...
def fs_write(path: str, content: str) -> str: ...
def fs_list(path: str) -> list[FSEntry]: ...
def fs_exists(path: str) -> bool: ...
def fs_mkdir(path: str) -> str: ...
def fs_remove(path: str) -> str: ...
def fs_stat(path: str) -> FSStatResult: ...
```

---

## JavaScript

**Runtime:** QuickJS-NG compiled to WASI
**Source:** [paralin/go-quickjs-wasi](https://github.com/paralin/go-quickjs-wasi)
**Size:** ~1.5MB
**Performance:** ~200ms cold, ~60ms warm

### Features

- ES2023 support
- `std` and `os` modules (via `--std` flag)
- Single-threaded

### Blocked

| Feature | Reason |
|---------|--------|
| `fetch` | No network (use `http_get`) |
| `require`/`import` | No module loading |
| `setTimeout` | No event loop |
| File I/O | No filesystem (use mounts) |

### Type Definitions

`language/javascript/goru.d.ts`:

```typescript
interface HTTPResponse {
  status: number;
  body: string;
}

interface FSEntry {
  name: string;
  is_dir: boolean;
  size: number;
}

interface FSStatResult {
  name: string;
  size: number;
  is_dir: boolean;
  mod_time: number;
}

declare function kv_get(key: string): string | null;
declare function kv_set(key: string, value: string): string;
declare function kv_delete(key: string): string;
declare function http_get(url: string): HTTPResponse;
declare function fs_read(path: string): string;
declare function fs_write(path: string, content: string): string;
declare function fs_list(path: string): FSEntry[];
declare function fs_exists(path: string): boolean;
declare function fs_mkdir(path: string): string;
declare function fs_remove(path: string): string;
declare function fs_stat(path: string): FSStatResult;
```

---

## Performance Comparison

| Language | Cold Start | Warm Start | Binary Size |
|----------|------------|------------|-------------|
| JavaScript | ~200ms | ~60ms | 1.5MB |
| Python | ~1.5s | ~120ms | 25MB |

JavaScript is recommended for latency-sensitive applications. Python is better for data processing and when stdlib access matters.
