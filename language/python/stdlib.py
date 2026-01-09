import sys as _sys, json as _json, time as _time_module
from collections import deque as _deque

# =============================================================================
# Core: Host Function Protocol
# =============================================================================

def _goru_call(fn, args):
    _sys.stderr.write("\x00GORU:" + _json.dumps({"fn": fn, "args": args}) + "\x00")
    _sys.stderr.flush()
    resp = _json.loads(input())
    if "error" in resp:
        raise RuntimeError(resp["error"])
    return resp.get("data")

class _AsyncBatch:
    def __init__(self):
        self.pending = {}
        self.next_id = 0

    def queue(self, fn, args, future):
        req_id = str(self.next_id)
        self.next_id += 1
        self.pending[req_id] = future
        _sys.stderr.write("\x00GORU:" + _json.dumps({"id": req_id, "fn": fn, "args": args}) + "\x00")
        _sys.stderr.flush()
        return req_id

    def flush(self):
        if not self.pending:
            return
        count = len(self.pending)
        _sys.stderr.write(f"\x00GORU_FLUSH:{count}\x00")
        _sys.stderr.flush()
        for _ in range(count):
            line = input()
            resp = _json.loads(line)
            req_id = resp.get("id")
            if req_id in self.pending:
                future = self.pending.pop(req_id)
                if "error" in resp:
                    future.set_exception(RuntimeError(resp["error"]))
                else:
                    future.set_result(resp.get("data"))

_batch = _AsyncBatch()

def _async_call(fn, args):
    import asyncio
    loop = asyncio.get_event_loop()
    future = loop.create_future()
    _batch.queue(fn, args, future)
    return future

def _init_async():
    import asyncio

    class WASIEventLoop(asyncio.AbstractEventLoop):
        def __init__(self):
            self._ready = _deque()
            self._running = False
            self._closed = False

        def run_until_complete(self, future):
            import asyncio
            asyncio.events._set_running_loop(self)
            self._running = True
            future = asyncio.ensure_future(future, loop=self)
            try:
                while not future.done():
                    self._run_once()
                    _batch.flush()
            finally:
                self._running = False
                asyncio.events._set_running_loop(None)
            return future.result()

        def _run_once(self):
            while self._ready:
                handle = self._ready.popleft()
                if not handle._cancelled:
                    handle._run()

        def stop(self): pass
        def is_running(self): return self._running
        def is_closed(self): return self._closed
        def close(self): self._closed = True
        def get_debug(self): return False

        def create_future(self):
            import asyncio
            return asyncio.Future(loop=self)

        def create_task(self, coro, *, name=None, context=None):
            import asyncio
            return asyncio.Task(coro, loop=self, name=name, context=context)

        def call_soon(self, callback, *args, context=None):
            import asyncio
            handle = asyncio.Handle(callback, args, self, context)
            self._ready.append(handle)
            return handle

        def call_exception_handler(self, context):
            pass

    loop = WASIEventLoop()
    asyncio.set_event_loop(loop)
    return loop

def run_async(coro):
    loop = _init_async()
    return loop.run_until_complete(coro)

def call(fn, **kwargs):
    return _goru_call(fn, kwargs)

async def async_call(fn, **kwargs):
    return await _async_call(fn, kwargs)

# =============================================================================
# kv - Key-Value Store
# =============================================================================

class _KVModule:
    def get(self, key, *, default=None):
        result = _goru_call("kv_get", {"key": key})
        return result if result is not None else default

    def set(self, key, value):
        return _goru_call("kv_set", {"key": key, "value": value})

    def delete(self, key):
        return _goru_call("kv_delete", {"key": key})

    async def async_get(self, key, *, default=None):
        result = await _async_call("kv_get", {"key": key})
        return result if result is not None else default

    async def async_set(self, key, value):
        return await _async_call("kv_set", {"key": key, "value": value})

    async def async_delete(self, key):
        return await _async_call("kv_delete", {"key": key})

