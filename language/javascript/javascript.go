package javascript

import (
	_ "embed"
)

//go:generate go run ../../internal/tools/download https://github.com/quickjs-ng/quickjs/releases/download/v0.11.0/qjs-wasi.wasm javascript.wasm

//go:embed javascript.wasm
var wasmModule []byte

//go:embed stdlib.js
var stdlib string

type JavaScript struct{}

func New() *JavaScript {
	return &JavaScript{}
}

func (j *JavaScript) Name() string {
	return "javascript"
}

func (j *JavaScript) Module() []byte {
	return wasmModule
}

// WrapCode prepends the goru stdlib to user code.
func (j *JavaScript) WrapCode(code string) string {
	return stdlib + "\n" + code
}

// Args returns the command-line arguments for the QuickJS interpreter.
func (j *JavaScript) Args(wrappedCode string) []string {
	return []string{"qjs", "--std", "-e", wrappedCode}
}

// SessionInit returns code to set session mode flag for JavaScript.
func (j *JavaScript) SessionInit() string {
	return "globalThis._GORU_SESSION_MODE = true;\n"
}
