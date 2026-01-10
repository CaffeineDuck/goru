package hostfunc

import (
	"context"
	"sync"
)

// Func is the signature for host functions callable from sandboxed code.
// Functions receive a context and a map of arguments, returning a result or error.
type Func func(ctx context.Context, args map[string]any) (any, error)

// Registry holds registered host functions that can be called from sandboxed code.
type Registry struct {
	mu    sync.RWMutex
	funcs map[string]Func
}

// NewRegistry creates an empty host function registry.
func NewRegistry() *Registry {
	return &Registry{funcs: make(map[string]Func)}
}

// Register adds a host function to the registry.
// If a function with the same name exists, it is replaced.
func (r *Registry) Register(name string, fn Func) {
	r.mu.Lock()
	r.funcs[name] = fn
	r.mu.Unlock()
}

// Get retrieves a host function by name.
func (r *Registry) Get(name string) (Func, bool) {
	r.mu.RLock()
	fn, ok := r.funcs[name]
	r.mu.RUnlock()
	return fn, ok
}

// List returns the names of all registered functions.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}

// All returns a copy of all registered functions.
func (r *Registry) All() map[string]Func {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Func, len(r.funcs))
	for name, fn := range r.funcs {
		result[name] = fn
	}
	return result
}
