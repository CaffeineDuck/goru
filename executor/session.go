package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/caffeineduck/goru/hostfunc"
	"github.com/tetratelabs/wazero"
)

// Session errors.
var (
	ErrSessionClosed = errors.New("session closed")
	ErrSessionBusy   = errors.New("session busy")
)

// Session represents a stateful execution environment that maintains state
// across multiple Run calls. Create sessions via [Executor.NewSession].
type Session struct {
	exec     *Executor
	lang     Language
	cfg      sessionConfig
	registry *hostfunc.Registry

	stdin       *io.PipeWriter
	stdinReader *io.PipeReader
	stdout      *cappedOutput
	protocol    *sessionProtocol
	cancel      context.CancelFunc

	mu       sync.Mutex
	execMu   sync.Mutex
	closed   bool
	started  bool
	startErr error
}

type sessionConfig struct {
	timeout         time.Duration
	clockEnabled    bool
	mounts          []hostfunc.Mount
	fsOptions       []hostfunc.FSOption
	httpConfig      hostfunc.HTTPConfig
	kvEnabled       bool
	kvConfig        hostfunc.KVConfig
	packagesPath    string
	pkgInstall      bool
	allowedPackages []string
	env             map[string]string
}

func defaultSessionConfig() sessionConfig {
	return sessionConfig{
		timeout: 30 * time.Second,
		env:     make(map[string]string),
	}
}

// SessionOption configures session behavior.
type SessionOption func(*sessionConfig)

// WithSessionTimeout sets the maximum execution time per Run call.
func WithSessionTimeout(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.timeout = d
	}
}

// WithSessionAllowedHosts enables HTTP and sets allowed hosts.
func WithSessionAllowedHosts(hosts []string) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.AllowedHosts = hosts
	}
}

// WithSessionMount adds a filesystem mount point.
func WithSessionMount(virtualPath, hostPath string, mode hostfunc.MountMode) SessionOption {
	return func(c *sessionConfig) {
		c.mounts = append(c.mounts, hostfunc.Mount{
			VirtualPath: virtualPath,
			HostPath:    hostPath,
			Mode:        mode,
		})
	}
}

// WithPackages mounts a directory as /packages for Python imports.
func WithPackages(path string) SessionOption {
	return func(c *sessionConfig) {
		c.packagesPath = path
	}
}

// WithPackageInstall enables runtime package installation via install_pkg().
func WithPackageInstall(enabled bool) SessionOption {
	return func(c *sessionConfig) {
		c.pkgInstall = enabled
	}
}

// WithAllowedPackages enables package installation restricted to the specified packages.
func WithAllowedPackages(packages []string) SessionOption {
	return func(c *sessionConfig) {
		c.pkgInstall = true
		c.allowedPackages = packages
	}
}

// WithSessionHTTPTimeout sets the timeout for individual HTTP requests.
func WithSessionHTTPTimeout(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.RequestTimeout = d
	}
}

// WithSessionHTTPMaxURLLength sets the maximum URL length for HTTP requests.
func WithSessionHTTPMaxURLLength(size int) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.MaxURLLength = size
	}
}

// WithSessionHTTPMaxBodySize sets the maximum response body size for HTTP requests.
func WithSessionHTTPMaxBodySize(size int64) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.MaxBodySize = size
	}
}

// WithSessionKV enables the in-memory key-value store with default limits.
func WithSessionKV() SessionOption {
	return func(c *sessionConfig) {
		c.kvEnabled = true
		c.kvConfig = hostfunc.DefaultKVConfig()
	}
}

// WithSessionKVConfig enables the key-value store with custom limits.
func WithSessionKVConfig(cfg hostfunc.KVConfig) SessionOption {
	return func(c *sessionConfig) {
		c.kvEnabled = true
		c.kvConfig = cfg
	}
}

// WithSessionFSMaxFileSize sets the maximum file size for read operations.
func WithSessionFSMaxFileSize(size int64) SessionOption {
	return func(c *sessionConfig) {
		c.fsOptions = append(c.fsOptions, hostfunc.WithMaxFileSize(size))
	}
}

