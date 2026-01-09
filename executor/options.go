package executor

import (
	"time"

	"github.com/caffeineduck/goru/hostfunc"
)

// Option configures execution behavior.
type Option func(*runConfig)

type runConfig struct {
	timeout      time.Duration
	allowedHosts []string
	kvStore      *hostfunc.KVStore
}

func defaultRunConfig() runConfig {
	return runConfig{
		timeout: 30 * time.Second,
	}
}

// WithTimeout sets the maximum execution time.
func WithTimeout(d time.Duration) Option {
	return func(c *runConfig) {
		c.timeout = d
	}
}

// WithAllowedHosts sets the list of hosts that HTTP requests can access.
func WithAllowedHosts(hosts []string) Option {
	return func(c *runConfig) {
		c.allowedHosts = hosts
	}
}

// WithKVStore provides a custom KV store for persistence across runs.
func WithKVStore(kv *hostfunc.KVStore) Option {
	return func(c *runConfig) {
		c.kvStore = kv
	}
}

// ExecutorOption configures the Executor at creation time.
type ExecutorOption func(*executorConfig)

type executorConfig struct {
	diskCache  bool
	cacheDir   string
	precompile []Language // Languages to precompile at startup
}

func defaultExecutorConfig() executorConfig {
	return executorConfig{
		diskCache: false,
	}
}

// WithDiskCache enables persistent compilation cache for faster CLI startup.
// Optionally provide a custom directory; otherwise uses ~/.cache/goru or XDG_CACHE_HOME/goru.
//
// Examples:
//
//	executor.New(registry, executor.WithDiskCache())            // default dir
//	executor.New(registry, executor.WithDiskCache("/tmp/cache")) // custom dir
func WithDiskCache(dir ...string) ExecutorOption {
	return func(c *executorConfig) {
		c.diskCache = true
		if len(dir) > 0 && dir[0] != "" {
			c.cacheDir = dir[0]
		}
	}
}

// WithPrecompile compiles the specified languages at Executor creation time.
// This moves the compilation cost to startup rather than first execution.
func WithPrecompile(langs ...Language) ExecutorOption {
	return func(c *executorConfig) {
		c.precompile = langs
	}
}
