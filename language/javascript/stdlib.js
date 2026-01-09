// goru host function bindings for JavaScript

// =============================================================================
// Internal: Synchronous Host Function Protocol
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
// Internal: Async Support - Batched host calls with Promises
// =============================================================================

const _asyncBatch = {
  pending: new Map(),
  nextId: 0,

  queue(fn, args) {
    const id = String(this.nextId++);
    const promise = new Promise((resolve, reject) => {
      this.pending.set(id, {resolve, reject});
    });
    std.err.puts("\x00GORU:" + JSON.stringify({id, fn, args}) + "\x00");
    std.err.flush();
    return promise;
  },

  flush() {
    if (this.pending.size === 0) return;
    const count = this.pending.size;
    std.err.puts("\x00GORU_FLUSH:" + count + "\x00");
    std.err.flush();
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

const _asyncCall = (fn, args) => _asyncBatch.queue(fn, args);
const flushAsync = () => _asyncBatch.flush();

const runAsync = async (...promises) => {
  flushAsync();
  return Promise.all(promises);
};

// =============================================================================
// kv - Key-Value Store Module
// =============================================================================

const kv = {
  get(key, defaultValue = null) {
    const result = _goru_call("kv_get", {key});
    return result !== null ? result : defaultValue;
  },

  set(key, value) {
    return _goru_call("kv_set", {key, value});
  },

  delete(key) {
    return _goru_call("kv_delete", {key});
  },

  async asyncGet(key, defaultValue = null) {
    const result = await _asyncCall("kv_get", {key});
    return result !== null ? result : defaultValue;
  },

  async asyncSet(key, value) {
    return await _asyncCall("kv_set", {key, value});
  },

  async asyncDelete(key) {
    return await _asyncCall("kv_delete", {key});
  }
};

// =============================================================================
// http - HTTP Client Module
// =============================================================================

class HTTPResponse {
  constructor(data) {
    this._data = data;
    this.statusCode = data.status || 0;
    this.text = data.body || "";
  }

  get ok() {
    return this.statusCode >= 200 && this.statusCode < 300;
  }

  json() {
    return JSON.parse(this.text);
  }
}

const http = {
  get(url) {
    const data = _goru_call("http_get", {url});
    return new HTTPResponse(data);
  },

  async asyncGet(url) {
    const data = await _asyncCall("http_get", {url});
    return new HTTPResponse(data);
  }
};

// =============================================================================
// fs - Filesystem Module
// =============================================================================

const fs = {
  readText(path) {
    return _goru_call("fs_read", {path});
  },

  readJson(path) {
    return JSON.parse(this.readText(path));
  },

  writeText(path, content) {
    return _goru_call("fs_write", {path, content});
  },

  writeJson(path, data, indent = null) {
    const content = indent ? JSON.stringify(data, null, indent) : JSON.stringify(data);
    return this.writeText(path, content);
  },

  listdir(path) {
    return _goru_call("fs_list", {path});
  },

  exists(path) {
    return _goru_call("fs_exists", {path});
  },

  mkdir(path) {
    return _goru_call("fs_mkdir", {path});
  },

  remove(path) {
    return _goru_call("fs_remove", {path});
  },

  stat(path) {
    return _goru_call("fs_stat", {path});
  },

  // Async versions
  async asyncReadText(path) {
    return await _asyncCall("fs_read", {path});
  },

  async asyncReadJson(path) {
    const text = await this.asyncReadText(path);
    return JSON.parse(text);
  },

  async asyncWriteText(path, content) {
    return await _asyncCall("fs_write", {path, content});
  },

  async asyncListdir(path) {
    return await _asyncCall("fs_list", {path});
  },

  async asyncExists(path) {
    return await _asyncCall("fs_exists", {path});
  },

  async asyncMkdir(path) {
    return await _asyncCall("fs_mkdir", {path});
  },

  async asyncRemove(path) {
    return await _asyncCall("fs_remove", {path});
  },

  async asyncStat(path) {
    return await _asyncCall("fs_stat", {path});
  }
};

// =============================================================================
// Time
// =============================================================================

const time_now = () => _goru_call("time_now", {});

