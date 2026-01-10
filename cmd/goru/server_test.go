package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

		language, _ := getLanguage(lang, "")
		sessionID, err := sessions.create(exec, language)
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

	sessionID, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session, ok := sessions.get(sessionID)
	if !ok {
		t.Fatal("session not found after creation")
	}

	result := session.Run(t.Context(), `x = 42`)
	if result.Error != nil {
		t.Fatalf("first run failed: %v", result.Error)
	}

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

	sessionID, err := sessions.create(exec, python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	_, ok := sessions.get(sessionID)
	if !ok {
		t.Fatal("session not found after creation")
	}

	closed := sessions.close(sessionID)
	if !closed {
		t.Error("expected close to return true")
	}

	_, ok = sessions.get(sessionID)
	if ok {
		t.Error("session should not exist after close")
	}

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

	session1.Run(t.Context(), `x = "session1"`)
	session2.Run(t.Context(), `x = "session2"`)

	result1 := session1.Run(t.Context(), `print(x)`)
	result2 := session2.Run(t.Context(), `print(x)`)

	if !strings.Contains(result1.Output, "session1") {
		t.Errorf("session1 should have x='session1', got %q", result1.Output)
	}

	if !strings.Contains(result2.Output, "session2") {
		t.Errorf("session2 should have x='session2', got %q", result2.Output)
	}
}

func TestREPLSessionWorkflow(t *testing.T) {
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

	commands := []struct {
		code    string
		wantErr bool
		wantOut string
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

	result := session.Run(t.Context(), `
def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n-1) + fibonacci(n-2)
`)
	if result.Error != nil {
		t.Fatalf("failed to define function: %v", result.Error)
	}

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
