package hostfunc

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
)

// Default limits for key-value store.
const (
	DefaultMaxKVKeySize    = 256               // Maximum key size in bytes
	DefaultMaxKVValueSize  = 64 * 1024         // Maximum value size (64KB)
	DefaultMaxKVEntries    = 1000              // Maximum number of entries
	DefaultMaxKVTotalBytes = 10 * 1024 * 1024  // Maximum total storage (10MB)
)

// KV provides an in-memory key-value store for sandboxed code.
// Values are JSON-serializable and subject to configurable size limits.
type KV struct {
	data          sync.Map
	count         atomic.Int64
	totalBytes    atomic.Int64
	maxKeySize    int
	maxValueSize  int
	maxEntries    int
	maxTotalBytes int64
}

// KVConfig configures key-value store limits.
type KVConfig struct {
	MaxKeySize    int   // Maximum key size in bytes
	MaxValueSize  int   // Maximum value size in bytes
	MaxEntries    int   // Maximum number of entries
	MaxTotalBytes int64 // Maximum total storage in bytes
}

// DefaultKVConfig returns the default KV configuration.
func DefaultKVConfig() KVConfig {
	return KVConfig{
		MaxKeySize:    DefaultMaxKVKeySize,
		MaxValueSize:  DefaultMaxKVValueSize,
		MaxEntries:    DefaultMaxKVEntries,
		MaxTotalBytes: DefaultMaxKVTotalBytes,
	}
}

// NewKV creates a key-value store with the given configuration.
func NewKV(cfg KVConfig) *KV {
	if cfg.MaxKeySize <= 0 {
		cfg.MaxKeySize = DefaultMaxKVKeySize
	}
	if cfg.MaxValueSize <= 0 {
		cfg.MaxValueSize = DefaultMaxKVValueSize
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = DefaultMaxKVEntries
	}
	if cfg.MaxTotalBytes <= 0 {
		cfg.MaxTotalBytes = DefaultMaxKVTotalBytes
	}
	return &KV{
		maxKeySize:    cfg.MaxKeySize,
		maxValueSize:  cfg.MaxValueSize,
		maxEntries:    cfg.MaxEntries,
		maxTotalBytes: cfg.MaxTotalBytes,
	}
}

// Get retrieves a value by key. Args: key, default (optional).
func (k *KV) Get(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}
	if len(key) > k.maxKeySize {
		return nil, errors.New("key too large")
	}

	val, exists := k.data.Load(key)
	if !exists {
		if def, ok := args["default"]; ok {
			return def, nil
		}
		return nil, nil
	}
	return val, nil
}

// Set stores a value. Args: key, value.
func (k *KV) Set(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}
	if len(key) > k.maxKeySize {
		return nil, errors.New("key too large")
	}

	value, ok := args["value"]
	if !ok {
		return nil, errors.New("value required")
	}

	valueBytes, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("value not serializable")
	}
	valueSize := len(valueBytes)
	if valueSize > k.maxValueSize {
		return nil, errors.New("value too large")
	}

	entrySize := int64(len(key) + valueSize)

	oldVal, exists := k.data.Load(key)
	var oldSize int64
	if exists {
		if oldBytes, err := json.Marshal(oldVal); err == nil {
			oldSize = int64(len(key) + len(oldBytes))
		}
	}

	if !exists {
		if k.count.Load() >= int64(k.maxEntries) {
			return nil, errors.New("too many entries")
		}
	}

	newTotal := k.totalBytes.Load() - oldSize + entrySize
	if newTotal > k.maxTotalBytes {
		return nil, errors.New("kv store full")
	}

	k.data.Store(key, value)
	if !exists {
		k.count.Add(1)
	}
	k.totalBytes.Add(entrySize - oldSize)

	return "ok", nil
}

// Delete removes a key. Args: key.
func (k *KV) Delete(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}

	oldVal, exists := k.data.LoadAndDelete(key)
	if exists {
		k.count.Add(-1)
		if oldBytes, err := json.Marshal(oldVal); err == nil {
			k.totalBytes.Add(-int64(len(key) + len(oldBytes)))
		}
	}

	return "ok", nil
}

// Keys returns all keys in the store.
func (k *KV) Keys(ctx context.Context, args map[string]any) (any, error) {
	keys := make([]string, 0)
	k.data.Range(func(key, _ any) bool {
		if s, ok := key.(string); ok {
			keys = append(keys, s)
		}
		return true
	})
	return keys, nil
}
