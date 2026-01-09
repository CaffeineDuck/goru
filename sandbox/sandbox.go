package sandbox

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"time"

	"github.com/caffeineduck/goru/hostfunc"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed python.wasm
var pythonWasm []byte

//go:embed stdlib.py
var stdlibPy string

type Result struct {
	Output   string
	Duration time.Duration
	Error    error
}

type Config struct {
	Timeout      time.Duration
	AllowedHosts []string
	Registry     *hostfunc.Registry
	KVStore      *hostfunc.KVStore
}

func DefaultConfig() Config {
	return Config{
		Timeout: 30 * time.Second,
	}
}

func Run(code string, cfg Config) Result {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	registry := cfg.Registry
	if registry == nil {
		registry = hostfunc.NewRegistry()
	}

	kv := cfg.KVStore
	if kv == nil {
		kv = hostfunc.NewKVStore()
	}

	registry.Register("kv_get", kv.Get)
	registry.Register("kv_set", kv.Set)
	registry.Register("kv_delete", kv.Delete)
	registry.Register("http_get", hostfunc.NewHTTPGet(hostfunc.HTTPConfig{
		AllowedHosts: cfg.AllowedHosts,
	}))

	rtConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	var stdout bytes.Buffer
	stdinReader, stdinWriter := io.Pipe()
	protocol := newProtocolHandler(ctx, registry, stdinWriter)

	fullCode := stdlibPy + "\n" + code

	moduleConfig := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(protocol).
		WithStdin(stdinReader).
		WithArgs("python", "-c", fullCode).
		WithName("python")

	errCh := make(chan error, 1)
	go func() {
		_, err := rt.InstantiateWithConfig(ctx, pythonWasm, moduleConfig)
		stdinWriter.Close()
		errCh <- err
	}()

	err := <-errCh

	result := Result{
		Output:   stdout.String() + protocol.Stderr(),
		Duration: time.Since(start),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("timeout after %v", cfg.Timeout)
		} else {
			result.Error = fmt.Errorf("execution failed: %w", err)
		}
	}

	return result
}

// RunPython is an alias for backward compatibility
func RunPython(code string, opts Options) Result {
	return Run(code, Config{
		Timeout:      opts.Timeout,
		AllowedHosts: opts.AllowedHosts,
	})
}

// Options for backward compatibility
type Options struct {
	Timeout      time.Duration
	AllowedHosts []string
}

func DefaultOptions() Options {
	return Options{Timeout: 30 * time.Second}
}
