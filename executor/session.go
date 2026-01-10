package executor

import (
	"bytes"
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
	"github.com/tetratelabs/wazero/api"
)

var (
	ErrSessionClosed = errors.New("session closed")
	ErrSessionBusy   = errors.New("session busy")
)

type Session struct {
	exec     *Executor
	lang     Language
	cfg      sessionConfig
	registry *hostfunc.Registry

	stdin       *io.PipeWriter
	stdinReader *io.PipeReader
	stdout      *sessionOutput
	protocol    *sessionProtocol
	module      api.Module

	mu       sync.Mutex
	execMu   sync.Mutex
	closed   bool
	started  bool
	startErr error
}

type sessionConfig struct {
	timeout         time.Duration
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

type SessionOption func(*sessionConfig)

func WithSessionTimeout(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.timeout = d
	}
}

func WithSessionAllowedHosts(hosts []string) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.AllowedHosts = hosts
	}
}

func WithSessionMount(virtualPath, hostPath string, mode hostfunc.MountMode) SessionOption {
	return func(c *sessionConfig) {
		c.mounts = append(c.mounts, hostfunc.Mount{
			VirtualPath: virtualPath,
			HostPath:    hostPath,
			Mode:        mode,
		})
	}
}

func WithPackages(path string) SessionOption {
	return func(c *sessionConfig) {
		c.packagesPath = path
	}
}

func WithPackageInstall(enabled bool) SessionOption {
	return func(c *sessionConfig) {
		c.pkgInstall = enabled
	}
}

func WithAllowedPackages(packages []string) SessionOption {
	return func(c *sessionConfig) {
		c.pkgInstall = true
		c.allowedPackages = packages
	}
}

func WithSessionHTTPTimeout(d time.Duration) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.RequestTimeout = d
	}
}

func WithSessionHTTPMaxURLLength(size int) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.MaxURLLength = size
	}
}

func WithSessionHTTPMaxBodySize(size int64) SessionOption {
	return func(c *sessionConfig) {
		c.httpConfig.MaxBodySize = size
	}
}

func WithSessionKV() SessionOption {
	return func(c *sessionConfig) {
		c.kvEnabled = true
		c.kvConfig = hostfunc.DefaultKVConfig()
	}
}

func WithSessionKVConfig(cfg hostfunc.KVConfig) SessionOption {
	return func(c *sessionConfig) {
		c.kvEnabled = true
		c.kvConfig = cfg
	}
}

func WithSessionFSMaxFileSize(size int64) SessionOption {
	return func(c *sessionConfig) {
		c.fsOptions = append(c.fsOptions, hostfunc.WithMaxFileSize(size))
	}
}

func (e *Executor) NewSession(lang Language, opts ...SessionOption) (*Session, error) {
	cfg := defaultSessionConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	cfg.env["GORU_SESSION"] = "1"

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

	ctx := context.Background()

	compiled, err := s.exec.getCompiled(ctx, s.lang)
	if err != nil {
		s.startErr = err
		return err
	}

	s.registerHostFunctions()

	s.stdinReader, s.stdin = io.Pipe()
	s.stdout = newSessionOutput()
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

	go func() {
		mod, err := s.exec.runtime.InstantiateModule(ctx, compiled, moduleConfig)
		if err != nil {
			s.mu.Lock()
			s.startErr = fmt.Errorf("start session: %w", err)
			s.mu.Unlock()
			return
		}
		s.module = mod
	}()

	select {
	case <-s.protocol.Ready():
		s.started = true
		return nil
	case <-time.After(30 * time.Second):
		s.startErr = errors.New("session start timeout")
		return s.startErr
	}
}

