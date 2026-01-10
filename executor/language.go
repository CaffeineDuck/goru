// Package executor provides a language-agnostic WASM code execution engine.
package executor

// Language defines the interface for a WASM-based language runtime.
// Implement this interface to add support for new languages (Python, JavaScript, etc.)
type Language interface {
	// Name returns a unique identifier for this language (e.g., "python", "javascript").
	// Used as the cache key for compiled modules.
	Name() string

	// Module returns the WASM binary for the language interpreter.
	Module() []byte

	// WrapCode prepares user code for execution by prepending stdlib imports,
	// host function bindings, or any language-specific boilerplate.
	WrapCode(code string) string

	// Args returns the command-line arguments to pass to the WASM module.
	// For Python: []string{"python", "-c", code}
	// For QuickJS: []string{"qjs", "--std", "-e", code}
	Args(wrappedCode string) []string

	// SessionInit returns code to inject before stdlib for session mode.
	// This code sets a flag that the stdlib checks to enter session loop.
	SessionInit() string
}