// NewSession creates a stateful execution session for the given language.
// Sessions maintain state across multiple Run calls and support capabilities
// like HTTP, filesystem access, and key-value storage.
func (e *Executor) NewSession(lang Language, opts ...SessionOption) (*Session, error) {
	cfg := defaultSessionConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	cfg.env["GORU_SESSION"] = "1"
	if cfg.clockEnabled {
		cfg.env["GORU_REAL_CLOCK"] = "1"
	}

	if cfg.packagesPath != "" {
		cfg.mounts = append(cfg.mounts, hostfunc.Mount{
			VirtualPath: "/packages",
			HostPath:    cfg.packagesPath,
			Mode:        hostfunc.MountReadOnly,
		})
		cfg.env["PYTHONPATH"] = "/packages"
	}

	registry := hostfunc.NewRegistry()
	if e.registry != nil {
		for name, fn := range e.registry.All() {
			registry.Register(name, fn)
		}
	}

	s := &Session{
		exec:     e,
		lang:     lang,
		cfg:      cfg,
		registry: registry,
	}

	if err := s.start(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Session) start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	compiled, err := s.exec.getCompiled(ctx, s.lang)
	if err != nil {
		cancel()
		s.startErr = err
		return err
	}

	s.registerHostFunctions()

	s.stdinReader, s.stdin = io.Pipe()
	s.stdout = &cappedOutput{}
	s.protocol = newSessionProtocol(ctx, s.registry, s.stdin)

	initCode := s.lang.SessionInit() + s.lang.WrapCode("")
	args := s.lang.Args(initCode)

	moduleConfig := wazero.NewModuleConfig().
		WithStdout(s.stdout).
		WithStderr(s.protocol).
		WithStdin(s.stdinReader).
		WithArgs(args...).
		WithName("")

	for k, v := range s.cfg.env {
		moduleConfig = moduleConfig.WithEnv(k, v)
	}

	exit := make(chan error, 1)
	go func() {
		_, err := s.exec.runtime.InstantiateModule(ctx, compiled, moduleConfig)
		s.stdinReader.Close()
		exit <- err
	}()

	select {
	case <-s.protocol.readyCh:
		s.started = true
		return nil
	case err := <-exit:
		s.startErr = fmt.Errorf("session exited before ready: %v", err)
	case <-s.protocol.failed:
		s.startErr = s.protocol.err
	case <-time.After(30 * time.Second):
		s.startErr = errors.New("session start timeout")
	}
	cancel()
	s.stdinReader.Close()
	s.stdin.Close()
	return s.startErr
}

func (s *Session) registerHostFunctions() {
	if s.cfg.clockEnabled {
		s.registry.Register("time_now", realTime)
	}

	if s.cfg.kvEnabled {
		kv := hostfunc.NewKV(s.cfg.kvConfig)
		s.registry.Register("kv_get", kv.Get)
		s.registry.Register("kv_set", kv.Set)
		s.registry.Register("kv_delete", kv.Delete)
		s.registry.Register("kv_keys", kv.Keys)
	}

	if len(s.cfg.httpConfig.AllowedHosts) > 0 {
		httpHandler := hostfunc.NewHTTP(s.cfg.httpConfig)
		s.registry.Register("http_request", httpHandler.Request)
	}

	if len(s.cfg.mounts) > 0 {
		fs := hostfunc.NewFS(s.cfg.mounts, s.cfg.fsOptions...)
		s.registry.Register("fs_read", fs.Read)
		s.registry.Register("fs_write", fs.Write)
		s.registry.Register("fs_list", fs.List)
		s.registry.Register("fs_exists", fs.Exists)
		s.registry.Register("fs_mkdir", fs.Mkdir)
		s.registry.Register("fs_remove", fs.Remove)
		s.registry.Register("fs_stat", fs.Stat)
	}

	if s.cfg.pkgInstall {
		pkgDir := s.cfg.packagesPath
		if pkgDir == "" {
			pkgDir = ".goru/python/packages"
		}
		installer := hostfunc.NewPkgInstaller(hostfunc.PkgConfig{
			PackageDir:      pkgDir,
			AllowedPackages: s.cfg.allowedPackages,
			Enabled:         true,
		})
		s.registry.Register("install_pkg", installer)
	}
}

type execCommand struct {
	Type string `json:"type"`
	Code string `json:"code,omitempty"`
	Repl bool   `json:"repl,omitempty"`
}

// Run executes code in the session, maintaining state from previous calls.
func (s *Session) Run(ctx context.Context, code string) Result {
	return s.runInternal(ctx, code, false)
}

// RunRepl executes code in REPL mode - expressions are auto-printed and result stored in _
func (s *Session) RunRepl(ctx context.Context, code string) Result {
	return s.runInternal(ctx, code, true)
}

