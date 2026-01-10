package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/caffeineduck/goru/hostfunc"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Result holds the output and metadata from code execution.
type Result struct {
	Output   string
	Duration time.Duration
	Error    error
}

// Executor manages WASM runtimes and compiled module caching.
type Executor struct {
	runtime  wazero.Runtime
	cache    wazero.CompilationCache
	compiled map[string]wazero.CompiledModule
	registry *hostfunc.Registry
	mu       sync.RWMutex
	closed   bool
}

// New creates an Executor with the given host function registry.
func New(registry *hostfunc.Registry, opts ...ExecutorOption) (*Executor, error) {
	cfg := defaultExecutorConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx := context.Background()

	var cache wazero.CompilationCache
	var err error

	if cfg.diskCache {
		cacheDir := cfg.cacheDir
		if cacheDir == "" {
			cacheDir = defaultCacheDir()
		}
		cache, err = wazero.NewCompilationCacheWithDir(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("create disk cache: %w", err)
		}
	}

	rtConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	if cache != nil {
		rtConfig = rtConfig.WithCompilationCache(cache)
	}
	if cfg.memoryLimitPages > 0 {
		rtConfig = rtConfig.WithMemoryLimitPages(cfg.memoryLimitPages)
	}

	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		if cache != nil {
			cache.Close(ctx)
		}
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	e := &Executor{
		runtime:  rt,
		cache:    cache,
		compiled: make(map[string]wazero.CompiledModule),
		registry: registry,
	}

	for _, lang := range cfg.precompile {
		if _, err := e.getCompiled(ctx, lang); err != nil {
			e.Close()
			return nil, fmt.Errorf("precompile %s: %w", lang.Name(), err)
		}
	}

	return e, nil
}

// Run executes code in the specified language.
func (e *Executor) Run(ctx context.Context, lang Language, code string, opts ...Option) Result {
	start := time.Now()

	cfg := defaultRunConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}

	compiled, err := e.getCompiled(ctx, lang)
	if err != nil {
		return Result{Error: err, Duration: time.Since(start)}
	}

	registry := e.registry
	if registry == nil {
		registry = hostfunc.NewRegistry()
	}

	registry.Register("time_now", func(ctx context.Context, args map[string]any) (any, error) {
		return float64(time.Now().UnixNano()) / 1e9, nil
	})

	if cfg.kvEnabled {
		kv := hostfunc.NewKV(cfg.kvConfig)
		registry.Register("kv_get", kv.Get)
		registry.Register("kv_set", kv.Set)
		registry.Register("kv_delete", kv.Delete)
		registry.Register("kv_keys", kv.Keys)
	}

	if len(cfg.httpConfig.AllowedHosts) > 0 {
		httpHandler := hostfunc.NewHTTP(cfg.httpConfig)
		registry.Register("http_request", httpHandler.Request)
		registry.Register("http_get", hostfunc.NewHTTPGet(cfg.httpConfig))
	}

	if len(cfg.mounts) > 0 {
		fs := hostfunc.NewFS(cfg.mounts, cfg.fsOptions...)
		registry.Register("fs_read", fs.Read)
		registry.Register("fs_write", fs.Write)
		registry.Register("fs_list", fs.List)
		registry.Register("fs_exists", fs.Exists)
		registry.Register("fs_mkdir", fs.Mkdir)
		registry.Register("fs_remove", fs.Remove)
		registry.Register("fs_stat", fs.Stat)
	}

	var stdout bytes.Buffer
	stdinReader, stdinWriter := io.Pipe()
	protocol := newProtocolHandler(ctx, registry, stdinWriter)

	wrappedCode := lang.WrapCode(code)
	args := lang.Args(wrappedCode)

	moduleConfig := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(protocol).
		WithStdin(stdinReader).
		WithArgs(args...).
		WithName("")

	errCh := make(chan error, 1)
	go func() {
		_, err := e.runtime.InstantiateModule(ctx, compiled, moduleConfig)
		stdinWriter.Close()
		errCh <- err
	}()

	err = <-errCh

	result := Result{
		Output:   stdout.String() + protocol.Stderr(),
		Duration: time.Since(start),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("timeout after %v", cfg.timeout)
		} else {
			result.Error = fmt.Errorf("execution failed: %w", err)
		}
	}

	return result
}

// getCompiled returns a cached compiled module, compiling if necessary.
func (e *Executor) getCompiled(ctx context.Context, lang Language) (wazero.CompiledModule, error) {
	name := lang.Name()

	e.mu.RLock()
	if compiled, ok := e.compiled[name]; ok {
		e.mu.RUnlock()
		return compiled, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	if compiled, ok := e.compiled[name]; ok {
		return compiled, nil
	}

	compiled, err := e.runtime.CompileModule(ctx, lang.Module())
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", name, err)
	}

	e.compiled[name] = compiled
	return compiled, nil
}

// Close releases all resources held by the Executor.
func (e *Executor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}
	e.closed = true

	ctx := context.Background()

	var errs []error
	if err := e.runtime.Close(ctx); err != nil {
		errs = append(errs, err)
	}
	if e.cache != nil {
		if err := e.cache.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func defaultCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "goru")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "goru")
	}
	return filepath.Join(os.TempDir(), "goru-cache")
}
