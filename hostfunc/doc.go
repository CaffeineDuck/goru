// Package hostfunc provides host function implementations for sandboxed WASM code.
//
// Host functions are Go functions that can be called from within sandboxed code,
// enabling controlled access to external resources like HTTP, filesystem, and
// key-value storage.
//
// # Overview
//
// Sandboxed code has no implicit access to system resources. Each capability
// must be explicitly enabled via a [Registry] and appropriate configuration.
//
// # Registry
//
// The [Registry] manages available host functions. Register custom functions
// or use the built-in helpers:
//
//	registry := hostfunc.NewRegistry()
//	registry.Register("my_func", func(ctx context.Context, args map[string]any) (any, error) {
//	    return "result", nil
//	})
//
// # Built-in Capabilities
//
// HTTP: Controlled network access via [HTTP] and [HTTPConfig].
//
//	http := hostfunc.NewHTTP(hostfunc.HTTPConfig{
//	    AllowedHosts: []string{"api.example.com"},
//	})
//	registry.Register("http_request", http.Request)
//
// Filesystem: Mount-based access via [FS], [Mount], and [MountMode].
//
//	fs := hostfunc.NewFS([]hostfunc.Mount{
//	    {VirtualPath: "/data", HostPath: "./input", Mode: hostfunc.MountReadOnly},
//	})
//	registry.Register("fs_read", fs.Read)
//
// Key-Value Store: In-memory storage via [KV] and [KVConfig].
//
//	kv := hostfunc.NewKV(hostfunc.DefaultKVConfig())
//	registry.Register("kv_get", kv.Get)
//	registry.Register("kv_set", kv.Set)
//
// # Security Model
//
// All host functions follow the principle of least privilege:
//   - HTTP requests are limited to explicitly allowed hosts
//   - Filesystem access is restricted to mounted paths with specific permissions
//   - All operations have configurable size limits to prevent resource exhaustion
//
// See the executor package for higher-level APIs that configure these
// capabilities automatically.
package hostfunc
