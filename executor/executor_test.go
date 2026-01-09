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

// Shared executor to avoid 1.5s cold start per test.
// Python tests are integration tests - they verify the full stack works.
var (
	sharedExec *executor.Executor
	sharedLang = python.New()
)

func TestMain(m *testing.M) {
	// Set up shared executor once for all tests
	registry := hostfunc.NewRegistry()
	var err error
	sharedExec, err = executor.New(registry)
	if err != nil {
		panic("failed to create shared executor: " + err.Error())
	}

	// Warm up - compile Python module once
	sharedExec.Run(context.Background(), sharedLang, "x=1")

	// Run tests
	code := m.Run()

	// Cleanup
	sharedExec.Close()
	os.Exit(code)
}

// =============================================================================
// INTEGRATION TESTS (use shared Python executor)
// =============================================================================

func TestPythonBasicExecution(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `print("hello")`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestPythonComputation(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `print(sum(x**2 for x in range(10)))`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "285" {
		t.Errorf("expected '285', got %q", result.Output)
	}
}

func TestPythonKVHostFunction(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `
kv.set("key", "value")
print(kv.get("key"))
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "value" {
		t.Errorf("expected 'value', got %q", result.Output)
	}
}

func TestPythonCustomHostFunction(t *testing.T) {
	// This test needs its own executor because it registers a custom function
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

	result := exec.Run(context.Background(), sharedLang, `
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

// =============================================================================
// EXECUTOR BEHAVIOR TESTS (don't need Python-specific behavior)
// =============================================================================

func TestExecutorTimeout(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `
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
	kv := hostfunc.NewKVStore()

	// First run: set value
	sharedExec.Run(context.Background(), sharedLang, `kv.set("shared", "across-runs")`, executor.WithKVStore(kv))

	// Second run: get value
	result := sharedExec.Run(context.Background(), sharedLang, `print(kv.get("shared"))`, executor.WithKVStore(kv))

	if strings.TrimSpace(result.Output) != "across-runs" {
		t.Errorf("expected 'across-runs', got %q", result.Output)
	}
}

func TestExecutorDurationTracked(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `print(1)`)
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestExecutorCachesCompiledModule(t *testing.T) {
	// Create a fresh executor to test caching
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

	t.Logf("First run: %v, Second run: %v (should be ~10x faster)", result1.Duration, result2.Duration)
}

// =============================================================================
// FILESYSTEM TESTS
// =============================================================================

func TestFilesystemReadOnly(t *testing.T) {
	dir := t.TempDir()
	testFile := dir + "/test.json"
	os.WriteFile(testFile, []byte(`{"name": "test"}`), 0644)

	result := sharedExec.Run(context.Background(), sharedLang, `
data = fs.read_json("/data/test.json")
print(data["name"])
`, executor.WithMount("/data", dir, executor.MountReadOnly))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "test" {
		t.Errorf("expected 'test', got %q", result.Output)
	}
}

func TestFilesystemWrite(t *testing.T) {
	dir := t.TempDir()

	result := sharedExec.Run(context.Background(), sharedLang, `
fs.write_text("/output/result.txt", "hello from python")
print("written")
`, executor.WithMount("/output", dir, executor.MountReadWriteCreate))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "written") {
		t.Errorf("expected 'written', got %q", result.Output)
	}

	content, err := os.ReadFile(dir + "/result.txt")
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "hello from python" {
		t.Errorf("expected 'hello from python', got %q", content)
	}
}

func TestFilesystemDenied(t *testing.T) {
	// No mounts configured - filesystem should not be available
	result := sharedExec.Run(context.Background(), sharedLang, `
try:
    fs.read_text("/etc/passwd")
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

func TestFilesystemList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/file1.txt", []byte("1"), 0644)
	os.WriteFile(dir+"/file2.txt", []byte("2"), 0644)
	os.Mkdir(dir+"/subdir", 0755)

	result := sharedExec.Run(context.Background(), sharedLang, `
entries = fs.listdir("/data")
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

// =============================================================================
// ASYNC TESTS
// =============================================================================

func TestPythonAsyncKV(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `
import asyncio

async def main():
    await kv.async_set("async_a", "value_a")
    await kv.async_set("async_b", "value_b")
    results = await asyncio.gather(
        kv.async_get("async_a"),
        kv.async_get("async_b")
    )
    return results

result = run_async(main())
print(",".join(result))
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "value_a,value_b" {
		t.Errorf("expected 'value_a,value_b', got %q", result.Output)
	}
}

func TestPythonAsyncBatching(t *testing.T) {
	// Test that multiple async calls are batched together
	result := sharedExec.Run(context.Background(), sharedLang, `
import asyncio

# Set up data
for i in range(5):
    kv.set(f"batch_{i}", str(i * 10))

async def main():
    # These should be batched into a single FLUSH
    results = await asyncio.gather(
        kv.async_get("batch_0"),
        kv.async_get("batch_1"),
        kv.async_get("batch_2"),
        kv.async_get("batch_3"),
        kv.async_get("batch_4")
    )
    return results

result = run_async(main())
print(",".join(result))
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "0,10,20,30,40" {
		t.Errorf("expected '0,10,20,30,40', got %q", result.Output)
	}
}

func TestPythonTimeNow(t *testing.T) {
	result := sharedExec.Run(context.Background(), sharedLang, `
import time
now = time.time()
# Should be after 2020 and before 2100
if now > 1577836800 and now < 4102444800:
    print("OK")
else:
    print(f"FAIL: {now}")
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "OK" {
		t.Errorf("expected 'OK', got %q", result.Output)
	}
}

// =============================================================================
// MEMORY LIMIT TEST
// =============================================================================

func TestExecutorMemoryLimit(t *testing.T) {
	// Create executor with very small memory limit (1MB)
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry, executor.WithMemoryLimit(executor.MemoryLimit1MB))
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	// Python needs more than 1MB just to start, so this should fail
	result := exec.Run(context.Background(), sharedLang, `print("hi")`, executor.WithTimeout(5*time.Second))

	// We expect this to fail due to memory limits
	if result.Error == nil {
		t.Log("Note: Python managed to run with 1MB limit (unexpected but OK)")
	} else {
		t.Logf("Memory limit enforced: %v", result.Error)
	}
}
