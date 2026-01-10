# JavaScript

**Runtime:** QuickJS-NG compiled to WASI
**Source:** [quickjs-ng/quickjs](https://github.com/quickjs-ng/quickjs)

## Features

- ES2023 support (classes, async/await, modules syntax)
- `std` and `os` modules via QuickJS
- Single-threaded execution
- Much faster cold start than Python

## Blocked Features

| Feature | Reason | Alternative |
|---------|--------|-------------|
| `fetch` | No network stack | `http.get()` |
| `require`/dynamic `import` | No module loading | Bundle code |
| `setTimeout`/`setInterval` | No event loop | N/A |
| `fs` (Node.js) | No filesystem | `fs.readText()` |
| npm packages | No package manager | Bundle with esbuild/webpack |

## Goru APIs

```javascript
// KV Store (requires WithKV)
kv.set("key", {nested: "value"});
kv.get("key");                     // returns value or null
kv.get("missing", "default");      // returns "default"
kv.delete("key");
kv.keys();                         // ["key", ...]

// HTTP (requires WithAllowedHosts)
const resp = http.get("https://api.example.com");
const resp = http.post(url, {body: '{"x":1}', headers: {"Content-Type": "application/json"}});
resp.ok          // true if 2xx
resp.statusCode  // 200
resp.text        // body string
resp.json()      // parsed JSON

// Filesystem (requires WithMount)
fs.readText("/data/file.txt");
fs.readJson("/data/config.json");
fs.writeText("/out/result.txt", "content");
fs.writeJson("/out/data.json", {x: 1}, 2);
fs.listdir("/data");      // [{name: "file.txt", is_dir: false, size: 123}]
fs.exists("/data/file");
fs.stat("/data/file");    // {name, size, is_dir, mod_time}
fs.mkdir("/out/subdir");
fs.remove("/out/temp.txt");

// Time
time_now();  // Unix timestamp (seconds)

// Custom Host Functions
const user = call("get_user", {id: "123"});
```

## Async Support

```javascript
// Concurrent host function calls
const results = await runAsync(
    http.asyncGet("https://api1.example.com"),
    http.asyncGet("https://api2.example.com"),
    kv.asyncGet("key"),
);

// Or use flushAsync() manually
const p1 = kv.asyncGet("a");
const p2 = kv.asyncGet("b");
flushAsync();
const [a, b] = await Promise.all([p1, p2]);
```

All sync methods have async variants: `asyncGet`, `asyncSet`, `asyncReadText`, etc.

## Using npm Packages

JavaScript doesn't have runtime package install like Python. Bundle your dependencies:

```bash
# Install locally
npm install lodash

# Bundle with esbuild
npx esbuild your-code.js --bundle --outfile=bundled.js --format=esm

# Run bundled code
goru bundled.js
```

### Package Limitations

| Works (when bundled) | Doesn't Work |
|---------------------|--------------|
| lodash, underscore | axios (needs fetch) |
| date-fns, dayjs | node-fetch |
| Pure JS utilities | Anything using Node APIs |
| zod, yup | Anything using fs, http, etc. |

## Type Definitions

IDE autocomplete: `language/javascript/goru.d.ts`

Copy to your project and reference in `tsconfig.json`:
```json
{
  "compilerOptions": {
    "types": ["./goru.d.ts"]
  }
}
```
