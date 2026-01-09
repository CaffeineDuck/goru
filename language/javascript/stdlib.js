// goru host function bindings for JavaScript

// =============================================================================
// Synchronous Host Function Protocol
// =============================================================================

const _goru_call = (fn, args) => {
  std.err.puts("\x00GORU:" + JSON.stringify({fn, args}) + "\x00");
  std.err.flush();
  const resp = JSON.parse(std.in.getline());
  if (resp.error) {
    throw new Error(resp.error);
  }
  return resp.data;
};

// =============================================================================
// Async Support - Batched host calls with Promises
// =============================================================================

const _asyncBatch = {
  pending: new Map(),  // id -> {resolve, reject}
  nextId: 0,

  queue(fn, args) {
    const id = String(this.nextId++);
    const promise = new Promise((resolve, reject) => {
      this.pending.set(id, {resolve, reject});
    });
    // Send request with ID (non-blocking, just queues)
    std.err.puts("\x00GORU:" + JSON.stringify({id, fn, args}) + "\x00");
    std.err.flush();
    return promise;
  },

  flush() {
    if (this.pending.size === 0) return;

    const count = this.pending.size;
    std.err.puts("\x00GORU_FLUSH:" + count + "\x00");
    std.err.flush();

    // Read exactly 'count' responses
    for (let i = 0; i < count; i++) {
      const line = std.in.getline();
      const resp = JSON.parse(line);
      const id = resp.id;
      if (this.pending.has(id)) {
        const {resolve, reject} = this.pending.get(id);
        this.pending.delete(id);
        if (resp.error) {
          reject(new Error(resp.error));
        } else {
          resolve(resp.data);
        }
      }
    }
  }
};

// Async call helper - returns a Promise
const asyncCall = (fn, args) => _asyncBatch.queue(fn, args);

// Flush pending requests and resolve promises
const flushAsync = () => _asyncBatch.flush();

// Run multiple async operations concurrently (like Promise.all but with flush)
const runAsync = async (...promises) => {
  // Flush to process all queued requests
  flushAsync();
  // Wait for all promises
  return Promise.all(promises);
};

// =============================================================================
// Time Functions
// =============================================================================

const time_now = () => _goru_call("time_now", {});

// =============================================================================
// Synchronous Host Functions
// =============================================================================

const kv_get = (key) => _goru_call("kv_get", {key});
const kv_set = (key, value) => _goru_call("kv_set", {key, value});
const kv_delete = (key) => _goru_call("kv_delete", {key});
const http_get = (url) => _goru_call("http_get", {url});
const fs_read = (path) => _goru_call("fs_read", {path});
const fs_write = (path, content) => _goru_call("fs_write", {path, content});
const fs_list = (path) => _goru_call("fs_list", {path});
const fs_exists = (path) => _goru_call("fs_exists", {path});
const fs_mkdir = (path) => _goru_call("fs_mkdir", {path});
const fs_remove = (path) => _goru_call("fs_remove", {path});
const fs_stat = (path) => _goru_call("fs_stat", {path});

// =============================================================================
// Async Host Functions - return Promises, use with runAsync()
// =============================================================================

const async_kv_get = (key) => asyncCall("kv_get", {key});
const async_kv_set = (key, value) => asyncCall("kv_set", {key, value});
const async_kv_delete = (key) => asyncCall("kv_delete", {key});
const async_http_get = (url) => asyncCall("http_get", {url});
const async_fs_read = (path) => asyncCall("fs_read", {path});
const async_fs_write = (path, content) => asyncCall("fs_write", {path, content});
const async_fs_list = (path) => asyncCall("fs_list", {path});
const async_fs_exists = (path) => asyncCall("fs_exists", {path});
const async_fs_mkdir = (path) => asyncCall("fs_mkdir", {path});
const async_fs_remove = (path) => asyncCall("fs_remove", {path});
const async_fs_stat = (path) => asyncCall("fs_stat", {path});
