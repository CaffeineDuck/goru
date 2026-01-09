package javascript

import (
	_ "embed"

	quickjswasi "github.com/paralin/go-quickjs-wasi"
)

//go:embed stdlib.js
var stdlib string

// JavaScript implements the executor.Language interface for JavaScript execution.
type JavaScript struct{}

// New returns a JavaScript language adapter.
func New() *JavaScript {
	return &JavaScript{}
}

// Name returns "javascript".
func (j *JavaScript) Name() string {
	return "javascript"
}

// Module returns the QuickJS WASM binary.
func (j *JavaScript) Module() []byte {
	return quickjswasi.QuickJSWASM
}

// WrapCode prepends the goru stdlib to user code.
func (j *JavaScript) WrapCode(code string) string {
	return stdlib + "\n" + code
}

// Args returns the command-line arguments for the QuickJS interpreter.
func (j *JavaScript) Args(wrappedCode string) []string {
	return []string{"qjs", "--std", "-e", wrappedCode}
}
