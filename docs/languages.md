# Languages

goru supports Python and JavaScript execution via WebAssembly.

| Language | Runtime | Best For |
|----------|---------|----------|
| [Python](python.md) | CPython 3.12 | Data processing, stdlib access, PyPI packages |
| [JavaScript](javascript.md) | QuickJS-NG | Fast startup, ES2023, bundled npm packages |

## Quick Comparison

| Feature | Python | JavaScript |
|---------|--------|------------|
| Package install | Runtime (`install_pkg`) | Bundle only |
| Async | `asyncio.gather()` | `runAsync()` |
| Stdlib | 175 modules | `std`, `os` |
| Type hints | `goru.pyi` | `goru.d.ts` |

See individual language docs for full API details and limitations.