kv = _KVModule()

# =============================================================================
# http - HTTP Client
# =============================================================================

class HTTPResponse:
    def __init__(self, data):
        self._data = data
        self.status_code = data.get("status", 0)
        self.text = data.get("body", "")
        self.headers = data.get("headers", {})

    @property
    def ok(self):
        return 200 <= self.status_code < 300

    def json(self):
        return _json.loads(self.text)

class _HTTPModule:
    def request(self, method, url, *, headers=None, body=None):
        args = {"method": method, "url": url}
        if headers:
            args["headers"] = headers
        if body:
            args["body"] = body
        data = _goru_call("http_request", args)
        return HTTPResponse(data)

    def get(self, url, *, headers=None):
        return self.request("GET", url, headers=headers)

    def post(self, url, *, headers=None, body=None):
        return self.request("POST", url, headers=headers, body=body)

    def put(self, url, *, headers=None, body=None):
        return self.request("PUT", url, headers=headers, body=body)

    def patch(self, url, *, headers=None, body=None):
        return self.request("PATCH", url, headers=headers, body=body)

    def delete(self, url, *, headers=None):
        return self.request("DELETE", url, headers=headers)

    async def async_request(self, method, url, *, headers=None, body=None):
        args = {"method": method, "url": url}
        if headers:
            args["headers"] = headers
        if body:
            args["body"] = body
        data = await _async_call("http_request", args)
        return HTTPResponse(data)

    async def async_get(self, url, *, headers=None):
        return await self.async_request("GET", url, headers=headers)

    async def async_post(self, url, *, headers=None, body=None):
        return await self.async_request("POST", url, headers=headers, body=body)

    async def async_put(self, url, *, headers=None, body=None):
        return await self.async_request("PUT", url, headers=headers, body=body)

    async def async_patch(self, url, *, headers=None, body=None):
        return await self.async_request("PATCH", url, headers=headers, body=body)

    async def async_delete(self, url, *, headers=None):
        return await self.async_request("DELETE", url, headers=headers)

http = _HTTPModule()

# =============================================================================
# fs - Filesystem
# =============================================================================

class _FSModule:
    def read_text(self, path):
        return _goru_call("fs_read", {"path": path})

    def read_json(self, path):
        return _json.loads(self.read_text(path))

    def write_text(self, path, content):
        return _goru_call("fs_write", {"path": path, "content": content})

    def write_json(self, path, data, *, indent=None):
        return self.write_text(path, _json.dumps(data, indent=indent))

    def listdir(self, path):
        return _goru_call("fs_list", {"path": path})

    def exists(self, path):
        return _goru_call("fs_exists", {"path": path})

    def mkdir(self, path):
        return _goru_call("fs_mkdir", {"path": path})

    def remove(self, path):
        return _goru_call("fs_remove", {"path": path})

    def stat(self, path):
        return _goru_call("fs_stat", {"path": path})

    async def async_read_text(self, path):
        return await _async_call("fs_read", {"path": path})

    async def async_read_json(self, path):
        text = await self.async_read_text(path)
        return _json.loads(text)

    async def async_write_text(self, path, content):
        return await _async_call("fs_write", {"path": path, "content": content})

    async def async_listdir(self, path):
        return await _async_call("fs_list", {"path": path})

    async def async_exists(self, path):
        return await _async_call("fs_exists", {"path": path})

    async def async_mkdir(self, path):
        return await _async_call("fs_mkdir", {"path": path})

    async def async_remove(self, path):
        return await _async_call("fs_remove", {"path": path})

    async def async_stat(self, path):
        return await _async_call("fs_stat", {"path": path})

fs = _FSModule()

# =============================================================================
# time
# =============================================================================

def time_now():
    return _goru_call("time_now", {})

_time_module.time = time_now

# =============================================================================
# Patches
# =============================================================================

import asyncio as _asyncio
_asyncio.run = run_async
