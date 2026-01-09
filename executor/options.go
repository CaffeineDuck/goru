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
	mounts       []hostfunc.Mount
	// Security limits
	kvOptions        []hostfunc.KVOption
	httpMaxURLLength int
	httpMaxBodySize  int64
	fsOptions        []hostfunc.FSOption
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

// Mount permission modes (re-exported from hostfunc for convenience).
const (
	MountReadOnly        = hostfunc.MountReadOnly
	MountReadWrite       = hostfunc.MountReadWrite
	MountReadWriteCreate = hostfunc.MountReadWriteCreate
)

// WithMount adds a filesystem mount point with the specified permissions.
// The virtual path is what sandboxed code sees; host path is the actual location.
//
// Examples:
//
//	executor.WithMount("/data", "./input", executor.MountReadOnly)
//	executor.WithMount("/output", "./results", executor.MountReadWrite)
//	executor.WithMount("/workspace", "./work", executor.MountReadWriteCreate)
func WithMount(virtualPath, hostPath string, mode hostfunc.MountMode) Option {
	return func(c *runConfig) {
		c.mounts = append(c.mounts, hostfunc.Mount{
			VirtualPath: virtualPath,
			HostPath:    hostPath,
			Mode:        mode,
		})
	}
}

// Security limit options

// WithKVMaxKeySize sets the maximum key size for KV store operations.
func WithKVMaxKeySize(size int) Option {
	return func(c *runConfig) {
		c.kvOptions = append(c.kvOptions, hostfunc.WithMaxKeySize(size))
	}
}

// WithKVMaxValueSize sets the maximum value size for KV store operations.
func WithKVMaxValueSize(size int) Option {
	return func(c *runConfig) {
		c.kvOptions = append(c.kvOptions, hostfunc.WithMaxValueSize(size))
	}
}

// WithKVMaxEntries sets the maximum number of entries in the KV store.
func WithKVMaxEntries(n int) Option {
	return func(c *runConfig) {
		c.kvOptions = append(c.kvOptions, hostfunc.WithMaxEntries(n))
	}
}

// WithHTTPMaxURLLength sets the maximum URL length for HTTP requests.
func WithHTTPMaxURLLength(size int) Option {
	return func(c *runConfig) {
		c.httpMaxURLLength = size
	}
}

// WithHTTPMaxBodySize sets the maximum response body size for HTTP requests.
func WithHTTPMaxBodySize(size int64) Option {
	return func(c *runConfig) {
		c.httpMaxBodySize = size
	}
}

// WithFSMaxFileSize sets the maximum file size for read operations.
func WithFSMaxFileSize(size int64) Option {
	return func(c *runConfig) {
		c.fsOptions = append(c.fsOptions, hostfunc.WithMaxFileSize(size))
	}
}

// WithFSMaxWriteSize sets the maximum content size for write operations.
func WithFSMaxWriteSize(size int64) Option {
	return func(c *runConfig) {
		c.fsOptions = append(c.fsOptions, hostfunc.WithMaxWriteSize(size))
	}
}

// WithFSMaxPathLength sets the maximum path length for filesystem operations.
func WithFSMaxPathLength(length int) Option {
	return func(c *runConfig) {
		c.fsOptions = append(c.fsOptions, hostfunc.WithMaxPathLength(length))
	}
}

// ExecutorOption configures the Executor at creation time.
type ExecutorOption func(*executorConfig)

type executorConfig struct {
	diskCache        bool
	cacheDir         string
	precompile       []Language // Languages to precompile at startup
	memoryLimitPages uint32     // Max memory pages (each page = 64KB), 0 = default (4GB)
}

func defaultExecutorConfig() executorConfig {
	return executorConfig{
		diskCache:        false,
		memoryLimitPages: 0, // 0 means use wazero default (65536 pages = 4GB)
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

// WithMemoryLimit sets the maximum memory available to WASM modules.
// Each page is 64KB. Examples:
//   - WithMemoryLimit(16) = 1MB max
//   - WithMemoryLimit(256) = 16MB max
//   - WithMemoryLimit(1024) = 64MB max
//   - WithMemoryLimit(4096) = 256MB max
//
// Default is 0 (no limit, up to 4GB).
func WithMemoryLimit(pages uint32) ExecutorOption {
	return func(c *executorConfig) {
		c.memoryLimitPages = pages
	}
}

// Memory limit constants for convenience.
const (
	MemoryLimit1MB   uint32 = 16    // 1 MB
	MemoryLimit16MB  uint32 = 256   // 16 MB
	MemoryLimit64MB  uint32 = 1024  // 64 MB
	MemoryLimit256MB uint32 = 4096  // 256 MB
	MemoryLimit1GB   uint32 = 16384 // 1 GB
)
