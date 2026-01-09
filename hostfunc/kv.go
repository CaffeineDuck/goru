package hostfunc

import (
	"context"
	"errors"
	"sync"
)

type KVStore struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewKVStore() *KVStore {
	return &KVStore{data: make(map[string]string)}
}

func (s *KVStore) Get(ctx context.Context, args map[string]any) (any, error) {
	key, ok := args["key"].(string)
	if !ok {
		return nil, errors.New("key required")
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
	val, ok := args["value"].(string)
	if !ok {
		return nil, errors.New("value required")
	}

	s.mu.Lock()
	s.data[key] = val
	s.mu.Unlock()

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
