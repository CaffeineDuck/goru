// Package executor provides a WebAssembly-based code execution engine
// for running untrusted Python and JavaScript code in a secure sandbox.
//
// # Overview
//
// The executor manages WASM module compilation, caching, and execution.
// It supports both stateless execution (single Run call) and stateful
// sessions (multiple Run calls with persistent state).
//
// # Basic Usage
//
//	exec, err := executor.New(hostfunc.NewRegistry())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer exec.Close()
//
//	result := exec.Run(ctx, python.New(), `print("hello")`)
//	fmt.Println(result.Output)
//
// # Sessions
//
// Sessions maintain state across multiple executions:
//
//	session, err := exec.NewSession(python.New())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Close()
//
//	session.Run(ctx, `x = 42`)
//	session.Run(ctx, `print(x)`)  // Output: 42
//
// # Capabilities
//
// By default, sandboxed code has no access to filesystem, network, or
// other system resources. Enable capabilities explicitly:
//
//	session, _ := exec.NewSession(python.New(),
//	    executor.WithSessionAllowedHosts([]string{"api.example.com"}),
//	    executor.WithSessionMount("/data", "./input", hostfunc.MountReadOnly),
//	    executor.WithSessionKV(),
//	)
//
// # Language Interface
//
// To add support for a new language, implement the [Language] interface.
// See [github.com/caffeineduck/goru/language/python] for an example.
package executor
