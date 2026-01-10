package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestCLIHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"goru",
		"WebAssembly",
		"python",
		"js",
		"run",
		"repl",
		"serve",
		"deps",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("help output should contain %q", phrase)
		}
	}
}

func TestCLIRunHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "run", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"--code",
		"--lang",
		"--timeout",
		"--kv",
		"--allow-host",
		"--mount",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("run help output should contain %q", phrase)
		}
	}
}

func TestCLIReplHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "repl", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"--lang",
		"--packages",
		"--kv",
		"--history",
		"Command history",
		"Line editing",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("repl help output should contain %q", phrase)
		}
	}
}

func TestCLIServeHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "serve", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"--port",
		"--lang",
		"--timeout",
		"/execute",
		"/sessions",
		"/health",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("serve help output should contain %q", phrase)
		}
	}
}

func TestCLIDepsHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "deps", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPhrases := []string{
		"install",
		"list",
		"remove",
		"cache",
		"Python",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("deps help output should contain %q", phrase)
		}
	}
}

func TestCLILanguageRequired(t *testing.T) {
	// Running with -c but no --lang should fail
	_, err := getLanguage("", "")
	if err == nil {
		t.Error("expected error when language not specified and no file extension")
	}
	if !strings.Contains(err.Error(), "language required") {
		t.Errorf("error should mention language required, got: %v", err)
	}
}

func TestCLILanguageAutoDetect(t *testing.T) {
	tests := []struct {
		filename string
		wantLang string
	}{
		{"script.py", "python"},
		{"script.js", "javascript"},
		{"script.mjs", "javascript"},
		{"SCRIPT.PY", "python"},
		{"SCRIPT.JS", "javascript"},
	}

	for _, tc := range tests {
		lang, err := getLanguage("", tc.filename)
		if err != nil {
			t.Errorf("getLanguage(%q, %q) error: %v", "", tc.filename, err)
			continue
		}
		if lang == nil {
			t.Errorf("getLanguage(%q, %q) returned nil", "", tc.filename)
			continue
		}
		name := lang.Name()
		if !strings.EqualFold(name, tc.wantLang) {
			t.Errorf("getLanguage(%q, %q) = %q, want %q", "", tc.filename, name, tc.wantLang)
		}
	}
}

func TestCLILanguageExplicit(t *testing.T) {
	tests := []struct {
		langFlag string
		wantLang string
	}{
		{"python", "python"},
		{"py", "python"},
		{"js", "javascript"},
		{"javascript", "javascript"},
	}

	for _, tc := range tests {
		lang, err := getLanguage(tc.langFlag, "")
		if err != nil {
			t.Errorf("getLanguage(%q, %q) error: %v", tc.langFlag, "", err)
			continue
		}
		if lang == nil {
			t.Errorf("getLanguage(%q, %q) returned nil", tc.langFlag, "")
			continue
		}
		name := lang.Name()
		if !strings.Contains(strings.ToLower(name), strings.ToLower(tc.wantLang)) {
			t.Errorf("getLanguage(%q, %q) = %q, want %q", tc.langFlag, "", name, tc.wantLang)
		}
	}
}

func TestCLIUnknownLanguage(t *testing.T) {
	_, err := getLanguage("ruby", "")
	if err == nil {
		t.Error("expected error for unknown language")
	}
	if !strings.Contains(err.Error(), "unknown language") {
		t.Errorf("error should mention unknown language, got: %v", err)
	}
}

func TestCLIMountParsing(t *testing.T) {
	tests := []struct {
		spec    string
		wantErr bool
	}{
		{"/data:./input:ro", false},
		{"/data:./input:rw", false},
		{"/data:./input:rwc", false},
		{"/data:./input", true},       // missing mode
		{"/data:./input:bad", true},   // invalid mode
		{"invalid", true},             // no colons
	}

	for _, tc := range tests {
		_, err := parseMount(tc.spec)
		if tc.wantErr && err == nil {
			t.Errorf("parseMount(%q) should error", tc.spec)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("parseMount(%q) unexpected error: %v", tc.spec, err)
		}
	}
}

func TestCLIDepsListEmpty(t *testing.T) {
	dir := t.TempDir()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set the pkg dir and run
	depsPkgDir = dir
	runDepsList(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "No packages") {
		t.Errorf("expected 'No packages' message, got: %q", output)
	}
}

func TestCLIDepsListWithPackages(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "requests"), 0755)
	os.MkdirAll(filepath.Join(dir, "pydantic"), 0755)
	os.MkdirAll(filepath.Join(dir, "__pycache__"), 0755)
	os.MkdirAll(filepath.Join(dir, "requests.dist-info"), 0755)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	depsPkgDir = dir
	runDepsList(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "requests") {
		t.Errorf("expected 'requests' in output, got: %q", output)
	}
	if !strings.Contains(output, "pydantic") {
		t.Errorf("expected 'pydantic' in output, got: %q", output)
	}
	// Should not list __pycache__ or .dist-info
	if strings.Contains(output, "__pycache__") {
		t.Errorf("should not list __pycache__, got: %q", output)
	}
}

func TestCLIDepsRemove(t *testing.T) {
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "requests")
	distInfo := filepath.Join(dir, "requests-2.28.0.dist-info")
	os.MkdirAll(pkgPath, 0755)
	os.MkdirAll(distInfo, 0755)

	depsPkgDir = dir
	runDepsRemove(nil, []string{"requests"})

	if _, err := os.Stat(pkgPath); !os.IsNotExist(err) {
		t.Error("package directory should be removed")
	}
	if _, err := os.Stat(distInfo); !os.IsNotExist(err) {
		t.Error("dist-info directory should be removed")
	}
}

func TestCLIDepsCacheClear(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cacheDir := filepath.Join(".goru", "cache")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(filepath.Join(cacheDir, "test.whl"), []byte("test"), 0644)

	runDepsCacheClear(nil, nil)

	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("cache directory should be removed")
	}
}

func TestCLICompletionCommands(t *testing.T) {
	// Verify completion subcommand exists
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completion command should exist (provided by cobra)")
	}
}
