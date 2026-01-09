package javascript

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
)

func TestJavaScriptBasicExecution(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `console.log("hello")`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
}

func TestJavaScriptComputation(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `
const sum = [1,2,3,4,5].reduce((a,b) => a + b, 0);
console.log(sum);
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "15" {
		t.Errorf("expected '15', got %q", result.Output)
	}
}

func TestJavaScriptKVHostFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `
kv_set("key", "value");
console.log(kv_get("key"));
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "value" {
		t.Errorf("expected 'value', got %q", result.Output)
	}
}

func TestJavaScriptMultipleHostCalls(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `
for (let i = 0; i < 3; i++) {
    kv_set("k" + i, "v" + i);
}
const values = [0,1,2].map(i => kv_get("k" + i)).join(",");
console.log(values);
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "v0,v1,v2" {
		t.Errorf("expected 'v0,v1,v2', got %q", result.Output)
	}
}

func TestJavaScriptTimeout(t *testing.T) {
	registry := hostfunc.NewRegistry()
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `while(true){}`,
		executor.WithTimeout(2*time.Second))

	if result.Error == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", result.Error)
	}
}

func TestJavaScriptCustomHostFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("greet", func(ctx context.Context, args map[string]any) (any, error) {
		name := args["name"].(string)
		return "Hello, " + name + "!", nil
	})

	exec, err := executor.New(registry)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	result := exec.Run(context.Background(), New(), `
const greeting = _goru_call("greet", {name: "World"});
console.log(greeting);
`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if strings.TrimSpace(result.Output) != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", result.Output)
	}
}
