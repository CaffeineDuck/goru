package hostfunc

import "context"

type Func func(ctx context.Context, args map[string]any) (any, error)

type Registry struct {
	funcs map[string]Func
}

func NewRegistry() *Registry {
	return &Registry{funcs: make(map[string]Func)}
}

func (r *Registry) Register(name string, fn Func) {
	r.funcs[name] = fn
}

func (r *Registry) Get(name string) (Func, bool) {
	fn, ok := r.funcs[name]
	return fn, ok
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}
