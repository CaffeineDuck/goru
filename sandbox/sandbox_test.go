package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestBasicPrint(t *testing.T) {
	result := RunPython("print('hello')", DefaultOptions())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestComputation(t *testing.T) {
	result := RunPython("print(sum(range(10)))", DefaultOptions())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "45" {
		t.Errorf("expected '45', got %q", result.Output)
	}
}

func TestKVStore(t *testing.T) {
	result := RunPython(`
kv_set("test_key", "test_value")
print(kv_get("test_key"))
`, DefaultOptions())
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "test_value" {
		t.Errorf("expected 'test_value', got %q", result.Output)
	}
}

func TestHTTPBlockedByDefault(t *testing.T) {
	result := RunPython(`http_get("https://example.com")`, DefaultOptions())
	if result.Error == nil {
		t.Error("expected error for blocked HTTP")
	}
	if !strings.Contains(result.Output, "http not enabled") {
		t.Errorf("expected 'http not enabled' error, got %q", result.Output)
	}
}

func TestHTTPAllowedWithHost(t *testing.T) {
	opts := Options{
		Timeout:      30 * time.Second,
		AllowedHosts: []string{"httpbin.org"},
	}
	result := RunPython(`
resp = http_get("https://httpbin.org/get")
print(resp["status"])
`, opts)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "200" {
		t.Errorf("expected '200', got %q", result.Output)
	}
}

func TestHTTPWrongHostBlocked(t *testing.T) {
	opts := Options{
		Timeout:      30 * time.Second,
		AllowedHosts: []string{"httpbin.org"},
	}
	result := RunPython(`http_get("https://evil.com")`, opts)
	if result.Error == nil {
		t.Error("expected error for wrong host")
	}
	if !strings.Contains(result.Output, "host not allowed") {
		t.Errorf("expected 'host not allowed' error, got %q", result.Output)
	}
}

func TestFilesystemBlocked(t *testing.T) {
	result := RunPython(`open("/etc/passwd")`, DefaultOptions())
	if result.Error == nil {
		t.Error("expected error for filesystem access")
	}
	if !strings.Contains(result.Output, "No such file") {
		t.Errorf("expected 'No such file' error, got %q", result.Output)
	}
}

func TestOsSystemBlocked(t *testing.T) {
	result := RunPython(`import os; os.system("ls")`, DefaultOptions())
	if result.Error == nil {
		t.Error("expected error for os.system")
	}
	if !strings.Contains(result.Output, "no attribute") {
		t.Errorf("expected 'no attribute' error, got %q", result.Output)
	}
}

func TestTimeout(t *testing.T) {
	opts := Options{Timeout: 2 * time.Second}
	result := RunPython(`
i = 0
while True:
    print(i)
    i += 1
`, opts)
	if result.Error == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", result.Error)
	}
}

func TestDurationTracked(t *testing.T) {
	result := RunPython("print(1)", DefaultOptions())
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}
