package hostfunc

import (
	"context"
	"sync"
	"testing"
)

func TestKVSetGet(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	_, err := kv.Set(ctx, map[string]any{"key": "foo", "value": "bar"})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := kv.Get(ctx, map[string]any{"key": "foo"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "bar" {
		t.Errorf("expected bar, got %v", val)
	}
}

func TestKVGetDefault(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	val, err := kv.Get(ctx, map[string]any{"key": "missing", "default": "fallback"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "fallback" {
		t.Errorf("expected fallback, got %v", val)
	}
}

func TestKVGetMissing(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	val, err := kv.Get(ctx, map[string]any{"key": "missing"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil, got %v", val)
	}
}

func TestKVDelete(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "foo", "value": "bar"})
	kv.Delete(ctx, map[string]any{"key": "foo"})

	val, _ := kv.Get(ctx, map[string]any{"key": "foo"})
	if val != nil {
		t.Errorf("expected nil after delete, got %v", val)
	}
}

func TestKVKeys(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "a", "value": 1})
	kv.Set(ctx, map[string]any{"key": "b", "value": 2})
	kv.Set(ctx, map[string]any{"key": "c", "value": 3})

	result, err := kv.Keys(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}

	keys := result.([]string)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestKVOverwrite(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "foo", "value": "original"})
	kv.Set(ctx, map[string]any{"key": "foo", "value": "updated"})

	val, _ := kv.Get(ctx, map[string]any{"key": "foo"})
	if val != "updated" {
		t.Errorf("expected updated, got %v", val)
	}
}

func TestKVAnyValue(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	tests := []struct {
		name  string
		value any
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []any{1, 2, 3}},
		{"map", map[string]any{"nested": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := kv.Set(ctx, map[string]any{"key": tt.name, "value": tt.value})
			if err != nil {
				t.Fatalf("Set %s failed: %v", tt.name, err)
			}

			val, err := kv.Get(ctx, map[string]any{"key": tt.name})
			if err != nil {
				t.Fatalf("Get %s failed: %v", tt.name, err)
			}
			if val == nil {
				t.Errorf("expected value for %s, got nil", tt.name)
			}
		})
	}
}

func TestKVKeyTooLarge(t *testing.T) {
	kv := NewKV(KVConfig{MaxKeySize: 10})
	ctx := context.Background()

	_, err := kv.Set(ctx, map[string]any{"key": "this-key-is-too-long", "value": "x"})
	if err == nil {
		t.Error("expected error for key too large")
	}
}

func TestKVValueTooLarge(t *testing.T) {
	kv := NewKV(KVConfig{MaxValueSize: 10})
	ctx := context.Background()

	_, err := kv.Set(ctx, map[string]any{"key": "k", "value": "this-value-is-way-too-large"})
	if err == nil {
		t.Error("expected error for value too large")
	}
}

func TestKVTooManyEntries(t *testing.T) {
	kv := NewKV(KVConfig{MaxEntries: 2})
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "a", "value": "1"})
	kv.Set(ctx, map[string]any{"key": "b", "value": "2"})

	_, err := kv.Set(ctx, map[string]any{"key": "c", "value": "3"})
	if err == nil {
		t.Error("expected error for too many entries")
	}
}

func TestKVConcurrent(t *testing.T) {
	kv := NewKV(DefaultKVConfig())
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + (n % 26)))
			kv.Set(ctx, map[string]any{"key": key, "value": n})
			kv.Get(ctx, map[string]any{"key": key})
		}(i)
	}
	wg.Wait()
}
