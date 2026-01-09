package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caffeineduck/goru/hostfunc"
)

// Integration tests - full Python execution
// Unit tests for individual components are in their respective packages

func TestPythonBasicExecution(t *testing.T) {
	result := Run("print('hello')", DefaultConfig())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestPythonComputation(t *testing.T) {
	result := Run("print(sum(x**2 for x in range(10)))", DefaultConfig())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "285" {
		t.Errorf("expected '285', got %q", result.Output)
	}
}

func TestPythonHostFunctionCall(t *testing.T) {
	result := Run(`
kv_set("key", "value")
print(kv_get("key"))
`, DefaultConfig())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "value" {
		t.Errorf("expected 'value', got %q", result.Output)
	}
}

func TestPythonHTTPWithAllowedHost(t *testing.T) {
	cfg := Config{
		Timeout:      30 * time.Second,
		AllowedHosts: []string{"httpbin.org"},
	}
	result := Run(`print(http_get("https://httpbin.org/get")["status"])`, cfg)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "200" {
		t.Errorf("expected '200', got %q", result.Output)
	}
}

func TestPythonTimeout(t *testing.T) {
	cfg := Config{Timeout: 2 * time.Second}
	result := Run(`
while True:
    print(".", end="", flush=True)
`, cfg)
	if result.Error == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", result.Error)
	}
}

func TestPythonKVIsolationAcrossRuns(t *testing.T) {
	kv := hostfunc.NewKVStore()
	cfg := Config{Timeout: 30 * time.Second, KVStore: kv}

	Run(`kv_set("persistent", "across-runs")`, cfg)
	result := Run(`print(kv_get("persistent"))`, cfg)

	if strings.TrimSpace(result.Output) != "across-runs" {
		t.Errorf("expected 'across-runs', got %q", result.Output)
	}
}

func TestPythonMultipleHostCalls(t *testing.T) {
	result := Run(`
for i in range(3):
    kv_set(f"k{i}", f"v{i}")
print(",".join(kv_get(f"k{i}") for i in range(3)))
`, DefaultConfig())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "v0,v1,v2" {
		t.Errorf("expected 'v0,v1,v2', got %q", result.Output)
	}
}

func TestPythonDurationTracked(t *testing.T) {
	result := Run("print(1)", DefaultConfig())
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

// --- Host function invocation verification ---

func TestHostFuncActuallyCalledWithCorrectArgs(t *testing.T) {
	var capturedFn string
	var capturedArgs map[string]any

	registry := hostfunc.NewRegistry()
	registry.Register("capture", func(ctx context.Context, args map[string]any) (any, error) {
		capturedFn = "capture"
		capturedArgs = args
		return "captured", nil
	})

	cfg := Config{Timeout: 30 * time.Second, Registry: registry}
	Run(`print(_goru_call("capture", {"foo": "bar", "num": 42}))`, cfg)

	if capturedFn != "capture" {
		t.Error("host function was not called")
	}
	if capturedArgs["foo"] != "bar" {
		t.Errorf("expected foo='bar', got %v", capturedArgs["foo"])
	}
	if capturedArgs["num"] != float64(42) { // JSON numbers are float64
		t.Errorf("expected num=42, got %v", capturedArgs["num"])
	}
}

func TestHTTPActuallyMakesRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test-path" {
			t.Errorf("expected path /test-path, got %s", r.URL.Path)
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"received": true}`))
	}))
	defer server.Close()

	// Extract host (127.0.0.1 or localhost)
	cfg := Config{
		Timeout:      30 * time.Second,
		AllowedHosts: []string{"127.0.0.1"},
	}

	result := Run(fmt.Sprintf(`
resp = http_get("%s/test-path")
print(resp["status"], resp["body"])
`, server.URL), cfg)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "201") {
		t.Errorf("expected status 201 in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "received") {
		t.Errorf("expected 'received' in output, got %q", result.Output)
	}
}

func TestKVPersistsAcrossMultipleCalls(t *testing.T) {
	kv := hostfunc.NewKVStore()
	cfg := Config{Timeout: 30 * time.Second, KVStore: kv}

	// Set from Python
	Run(`kv_set("from_python", "hello")`, cfg)

	// Verify in Go
	val, _ := kv.Get(context.Background(), map[string]any{"key": "from_python"})
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}

	// Set from Go
	kv.Set(context.Background(), map[string]any{"key": "from_go", "value": "world"})

	// Read from Python
	result := Run(`print(kv_get("from_go"))`, cfg)
	if strings.TrimSpace(result.Output) != "world" {
		t.Errorf("expected 'world', got %q", result.Output)
	}
}

func TestHostFuncErrorPropagates(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("fail", func(ctx context.Context, args map[string]any) (any, error) {
		return nil, errors.New("intentional failure")
	})

	cfg := Config{Timeout: 30 * time.Second, Registry: registry}
	result := Run(`
try:
    _goru_call("fail", {})
except RuntimeError as e:
    print(f"caught: {e}")
`, cfg)

	if !strings.Contains(result.Output, "caught: intentional failure") {
		t.Errorf("expected error to propagate, got %q", result.Output)
	}
}
