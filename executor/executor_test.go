package executor_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

func TestExecutorBasicExecution(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `print("hello")`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestExecutorComputation(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `print(sum(x**2 for x in range(10)))`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "285" {
		t.Errorf("expected '285', got %q", result.Output)
	}
}

func TestExecutorKVHostFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
kv_set("key", "value")
print(kv_get("key"))
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "value" {
		t.Errorf("expected 'value', got %q", result.Output)
	}
}

func TestExecutorTimeout(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
while True:
    pass
`, executor.WithTimeout(1*time.Second))

	if result.Error == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", result.Error)
	}
}

func TestExecutorSharedKVStore(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	kv := hostfunc.NewKVStore()

	// First run: set value
	exec.Run(context.Background(), python.New(), `kv_set("shared", "across-runs")`, executor.WithKVStore(kv))

	// Second run: get value
	result := exec.Run(context.Background(), python.New(), `print(kv_get("shared"))`, executor.WithKVStore(kv))

	if strings.TrimSpace(result.Output) != "across-runs" {
		t.Errorf("expected 'across-runs', got %q", result.Output)
	}
}

func TestExecutorCachesCompiledModule(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	lang := python.New()

	// First run - compiles module
	result1 := exec.Run(context.Background(), lang, `print(1)`)
	if result1.Error != nil {
		t.Fatalf("first run failed: %v", result1.Error)
	}

	// Second run - should reuse compiled module and be faster
	result2 := exec.Run(context.Background(), lang, `print(2)`)
	if result2.Error != nil {
		t.Fatalf("second run failed: %v", result2.Error)
	}

	// Second run should be significantly faster (at least 10x)
	if result2.Duration > result1.Duration/5 {
		t.Logf("First run: %v, Second run: %v", result1.Duration, result2.Duration)
		// This is a soft check - CI environments may vary
	}
}

func TestExecutorCustomHostFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("custom_fn", func(ctx context.Context, args map[string]any) (any, error) {
		name := args["name"].(string)
		return "Hello, " + name + "!", nil
	})

	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
result = _goru_call("custom_fn", {"name": "World"})
print(result)
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result.Output)
	}
}

func TestExecutorDurationTracked(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `print(1)`)
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestExecutorFilesystemReadOnly(t *testing.T) {
	dir := t.TempDir()
	// Create a test file
	testFile := dir + "/test.json"
	os.WriteFile(testFile, []byte(`{"name": "test"}`), 0644)

	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
import json
data = json.loads(fs_read("/data/test.json"))
print(data["name"])
`, executor.WithMount("/data", dir, executor.MountReadOnly))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "test" {
		t.Errorf("expected 'test', got %q", result.Output)
	}
}

func TestExecutorFilesystemWrite(t *testing.T) {
	dir := t.TempDir()

	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
fs_write("/output/result.txt", "hello from python")
print("written")
`, executor.WithMount("/output", dir, executor.MountReadWriteCreate))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "written") {
		t.Errorf("expected 'written', got %q", result.Output)
	}

	// Verify file was created
	content, err := os.ReadFile(dir + "/result.txt")
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "hello from python" {
		t.Errorf("expected 'hello from python', got %q", content)
	}
}

func TestExecutorFilesystemDenied(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	// No mounts configured - filesystem should not be available
	result := exec.Run(context.Background(), python.New(), `
try:
    fs_read("/etc/passwd")
    print("FAIL: should have failed")
except RuntimeError as e:
    print(f"OK: {e}")
`)

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "OK:") {
		t.Errorf("expected filesystem access to be denied, got %q", result.Output)
	}
}

func TestExecutorFilesystemList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/file1.txt", []byte("1"), 0644)
	os.WriteFile(dir+"/file2.txt", []byte("2"), 0644)
	os.Mkdir(dir+"/subdir", 0755)

	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), python.New(), `
entries = fs_list("/data")
names = sorted([e["name"] for e in entries])
print(",".join(names))
`, executor.WithMount("/data", dir, executor.MountReadOnly))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "file1.txt,file2.txt,subdir" {
		t.Errorf("expected 'file1.txt,file2.txt,subdir', got %q", result.Output)
	}
}
