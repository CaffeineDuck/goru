package executor

import (
	_ "embed"
)

//go:embed testdata/mock.wasm
var mockWasm []byte

// mockLanguage implements Language for testing executor logic
// without the overhead of real Python/JavaScript runtimes.
type mockLanguage struct{}

func (m *mockLanguage) Name() string {
	return "mock"
}

func (m *mockLanguage) Module() []byte {
	return mockWasm
}

func (m *mockLanguage) WrapCode(code string) string {
	return code
}

func (m *mockLanguage) Args(wrappedCode string) []string {
	return []string{"mock"}
}

func (m *mockLanguage) SessionInit() string {
	return ""
}

func newMockLanguage() *mockLanguage {
	return &mockLanguage{}
}
