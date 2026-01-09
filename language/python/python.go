// Package python provides the Python language adapter for goru.
package python

import (
	_ "embed"
)

//go:embed python.wasm
var wasmModule []byte

//go:embed stdlib.py
var stdlib string

// Python implements the executor.Language interface for Python execution.
type Python struct{}

// New returns a Python language adapter.
func New() *Python {
	return &Python{}
}

// Name returns "python".
func (p *Python) Name() string {
	return "python"
}

// Module returns the RustPython WASM binary.
func (p *Python) Module() []byte {
	return wasmModule
}

// WrapCode prepends the goru stdlib to user code.
func (p *Python) WrapCode(code string) string {
	return stdlib + "\n" + code
}

// Args returns the command-line arguments for the Python interpreter.
func (p *Python) Args(wrappedCode string) []string {
	return []string{"python", "-c", wrappedCode}
}