func (s *Session) registerHostFunctions() {
	s.registry.Register("time_now", func(ctx context.Context, args map[string]any) (any, error) {
		return float64(time.Now().UnixNano()) / 1e9, nil
	})

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

	if s.closed {
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
	case execErr := <-s.protocol.Done():
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

	if s.closed {
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

	if _, err := s.stdin.Write(cmdBytes); err != nil {
		return false, fmt.Errorf("write command: %w", err)
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case complete := <-s.protocol.CheckDone():
		return complete, nil
	}
}

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

	// Close the module if it's still running
	if s.module != nil {
		s.module.Close(context.Background())
	}

	return nil
}

type sessionOutput struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func newSessionOutput() *sessionOutput {
	return &sessionOutput{}
}

func (o *sessionOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Write(data)
}

func (o *sessionOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func (o *sessionOutput) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Reset()
}

const (
	sessionDoneSignal       = "\x00GORU_DONE\x00"
	sessionErrorPrefix      = "\x00GORU_ERROR:"
	sessionReadySignal      = "\x00GORU_READY\x00"
	sessionCompleteSignal   = "\x00GORU_COMPLETE\x00"
	sessionIncompleteSignal = "\x00GORU_INCOMPLETE\x00"
)

type sessionProtocol struct {
	ctx         context.Context
	registry    *hostfunc.Registry
	stdinWriter *io.PipeWriter

	buf        bytes.Buffer
	realStderr bytes.Buffer
	pending    []callRequest

	readyCh   chan struct{}
	doneCh    chan error
	checkCh   chan bool
	ready     bool

	mu      sync.Mutex
	writeMu sync.Mutex
}

func newSessionProtocol(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *sessionProtocol {
	return &sessionProtocol{
		ctx:         ctx,
		registry:    registry,
		stdinWriter: stdinWriter,
		pending:     make([]callRequest, 0),
		readyCh:     make(chan struct{}),
		doneCh:      make(chan error, 1),
		checkCh:     make(chan bool, 1),
	}
}

func (p *sessionProtocol) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(data)
	p.buf.Write(data)

	for {
		content := p.buf.String()

		if p.checkSessionSignals(content) {
			continue
		}

		if p.processProtocolMessages(content) {
			continue
		}

		break
	}

	return n, nil
}

func (p *sessionProtocol) checkSessionSignals(content string) bool {
	if idx := strings.Index(content, sessionReadySignal); idx != -1 {
		if idx > 0 {
			p.realStderr.WriteString(content[:idx])
		}
		p.buf.Reset()
		p.buf.WriteString(content[idx+len(sessionReadySignal):])

		if !p.ready {
			p.ready = true
			close(p.readyCh)
		}
		return true
	}

	if idx := strings.Index(content, sessionCompleteSignal); idx != -1 {
		if idx > 0 {
			p.realStderr.WriteString(content[:idx])
		}
		p.buf.Reset()
		p.buf.WriteString(content[idx+len(sessionCompleteSignal):])

		select {
		case p.checkCh <- true:
		default:
		}
		return true
	}

	if idx := strings.Index(content, sessionIncompleteSignal); idx != -1 {
		if idx > 0 {
			p.realStderr.WriteString(content[:idx])
		}
		p.buf.Reset()
		p.buf.WriteString(content[idx+len(sessionIncompleteSignal):])

		select {
		case p.checkCh <- false:
		default:
		}
		return true
	}

	if idx := strings.Index(content, sessionDoneSignal); idx != -1 {
		if idx > 0 {
			p.realStderr.WriteString(content[:idx])
		}
		p.buf.Reset()
		p.buf.WriteString(content[idx+len(sessionDoneSignal):])

		select {
		case p.doneCh <- nil:
		default:
		}
		return true
	}

	if idx := strings.Index(content, sessionErrorPrefix); idx != -1 {
		afterPrefix := content[idx+len(sessionErrorPrefix):]
		if endIdx := strings.Index(afterPrefix, "\x00"); endIdx != -1 {
			errMsg := afterPrefix[:endIdx]
			if idx > 0 {
				p.realStderr.WriteString(content[:idx])
			}
			p.buf.Reset()
			p.buf.WriteString(afterPrefix[endIdx+1:])

			select {
			case p.doneCh <- errors.New(errMsg):
			default:
			}
			return true
		}
	}

	return false
}

func (p *sessionProtocol) processProtocolMessages(content string) bool {
	idx, msgType := findNextMessage(content)
	if msgType == messageNone {
		return false
	}

	if idx > 0 {
		p.realStderr.WriteString(content[:idx])
		p.buf.Reset()
		p.buf.WriteString(content[idx:])
		content = p.buf.String()
		idx = 0
	}

	switch msgType {
	case messageFlush:
		payload, remaining, ok := extractMessage(content, idx, protocolFlushPrefix)
		if !ok {
			return false
		}
		p.buf.Reset()
		p.buf.WriteString(remaining)
		p.handleFlush(payload)
		return true

	case messageCall:
		payload, remaining, ok := extractMessage(content, idx, protocolPrefix)
		if !ok {
			return false
		}
		p.buf.Reset()
		p.buf.WriteString(remaining)
		p.handleCall(payload)
		return true
	}

	return false
}

func (p *sessionProtocol) handleFlush(payload string) {
	count := 0
	fmt.Sscanf(payload, "%d", &count)
	if count <= 0 || count > len(p.pending) {
		count = len(p.pending)
	}
	if count == 0 {
		return
	}

	requests := p.pending[:count]
	p.pending = p.pending[count:]

	var wg sync.WaitGroup
	wg.Add(len(requests))

	for _, req := range requests {
		go func(r callRequest) {
			defer wg.Done()
			resp := p.executeCall(r)
			resp.ID = r.ID
			p.respond(resp)
		}(req)
	}

	wg.Wait()
}

func (p *sessionProtocol) handleCall(payload string) {
	var req callRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		go p.respond(callResponse{Error: "invalid call format"})
		return
	}

	if req.ID != "" {
		p.pending = append(p.pending, req)
	} else {
		// Execute and respond in goroutine to avoid blocking Write()
		go func() {
			p.respond(p.executeCall(req))
		}()
	}
}

func (p *sessionProtocol) executeCall(req callRequest) callResponse {
	fn, ok := p.registry.Get(req.Fn)
	if !ok {
		return callResponse{Error: "unknown function: " + req.Fn}
	}

	result, err := fn(p.ctx, req.Args)
	if err != nil {
		return callResponse{Error: err.Error()}
	}
	return callResponse{Data: result}
}

func (p *sessionProtocol) respond(resp callResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		data = []byte(`{"error":"internal: failed to marshal response"}`)
	}

	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	p.stdinWriter.Write(append(data, '\n'))
}

func (p *sessionProtocol) Ready() <-chan struct{} {
	return p.readyCh
}

func (p *sessionProtocol) Done() <-chan error {
	return p.doneCh
}

func (p *sessionProtocol) ResetExec() {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.doneCh:
	default:
	}
	p.doneCh = make(chan error, 1)
	p.realStderr.Reset()
}

func (p *sessionProtocol) ResetCheck() {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.checkCh:
	default:
	}
	p.checkCh = make(chan bool, 1)
}

func (p *sessionProtocol) CheckDone() <-chan bool {
	return p.checkCh
}

func (p *sessionProtocol) Stderr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.realStderr.String()
}
