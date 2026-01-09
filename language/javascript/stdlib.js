// goru host function bindings for JavaScript
const _goru_call = (fn, args) => {
  std.err.puts("\x00GORU:" + JSON.stringify({fn, args}) + "\x00");
  std.err.flush();
  const resp = JSON.parse(std.in.getline());
  if (resp.error) {
    throw new Error(resp.error);
  }
  return resp.data;
};

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
