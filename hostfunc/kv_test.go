package hostfunc

import (
	"context"
	"testing"
)

func TestKVSetGet(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	_, err := kv.Set(ctx, map[string]any{"key": "foo", "value": "bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, err := kv.Get(ctx, map[string]any{"key": "foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "bar" {
		t.Errorf("expected 'bar', got %v", val)
	}
}

func TestKVGetMissing(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	val, err := kv.Get(ctx, map[string]any{"key": "nonexistent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Errorf("expected nil for missing key, got %v", val)
	}
}

func TestKVDelete(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "foo", "value": "bar"})
	kv.Delete(ctx, map[string]any{"key": "foo"})

	val, _ := kv.Get(ctx, map[string]any{"key": "foo"})
	if val != nil {
		t.Errorf("expected nil after delete, got %v", val)
	}
}

func TestKVOverwrite(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	kv.Set(ctx, map[string]any{"key": "foo", "value": "first"})
	kv.Set(ctx, map[string]any{"key": "foo", "value": "second"})

	val, _ := kv.Get(ctx, map[string]any{"key": "foo"})
	if val != "second" {
		t.Errorf("expected 'second', got %v", val)
	}
}

func TestKVIsolation(t *testing.T) {
	kv1 := NewKVStore()
	kv2 := NewKVStore()
	ctx := context.Background()

	kv1.Set(ctx, map[string]any{"key": "shared", "value": "from-kv1"})
	kv2.Set(ctx, map[string]any{"key": "shared", "value": "from-kv2"})

	val1, _ := kv1.Get(ctx, map[string]any{"key": "shared"})
	val2, _ := kv2.Get(ctx, map[string]any{"key": "shared"})

	if val1 != "from-kv1" {
		t.Errorf("kv1 expected 'from-kv1', got %v", val1)
	}
	if val2 != "from-kv2" {
		t.Errorf("kv2 expected 'from-kv2', got %v", val2)
	}
}

func TestKVMissingKey(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	_, err := kv.Get(ctx, map[string]any{})
	if err == nil || err.Error() != "key required" {
		t.Errorf("expected 'key required', got %v", err)
	}
}

func TestKVSetMissingValue(t *testing.T) {
	kv := NewKVStore()
	ctx := context.Background()

	_, err := kv.Set(ctx, map[string]any{"key": "foo"})
	if err == nil || err.Error() != "value required" {
		t.Errorf("expected 'value required', got %v", err)
	}
}
