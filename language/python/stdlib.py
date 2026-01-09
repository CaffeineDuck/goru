import sys as _sys, json as _json, time as _time_module
from collections import deque as _deque

# =============================================================================
# Synchronous Host Function Protocol
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
# Async Support - WASIEventLoop and batched host calls
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
        # Send request with ID (non-blocking, just queues)
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

        # Read exactly 'count' responses
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
                    # Flush any pending async host calls
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

        def stop(self):
            pass

        def is_running(self):
            return self._running

        def is_closed(self):
            return self._closed

        def close(self):
            self._closed = True

        def get_debug(self):
            return False

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
            pass  # Silently ignore for now

    # Install as default event loop
    loop = WASIEventLoop()
    asyncio.set_event_loop(loop)
    return loop

def async_call(fn, args):
    """
    Queue an async host function call, return a Future.
    Use with: result = await async_call("kv_get", {"key": "foo"})
    """
    import asyncio
    loop = asyncio.get_event_loop()
    future = loop.create_future()
    _batch.queue(fn, args, future)
    return future

def run_async(coro):
    """
    Run an async coroutine to completion.
    Usage: result = run_async(my_async_function())
    """
    loop = _init_async()
    return loop.run_until_complete(coro)

# =============================================================================
# Time Functions
# =============================================================================

def time_now():
    """Get current Unix timestamp (seconds since epoch) from host."""
    return _goru_call("time_now", {})

# Monkey-patch time.time to use real host time
_time_module.time = time_now

# =============================================================================
# Synchronous Host Functions
# =============================================================================

def http_get(url):
    """Fetch URL via HTTP GET. Returns {status, body}."""
    return _goru_call("http_get", {"url": url})

def kv_get(key):
    """Get value from KV store. Returns value or None."""
    return _goru_call("kv_get", {"key": key})

def kv_set(key, value):
    """Set value in KV store."""
    return _goru_call("kv_set", {"key": key, "value": value})

def kv_delete(key):
    """Delete key from KV store."""
    return _goru_call("kv_delete", {"key": key})

def fs_read(path):
    """Read file contents. Returns string."""
    return _goru_call("fs_read", {"path": path})

def fs_write(path, content):
    """Write content to file."""
    return _goru_call("fs_write", {"path": path, "content": content})

def fs_list(path):
    """List directory. Returns [{name, is_dir, size}, ...]."""
    return _goru_call("fs_list", {"path": path})

def fs_exists(path):
    """Check if path exists. Returns bool."""
    return _goru_call("fs_exists", {"path": path})

def fs_mkdir(path):
    """Create directory."""
    return _goru_call("fs_mkdir", {"path": path})

def fs_remove(path):
    """Remove file or empty directory."""
    return _goru_call("fs_remove", {"path": path})

def fs_stat(path):
    """Get file info. Returns {name, size, is_dir, mod_time}."""
    return _goru_call("fs_stat", {"path": path})

# =============================================================================
# Async Host Functions
# =============================================================================

async def async_http_get(url):
    """Async HTTP GET. Use with asyncio.gather() for concurrent requests."""
    return await async_call("http_get", {"url": url})

async def async_kv_get(key):
    """Async KV get."""
    return await async_call("kv_get", {"key": key})

async def async_kv_set(key, value):
    """Async KV set."""
    return await async_call("kv_set", {"key": key, "value": value})

async def async_kv_delete(key):
    """Async KV delete."""
    return await async_call("kv_delete", {"key": key})

async def async_fs_read(path):
    """Async file read."""
    return await async_call("fs_read", {"path": path})

async def async_fs_write(path, content):
    """Async file write."""
    return await async_call("fs_write", {"path": path, "content": content})

async def async_fs_list(path):
    """Async directory list."""
    return await async_call("fs_list", {"path": path})

async def async_fs_exists(path):
    """Async path exists check."""
    return await async_call("fs_exists", {"path": path})

async def async_fs_mkdir(path):
    """Async mkdir."""
    return await async_call("fs_mkdir", {"path": path})

async def async_fs_remove(path):
    """Async remove."""
    return await async_call("fs_remove", {"path": path})

async def async_fs_stat(path):
    """Async file stat."""
    return await async_call("fs_stat", {"path": path})
