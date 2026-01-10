package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

func TestSessionBasic(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `print("hello")`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected output to contain 'hello', got: %q", result.Output)
	}
}

func TestSessionStatePersists(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `x = 42`)
	if result.Error != nil {
		t.Fatalf("first run failed: %v", result.Error)
	}

	result = session.Run(context.Background(), `print(x)`)
	if result.Error != nil {
		t.Fatalf("second run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "42") {
		t.Errorf("expected output to contain '42', got: %q", result.Output)
	}
}

func TestSessionFunction(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
def greet(name):
    return f"Hello, {name}!"
`)
	if result.Error != nil {
		t.Fatalf("define function failed: %v", result.Error)
	}

	result = session.Run(context.Background(), `print(greet("World"))`)
	if result.Error != nil {
		t.Fatalf("call function failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("expected output to contain 'Hello, World!', got: %q", result.Output)
	}
}

func TestSessionError(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `raise ValueError("test error")`)
	if result.Error == nil {
		t.Fatal("expected error, got none")
	}

	if !strings.Contains(result.Error.Error(), "ValueError") {
		t.Errorf("expected error to contain 'ValueError', got: %v", result.Error)
	}
}

func TestSessionMultipleRuns(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	for i := 0; i < 5; i++ {
		result := session.Run(context.Background(), `print("iteration")`)
		if result.Error != nil {
			t.Fatalf("run %d failed: %v", i, result.Error)
		}
	}
}

func TestSessionClosedError(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.Close()

	result := session.Run(context.Background(), `print("test")`)
	if result.Error != ErrSessionClosed {
		t.Errorf("expected ErrSessionClosed, got: %v", result.Error)
	}
}

func TestSessionTimeout(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New(), WithSessionTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Use infinite loop since WASI doesn't support time.sleep
	result := session.Run(context.Background(), `
while True:
    pass
`)
	if result.Error == nil {
		t.Fatal("expected timeout error, got none")
	}

	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", result.Error)
	}
}

func TestSessionHostFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("get_value", func(ctx context.Context, args map[string]any) (any, error) {
		return "custom_value", nil
	})

	exec, err := New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
result = call("get_value")
print(result)
`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "custom_value") {
		t.Errorf("expected output to contain 'custom_value', got: %q", result.Output)
	}
}

func TestMultipleSessions(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session1, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session1: %v", err)
	}
	defer session1.Close()

	session2, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session2: %v", err)
	}
	defer session2.Close()

	session1.Run(context.Background(), `x = "session1"`)
	session2.Run(context.Background(), `x = "session2"`)

	result1 := session1.Run(context.Background(), `print(x)`)
	result2 := session2.Run(context.Background(), `print(x)`)

	if !strings.Contains(result1.Output, "session1") {
		t.Errorf("session1 should have x='session1', got: %q", result1.Output)
	}

	if !strings.Contains(result2.Output, "session2") {
		t.Errorf("session2 should have x='session2', got: %q", result2.Output)
	}
}

func TestSessionInstallPkgDisabled(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
try:
    install_pkg("requests")
    print("should not reach here")
except Exception as e:
    print(f"error: {e}")
`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "error:") {
		t.Errorf("expected error when install_pkg disabled, got: %q", result.Output)
	}
}

func TestSessionInstallPkgNotAllowed(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New(), WithAllowedPackages([]string{"requests"}))
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
try:
    install_pkg("dangerous-pkg")
    print("should not reach here")
except Exception as e:
    print(f"error: {e}")
`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "not allowed") {
		t.Errorf("expected 'not allowed' error, got: %q", result.Output)
	}
}

func TestSessionTimeNow(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
import time
t = time.time()
print(f"time: {t}")
`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "time:") {
		t.Errorf("expected time output, got: %q", result.Output)
	}
}

func TestSessionAsyncioRun(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	result := session.Run(context.Background(), `
import asyncio

async def hello():
    return "async works"

result = asyncio.run(hello())
print(result)
`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "async works") {
		t.Errorf("expected 'async works', got: %q", result.Output)
	}
}

func TestSessionReplModeExpression(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Test that expressions are auto-printed in REPL mode
	result := session.RunRepl(context.Background(), `1 + 1`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "2") {
		t.Errorf("expected REPL to output '2', got: %q", result.Output)
	}

	// Test that _ contains the last expression result
	result = session.RunRepl(context.Background(), `_`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "2") {
		t.Errorf("expected _ to be 2, got: %q", result.Output)
	}
}

func TestSessionReplModeStatement(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Statements should not auto-print
	result := session.RunRepl(context.Background(), `x = 42`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	// Empty output for assignment
	if strings.TrimSpace(result.Output) != "" {
		t.Errorf("expected no output for assignment, got: %q", result.Output)
	}

	// But the variable should be set
	result = session.RunRepl(context.Background(), `x`)
	if result.Error != nil {
		t.Fatalf("run failed: %v", result.Error)
	}

	if !strings.Contains(result.Output, "42") {
		t.Errorf("expected x to be 42, got: %q", result.Output)
	}
}

func TestSessionCheckComplete(t *testing.T) {
	exec, err := New(hostfunc.NewRegistry())
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	session, err := exec.NewSession(python.New())
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	tests := []struct {
		code     string
		complete bool
	}{
		{"1 + 1", true},
		{"x = 42", true},
		{"def foo():", false},
		{"def foo():\n    pass\n", true}, // trailing newline marks block complete
		{"if True:", false},
		{"if True:\n    pass\n", true},
		{"for i in range(10):", false},
		{"for i in range(10):\n    pass\n", true},
	}

	for _, tc := range tests {
		complete, err := session.CheckComplete(context.Background(), tc.code)
		if err != nil {
			t.Errorf("CheckComplete(%q) error: %v", tc.code, err)
			continue
		}
		if complete != tc.complete {
			t.Errorf("CheckComplete(%q) = %v, want %v", tc.code, complete, tc.complete)
		}
	}
}
