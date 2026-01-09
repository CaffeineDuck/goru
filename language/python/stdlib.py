import sys as _sys, json as _json, time as _time_module
from collections import deque as _deque

# =============================================================================
# Internal: Synchronous Host Function Protocol
# =============================================================================

def _goru_call(fn, args):
    """Synchronous host function call - blocks until response."""
    _sys.stderr.write("\x00GORU:" + _json.dumps({"fn": fn, "args": args}) + "\x00")
    _sys.stderr.flush()
    resp = _json.loads(input())
    if "error" in resp:
        raise RuntimeError(resp["error"])
    return resp.get("data")

# =============================================================================
# Internal: Async Support - WASIEventLoop and batched host calls
# =============================================================================

class _AsyncBatch:
    """Manages batched async host function calls."""
    def __init__(self):
        self.pending = {}  # id -> Future
        self.next_id = 0

    def queue(self, fn, args, future):
        """Queue an async call, associate with future."""
        req_id = str(self.next_id)
        self.next_id += 1
        self.pending[req_id] = future
        _sys.stderr.write("\x00GORU:" + _json.dumps({"id": req_id, "fn": fn, "args": args}) + "\x00")
        _sys.stderr.flush()
        return req_id

    def flush(self):
        """Send FLUSH and read all responses, resolving futures."""
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
    """Queue an async host function call, return a Future."""
    import asyncio
    loop = asyncio.get_event_loop()
    future = loop.create_future()
    _batch.queue(fn, args, future)
    return future

def _init_async():
    """Initialize async support - import asyncio and set up event loop."""
    import asyncio

    class WASIEventLoop(asyncio.AbstractEventLoop):
        """Event loop for WASI that handles coroutines without socket support."""

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
    """Run an async coroutine to completion."""
    loop = _init_async()
    return loop.run_until_complete(coro)

# =============================================================================
# goru.kv - Key-Value Store Module
# =============================================================================

class _KVModule:
    """Key-value store with explicit methods."""

    def get(self, key, *, default=None):
        """Get value by key. Returns default if not found."""
        result = _goru_call("kv_get", {"key": key})
        return result if result is not None else default

    def set(self, key, value):
        """Set a key-value pair."""
        return _goru_call("kv_set", {"key": key, "value": value})

    def delete(self, key):
        """Delete a key."""
        return _goru_call("kv_delete", {"key": key})

    async def async_get(self, key, *, default=None):
        """Async get value by key."""
        result = await _async_call("kv_get", {"key": key})
        return result if result is not None else default

    async def async_set(self, key, value):
        """Async set a key-value pair."""
        return await _async_call("kv_set", {"key": key, "value": value})

    async def async_delete(self, key):
        """Async delete a key."""
        return await _async_call("kv_delete", {"key": key})

kv = _KVModule()

# =============================================================================
# goru.http - HTTP Client Module
# =============================================================================

class _HTTPResponse:
    """HTTP response object with requests-like interface."""

    def __init__(self, data):
        self._data = data
        self.status_code = data.get("status", 0)
        self.text = data.get("body", "")

    @property
    def ok(self):
        """True if status code is 2xx."""
        return 200 <= self.status_code < 300

    def json(self):
        """Parse response body as JSON."""
        return _json.loads(self.text)

class _HTTPModule:
    """HTTP client with requests-like interface."""

    def get(self, url):
        """Sync HTTP GET request. Returns HTTPResponse."""
        data = _goru_call("http_get", {"url": url})
        return _HTTPResponse(data)

    async def async_get(self, url):
        """Async HTTP GET request. Returns HTTPResponse."""
        data = await _async_call("http_get", {"url": url})
        return _HTTPResponse(data)

http = _HTTPModule()

# =============================================================================
# goru.fs - Filesystem Module
# =============================================================================

class _FSModule:
    """Filesystem operations with pathlib-like interface."""

    def read_text(self, path):
        """Read file contents as string."""
        return _goru_call("fs_read", {"path": path})

    def read_json(self, path):
        """Read and parse JSON file."""
        return _json.loads(self.read_text(path))

    def write_text(self, path, content):
        """Write string to file."""
        return _goru_call("fs_write", {"path": path, "content": content})

    def write_json(self, path, data, *, indent=None):
        """Write data as JSON to file."""
        return self.write_text(path, _json.dumps(data, indent=indent))

    def listdir(self, path):
        """List directory contents. Returns list of entry dicts."""
        return _goru_call("fs_list", {"path": path})

    def exists(self, path):
        """Check if path exists."""
        return _goru_call("fs_exists", {"path": path})

    def mkdir(self, path):
        """Create directory."""
        return _goru_call("fs_mkdir", {"path": path})

    def remove(self, path):
        """Remove file or empty directory."""
        return _goru_call("fs_remove", {"path": path})

    def stat(self, path):
        """Get file info. Returns dict with name, size, is_dir, mod_time."""
        return _goru_call("fs_stat", {"path": path})

    # Async versions
    async def async_read_text(self, path):
        """Async read file contents."""
        return await _async_call("fs_read", {"path": path})

    async def async_read_json(self, path):
        """Async read and parse JSON file."""
        text = await self.async_read_text(path)
        return _json.loads(text)

    async def async_write_text(self, path, content):
        """Async write string to file."""
        return await _async_call("fs_write", {"path": path, "content": content})

    async def async_listdir(self, path):
        """Async list directory."""
        return await _async_call("fs_list", {"path": path})

    async def async_exists(self, path):
        """Async check if path exists."""
        return await _async_call("fs_exists", {"path": path})

    async def async_mkdir(self, path):
        """Async create directory."""
        return await _async_call("fs_mkdir", {"path": path})

    async def async_remove(self, path):
        """Async remove file or directory."""
        return await _async_call("fs_remove", {"path": path})

    async def async_stat(self, path):
        """Async get file info."""
        return await _async_call("fs_stat", {"path": path})

fs = _FSModule()

# =============================================================================
# Time - Monkey-patch time.time() for real host time
# =============================================================================

def time_now():
    """Get current Unix timestamp from host."""
    return _goru_call("time_now", {})

_time_module.time = time_now

