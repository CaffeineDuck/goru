package sandbox

import (
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
