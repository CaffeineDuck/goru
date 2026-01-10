// Package goru provides a WebAssembly-based sandbox for executing untrusted
// Python and JavaScript code safely.
//
// # Overview
//
// goru runs code in isolated WASM modules with zero default capabilities.
// Filesystem, network, and other system access must be explicitly enabled.
//
// # Basic Usage
//
//	exec, _ := executor.New(hostfunc.NewRegistry())
//	defer exec.Close()
//
//	// Stateless execution
//	result := exec.Run(ctx, python.New(), `print("hello")`)
//	fmt.Println(result.Output)
//
//	// Session with persistent state
//	session, _ := exec.NewSession(python.New())
//	session.Run(ctx, `x = 42`)
//	session.Run(ctx, `print(x)`)  // 42
//
// # Enabling Capabilities
//
//	// HTTP access
//	result := exec.Run(ctx, python.New(), code,
//	    executor.WithAllowedHosts([]string{"api.example.com"}))
//
//	// Filesystem access
//	result := exec.Run(ctx, python.New(), code,
//	    executor.WithMount("/data", "./input", hostfunc.MountReadOnly))
//
//	// Key-value store
//	result := exec.Run(ctx, python.New(), code,
//	    executor.WithKV())
//
// See the [executor], [hostfunc], [language/python], and [language/javascript]
// packages for detailed API documentation.
package goru
