package python

import (
	"strings"
	"testing"
)

func TestModuleEmbedded(t *testing.T) {
	lang := New()
	wasm := lang.Module()
	if len(wasm) == 0 {
		t.Fatal("WASM bytes not embedded")
	}
	if len(wasm) < 1000000 {
		t.Errorf("WASM too small: %d bytes", len(wasm))
	}
}

func TestStdlibContents(t *testing.T) {
	if len(stdlib) == 0 {
		t.Fatal("stdlib not embedded")
	}
	checks := []string{
		"_goru_call",
		"install_pkg",
		"_HTTPModule",
		"_FSModule",
		"run_async",
		"async_call",
	}
	for _, check := range checks {
		if !strings.Contains(stdlib, check) {
			t.Errorf("stdlib missing %q", check)
		}
	}
}

func TestSessionInit(t *testing.T) {
	lang := New()
	init := lang.SessionInit()
	if !strings.Contains(init, "_GORU_SESSION_MODE") {
		t.Error("SessionInit missing session mode flag")
	}
}

func TestWrapCode(t *testing.T) {
	lang := New()
	code := `print("hello")`
	wrapped := lang.WrapCode(code)
	if !strings.Contains(wrapped, code) {
		t.Error("WrapCode should include original code")
	}
}

func TestArgs(t *testing.T) {
	lang := New()
	args := lang.Args("test code")
	if len(args) == 0 {
		t.Error("Args should return non-empty slice")
	}
	if args[0] != "python" {
		t.Errorf("first arg should be 'python', got %q", args[0])
	}
}
