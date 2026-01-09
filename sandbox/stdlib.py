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
