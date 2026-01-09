package hostfunc

import (
	"context"
	"errors"
	"sync"
)

const (
	DefaultMaxKeySize   = 1024        // 1KB max key
	DefaultMaxValueSize = 1024 * 1024 // 1MB max value
	DefaultMaxEntries   = 10000       // 10K entries max
)

type KVStore struct {
	data         map[string]string
	mu           sync.RWMutex
	maxKeySize   int
	maxValueSize int
	maxEntries   int
}

type KVOption func(*KVStore)

func WithMaxKeySize(size int) KVOption {
	return func(s *KVStore) { s.maxKeySize = size }
}

func WithMaxValueSize(size int) KVOption {
	return func(s *KVStore) { s.maxValueSize = size }
}

func WithMaxEntries(n int) KVOption {
	return func(s *KVStore) { s.maxEntries = n }
}

func NewKVStore(opts ...KVOption) *KVStore {
	s := &KVStore{
		data:         make(map[string]string),
		maxKeySize:   DefaultMaxKeySize,
		maxValueSize: DefaultMaxValueSize,
		maxEntries:   DefaultMaxEntries,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *KVStore) Get(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}
	if len(key) > s.maxKeySize {
		return nil, errors.New("key too large")
	}

	s.mu.RLock()
	val, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return nil, nil
	}
	return val, nil
}

func (s *KVStore) Set(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}
	if len(key) > s.maxKeySize {
		return nil, errors.New("key too large")
	}
	val, ok := args["value"].(string)
	if !ok {
		return nil, errors.New("value required")
	}
	if len(val) > s.maxValueSize {
		return nil, errors.New("value too large")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check entry limit (only for new keys)
	if _, exists := s.data[key]; !exists && len(s.data) >= s.maxEntries {
		return nil, errors.New("too many entries")
	}

	s.data[key] = val
	return "ok", nil
}

func (s *KVStore) Delete(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
	}

	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()

	return "ok", nil
}
