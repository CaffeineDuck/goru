package hostfunc

import (
	"context"
	"sync"
)

type Func func(ctx context.Context, args map[string]any) (any, error)

type Registry struct {
	mu    sync.RWMutex
	funcs map[string]Func
}

func NewRegistry() *Registry {
	return &Registry{funcs: make(map[string]Func)}
}

func (r *Registry) Register(name string, fn Func) {
	r.mu.Lock()
	r.funcs[name] = fn
	r.mu.Unlock()
}

func (r *Registry) Get(name string) (Func, bool) {
	r.mu.RLock()
	fn, ok := r.funcs[name]
	r.mu.RUnlock()
	return fn, ok
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}
