package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

func setupTestServer(t *testing.T) (*executor.Executor, *sessionManager, func()) {
	t.Helper()

	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}

	sessions := newSessionManager(15 * time.Minute)

	cleanup := func() {
		sessions.closeAll()
		exec.Close()
	}

	return exec, sessions, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected 'ok', got %q", w.Body.String())
	}
}

func TestCreateSession(t *testing.T) {
	exec, sessions, cleanup := setupTestServer(t)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createSessionRequest
		json.NewDecoder(r.Body).Decode(&req)

		lang := req.Lang
		if lang == "" {
			lang = "python"
		}

		sessionID, err := sessions.create(exec, getLanguage(lang, ""))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createSessionResponse{SessionID: sessionID})
	})

	body := bytes.NewBufferString(`{"lang": "python"}`)
	req := httptest.NewRequest(http.MethodPost, "/sessions", body)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp createSessionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
}

func TestSessionExecution(t *testing.T) {
	exec, sessions, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session
	sessionID, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session, ok := sessions.get(sessionID)
	if !ok {
		t.Fatal("session not found after creation")
	}

	// Execute code
	result := session.Run(t.Context(), `x = 42`)
	if result.Error != nil {
		t.Fatalf("first run failed: %v", result.Error)
	}

	// Verify state persists
	result = session.Run(t.Context(), `print(x)`)
	if result.Error != nil {
		t.Fatalf("second run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "42") {
		t.Errorf("expected output to contain '42', got %q", result.Output)
	}
}

func TestSessionClose(t *testing.T) {
	exec, sessions, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a session
	sessionID, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Verify it exists
	_, ok := sessions.get(sessionID)
	if !ok {
		t.Fatal("session not found after creation")
	}

	// Close it
	closed := sessions.close(sessionID)
	if !closed {
		t.Error("expected close to return true")
	}

	// Verify it's gone
	_, ok = sessions.get(sessionID)
	if ok {
		t.Error("session should not exist after close")
	}

	// Close again should return false
	closed = sessions.close(sessionID)
	if closed {
		t.Error("expected close to return false for non-existent session")
	}
}

func TestSessionNotFound(t *testing.T) {
	_, sessions, cleanup := setupTestServer(t)
	defer cleanup()

	_, ok := sessions.get("nonexistent-session-id")
	if ok {
		t.Error("expected session not to be found")
	}
}

func TestMultipleSessions(t *testing.T) {
	exec, sessions, cleanup := setupTestServer(t)
	defer cleanup()

	// Create two sessions
	id1, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session 1: %v", err)
	}

	id2, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session 2: %v", err)
	}

	if id1 == id2 {
		t.Error("session IDs should be unique")
	}

	session1, _ := sessions.get(id1)
	session2, _ := sessions.get(id2)

	// Set different values in each session
	session1.Run(t.Context(), `x = "session1"`)
	session2.Run(t.Context(), `x = "session2"`)

	// Verify isolation
	result1 := session1.Run(t.Context(), `print(x)`)
	result2 := session2.Run(t.Context(), `print(x)`)

	if !strings.Contains(result1.Output, "session1") {
		t.Errorf("session1 should have x='session1', got %q", result1.Output)
	}

	if !strings.Contains(result2.Output, "session2") {
		t.Errorf("session2 should have x='session2', got %q", result2.Output)
	}
}

// --- REPL Integration Tests ---

func TestREPLSessionWorkflow(t *testing.T) {
	// Tests the same workflow as REPL: create session, run multiple commands
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Simulate REPL commands
	commands := []struct {
		code     string
		wantErr  bool
		wantOut  string
	}{
		{`x = 10`, false, ""},
		{`y = 20`, false, ""},
		{`print(x + y)`, false, "30"},
		{`def double(n): return n * 2`, false, ""},
		{`print(double(x))`, false, "20"},
		{`invalid syntax!!!`, true, ""},
	}

	for i, cmd := range commands {
		result := session.Run(t.Context(), cmd.code)

		if cmd.wantErr && result.Error == nil {
			t.Errorf("command %d (%q): expected error, got none", i, cmd.code)
		}
		if !cmd.wantErr && result.Error != nil {
			t.Errorf("command %d (%q): unexpected error: %v", i, cmd.code, result.Error)
		}
		if cmd.wantOut != "" && !strings.Contains(result.Output, cmd.wantOut) {
			t.Errorf("command %d (%q): expected output %q, got %q", i, cmd.code, cmd.wantOut, result.Output)
		}
	}
}

func TestREPLMultilineCode(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Define a multi-line function
	result := session.Run(t.Context(), `
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)
`)
	if result.Error != nil {
		t.Fatalf("failed to define function: %v", result.Error)
	}

	// Use it
	result = session.Run(t.Context(), `print(fibonacci(10))`)
	if result.Error != nil {
		t.Fatalf("failed to call function: %v", result.Error)
	}

	if !strings.Contains(result.Output, "55") {
		t.Errorf("expected fibonacci(10)=55, got %q", result.Output)
	}
}

func TestREPLImports(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Import persists
	result := session.Run(t.Context(), `import json`)
	if result.Error != nil {
		t.Fatalf("import failed: %v", result.Error)
	}

	result = session.Run(t.Context(), `data = json.dumps({"key": "value"})`)
	if result.Error != nil {
		t.Fatalf("json.dumps failed: %v", result.Error)
	}

	result = session.Run(t.Context(), `print(data)`)
	if result.Error != nil {
		t.Fatalf("print failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, `"key"`) {
		t.Errorf("expected JSON output, got %q", result.Output)
	}
}

// --- Deps Command Tests ---

func TestDepsList(t *testing.T) {
	dir := t.TempDir()

	// Empty dir
	depsList(dir) // Should print "No packages installed."

	// Create fake packages
	os.MkdirAll(filepath.Join(dir, "requests"), 0755)
	os.MkdirAll(filepath.Join(dir, "pydantic"), 0755)
	os.MkdirAll(filepath.Join(dir, "__pycache__"), 0755)
	os.MkdirAll(filepath.Join(dir, "requests-2.28.0.dist-info"), 0755)

	// Should list packages (excluding __pycache__ and .dist-info)
	depsList(dir) // Should print requests, pydantic
}

func TestDepsRemove(t *testing.T) {
	dir := t.TempDir()

	// Create fake package
	pkgDir := filepath.Join(dir, "requests")
	distInfo := filepath.Join(dir, "requests-2.28.0.dist-info")
	os.MkdirAll(pkgDir, 0755)
	os.MkdirAll(distInfo, 0755)

	// Remove it
	depsRemove(dir, []string{"requests"})

	// Verify both dirs are gone
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Error("package dir should be removed")
	}
	if _, err := os.Stat(distInfo); !os.IsNotExist(err) {
		t.Error("dist-info dir should be removed")
	}
}

func TestDepsCacheClear(t *testing.T) {
	// Create temp .goru/cache
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	cacheDir := filepath.Join(".goru", "cache")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(filepath.Join(cacheDir, "test.whl"), []byte("test"), 0644)

	// Clear cache
	depsCacheClear()

	// Verify cache is gone
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("cache dir should be removed")
	}
}