func (s *Session) runInternal(ctx context.Context, code string, replMode bool) Result {
	s.execMu.Lock()
	defer s.execMu.Unlock()

	start := time.Now()

	if s.isClosed() {
		return Result{Error: ErrSessionClosed, Duration: time.Since(start)}
	}

	if !s.started {
		return Result{Error: s.startErr, Duration: time.Since(start)}
	}

	if s.cfg.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.timeout)
		defer cancel()
	}

	s.stdout.Reset()
	s.protocol.ResetExec()

	cmd := execCommand{Type: "exec", Code: code, Repl: replMode}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return Result{Error: fmt.Errorf("marshal command: %w", err), Duration: time.Since(start)}
	}
	cmdBytes = append(cmdBytes, '\n')

	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	if _, err := s.stdin.Write(cmdBytes); err != nil {
		return Result{Error: fmt.Errorf("write command: %w", err), Duration: time.Since(start)}
	}

	select {
	case <-ctx.Done():
		return Result{
			Output:   s.stdout.String() + s.protocol.Stderr(),
			Error:    fmt.Errorf("timeout after %v", s.cfg.timeout),
			Duration: time.Since(start),
		}
	case <-s.protocol.failed:
		return Result{Error: s.protocol.err, Duration: time.Since(start)}
	case execErr := <-s.protocol.doneCh:
		return Result{
			Output:   s.stdout.String() + s.protocol.Stderr(),
			Error:    execErr,
			Duration: time.Since(start),
		}
	}
}

// CheckComplete checks if the code is a complete statement (for multi-line REPL input)
func (s *Session) CheckComplete(ctx context.Context, code string) (bool, error) {
	s.execMu.Lock()
	defer s.execMu.Unlock()

	if s.isClosed() {
		return false, ErrSessionClosed
	}

	if !s.started {
		return false, s.startErr
	}

	s.protocol.ResetCheck()

	cmd := execCommand{Type: "check", Code: code}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return false, fmt.Errorf("marshal command: %w", err)
	}
	cmdBytes = append(cmdBytes, '\n')

	stop := context.AfterFunc(ctx, func() { s.Close() })
	defer stop()
	if _, err := s.stdin.Write(cmdBytes); err != nil {
		return false, fmt.Errorf("write command: %w", err)
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.protocol.failed:
		return false, s.protocol.err
	case complete := <-s.protocol.checkCh:
		return complete, nil
	}
}

func (s *Session) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Close terminates the session and releases resources.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Close pipes directly - don't try to send exit command as Python may be blocked
	// Closing stdinReader will cause Python to receive EOF and exit
	if s.stdinReader != nil {
		s.stdinReader.Close()
	}
	if s.stdin != nil {
		s.stdin.Close()
	}

	if s.cancel != nil {
		s.cancel()
	}

	return nil
}

type sessionProtocol struct {
	*protocolHandler
	readyCh chan struct{}
	doneCh  chan error
	checkCh chan bool
	failed  chan struct{}
	ready   bool
}

func newSessionProtocol(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *sessionProtocol {
	p := &sessionProtocol{
		protocolHandler: newProtocolHandler(ctx, registry, stdinWriter),
		readyCh:         make(chan struct{}),
		doneCh:          make(chan error, 1),
		checkCh:         make(chan bool, 1),
		failed:          make(chan struct{}),
	}
	p.protocolHandler.session = p
	return p
}

func (p *sessionProtocol) signal(frame string) bool {
	switch frame {
	case "GORU_READY":
		if !p.ready {
			p.ready = true
			close(p.readyCh)
		}
	case "GORU_COMPLETE", "GORU_INCOMPLETE":
		select {
		case p.checkCh <- frame == "GORU_COMPLETE":
		default:
		}
	case "GORU_DONE":
		select {
		case p.doneCh <- nil:
		default:
		}
	default:
		if message, ok := strings.CutPrefix(frame, "GORU_ERROR:"); ok {
			select {
			case p.doneCh <- errors.New(message):
			default:
			}
		} else {
			return false
		}
	}
	return true
}

func (p *sessionProtocol) ResetExec() {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.doneCh:
	default:
	}
	p.realStderr.Reset()
}

func (p *sessionProtocol) ResetCheck() {
	select {
	case <-p.checkCh:
	default:
	}
}
