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

// Security tests

func TestKVKeyTooLarge(t *testing.T) {
	kv := NewKVStore(WithMaxKeySize(100))
	ctx := context.Background()

	longKey := string(make([]byte, 200))
	_, err := kv.Set(ctx, map[string]any{"key": longKey, "value": "test"})
	if err == nil {
		t.Error("expected large key to be rejected")
	}
	if err.Error() != "key too large" {
		t.Errorf("expected 'key too large' error, got %v", err)
	}
}

func TestKVValueTooLarge(t *testing.T) {
	kv := NewKVStore(WithMaxValueSize(100))
	ctx := context.Background()

	largeValue := string(make([]byte, 200))
	_, err := kv.Set(ctx, map[string]any{"key": "test", "value": largeValue})
	if err == nil {
		t.Error("expected large value to be rejected")
	}
	if err.Error() != "value too large" {
		t.Errorf("expected 'value too large' error, got %v", err)
	}
}

func TestKVTooManyEntries(t *testing.T) {
	kv := NewKVStore(WithMaxEntries(3))
	ctx := context.Background()

	// Add 3 entries (should succeed)
	for i := 0; i < 3; i++ {
		_, err := kv.Set(ctx, map[string]any{"key": string(rune('a' + i)), "value": "test"})
		if err != nil {
			t.Fatalf("unexpected error on entry %d: %v", i, err)
		}
	}

	// 4th entry should fail
	_, err := kv.Set(ctx, map[string]any{"key": "d", "value": "test"})
	if err == nil {
		t.Error("expected too many entries error")
	}
	if err.Error() != "too many entries" {
		t.Errorf("expected 'too many entries' error, got %v", err)
	}

	// Updating existing entry should still work
	_, err = kv.Set(ctx, map[string]any{"key": "a", "value": "updated"})
	if err != nil {
		t.Errorf("updating existing key should work: %v", err)
	}
}

func TestKVGetKeyTooLarge(t *testing.T) {
	kv := NewKVStore(WithMaxKeySize(100))
	ctx := context.Background()

	longKey := string(make([]byte, 200))
	_, err := kv.Get(ctx, map[string]any{"key": longKey})
	if err == nil {
		t.Error("expected large key to be rejected in Get")
	}
	if err.Error() != "key too large" {
		t.Errorf("expected 'key too large' error, got %v", err)
	}
}
