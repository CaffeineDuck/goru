import sys as _sys, json as _json

def _goru_call(fn, args):
    _sys.stderr.write("\x00GORU:" + _json.dumps({"fn": fn, "args": args}) + "\x00")
    _sys.stderr.flush()
    resp = _json.loads(input())
    if "error" in resp:
        raise RuntimeError(resp["error"])
    return resp.get("data")

def http_get(url):
    return _goru_call("http_get", {"url": url})

def kv_get(key):
    return _goru_call("kv_get", {"key": key})

def kv_set(key, value):
    return _goru_call("kv_set", {"key": key, "value": value})

def kv_delete(key):
    return _goru_call("kv_delete", {"key": key})

# Filesystem functions (only available if mounts are configured)
def fs_read(path):
    """Read the contents of a file. Returns string content."""
    return _goru_call("fs_read", {"path": path})

def fs_write(path, content):
    """Write content to a file. Requires write permission on mount."""
    return _goru_call("fs_write", {"path": path, "content": content})

def fs_list(path):
    """List directory contents. Returns list of {name, is_dir, size}."""
    return _goru_call("fs_list", {"path": path})

def fs_exists(path):
    """Check if a path exists. Returns True/False."""
    return _goru_call("fs_exists", {"path": path})

def fs_mkdir(path):
    """Create a directory. Requires create permission on mount."""
    return _goru_call("fs_mkdir", {"path": path})

def fs_remove(path):
    """Remove a file or empty directory. Requires write permission."""
    return _goru_call("fs_remove", {"path": path})

def fs_stat(path):
    """Get file/directory info. Returns {name, size, is_dir, mod_time}."""
    return _goru_call("fs_stat", {"path": path})
