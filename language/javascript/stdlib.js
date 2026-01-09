// =============================================================================
// Core: Host Function Protocol
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

const call = (fn, args) => _goru_call(fn, args || {});
const asyncCall = (fn, args) => _asyncCall(fn, args || {});

// =============================================================================
// kv - Key-Value Store
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
// http - HTTP Client
// =============================================================================

class HTTPResponse {
  constructor(data) {
    this._data = data;
    this.statusCode = data.status || 0;
    this.text = data.body || "";
    this.headers = data.headers || {};
  }

  get ok() {
    return this.statusCode >= 200 && this.statusCode < 300;
  }

  json() {
    return JSON.parse(this.text);
  }
}

const http = {
  request(method, url, options = {}) {
    const args = {method, url};
    if (options.headers) args.headers = options.headers;
    if (options.body) args.body = options.body;
    const data = _goru_call("http_request", args);
    return new HTTPResponse(data);
  },

  get(url, options = {}) {
    return this.request("GET", url, options);
  },

  post(url, options = {}) {
    return this.request("POST", url, options);
  },

  put(url, options = {}) {
    return this.request("PUT", url, options);
  },

  patch(url, options = {}) {
    return this.request("PATCH", url, options);
  },

  delete(url, options = {}) {
    return this.request("DELETE", url, options);
  },

  async asyncRequest(method, url, options = {}) {
    const args = {method, url};
    if (options.headers) args.headers = options.headers;
    if (options.body) args.body = options.body;
    const data = await _asyncCall("http_request", args);
    return new HTTPResponse(data);
  },

  async asyncGet(url, options = {}) {
    return await this.asyncRequest("GET", url, options);
  },

  async asyncPost(url, options = {}) {
    return await this.asyncRequest("POST", url, options);
  },

  async asyncPut(url, options = {}) {
    return await this.asyncRequest("PUT", url, options);
  },

  async asyncPatch(url, options = {}) {
    return await this.asyncRequest("PATCH", url, options);
  },

  async asyncDelete(url, options = {}) {
    return await this.asyncRequest("DELETE", url, options);
  }
};

// =============================================================================
// fs - Filesystem
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
// time
// =============================================================================

const time_now = () => _goru_call("time_now", {});
