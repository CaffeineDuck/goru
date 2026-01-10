package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/javascript"
	"github.com/caffeineduck/goru/language/python"
)

const usage = `goru - WASM code sandbox

Usage:
  goru [options] [file]          Run code (language auto-detected from extension)
  goru run [options] [file]      Run code
  goru repl [options]            Interactive REPL with persistent state
  goru serve [options]           Start HTTP server
  goru deps <command>            Manage Python packages (goru deps -h for help)

Common Options:
  -c string         Code to execute (run only)
  -lang string      Language: python, js (default: auto-detect)
  -timeout dur      Execution timeout (default 30s)
  -allow-host host  Allow HTTP to host (repeatable)
  -mount spec       Mount filesystem (virtual:host:mode, repeatable)
  -no-cache         Disable compilation cache

Security Limits (all commands):
  -http-max-url int    Max HTTP URL length (default 8192)
  -http-max-body int   Max HTTP response body (default 1048576)
  -fs-max-file int     Max file read size (default 10485760)
  -fs-max-write int    Max file write size (default 10485760)
  -fs-max-path int     Max path length (default 4096)

REPL Options:
  -lang string      Language: python, js (default: python)
  -packages path    Path to packages directory (Python only)

Serve Options:
  -port int         Port to listen on (default 8080)

HTTP API Endpoints:
  POST   /execute              Execute code (stateless)
  POST   /sessions             Create session, returns {"session_id":"..."}
  POST   /sessions/{id}/exec   Execute in session (state persists)
  DELETE /sessions/{id}        Close session
  GET    /health               Health check

Languages:
  python, py   Python 3.12 (CPython WASM, ~1.5s cold / ~120ms warm)
  js           JavaScript ES2023 (QuickJS WASM, ~200ms)

Mount Modes:
  ro   Read-only
  rw   Read-write (existing files only)
  rwc  Read-write-create (can create new files)

Examples:
  goru -c 'print(1+1)'
  goru script.py
  goru -lang js -c 'console.log(1+1)'
  goru -timeout 5s -c 'while True: pass'
  goru serve -port 8080 -lang js
`

func main() {
	if len(os.Args) < 2 {
		// No args, read from stdin
		runCmd(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "serve":
		serveCmd(os.Args[2:])
	case "run":
		runCmd(os.Args[2:])
	case "repl":
		replCmd(os.Args[2:])
	case "deps":
		depsCmd(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		// Default to run
		runCmd(os.Args[1:])
	}
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func parseMount(spec string) (hostfunc.Mount, error) {
	parts := strings.Split(spec, ":")
	if len(parts) != 3 {
		return hostfunc.Mount{}, fmt.Errorf("invalid mount spec %q (expected virtual:host:mode)", spec)
	}

	var mode hostfunc.MountMode
	switch parts[2] {
	case "ro":
		mode = hostfunc.MountReadOnly
	case "rw":
		mode = hostfunc.MountReadWrite
	case "rwc":
		mode = hostfunc.MountReadWriteCreate
	default:
		return hostfunc.Mount{}, fmt.Errorf("invalid mount mode %q (expected ro, rw, or rwc)", parts[2])
	}

	return hostfunc.Mount{
		VirtualPath: parts[0],
		HostPath:    parts[1],
		Mode:        mode,
	}, nil
}

func getLanguage(langFlag string, filename string) executor.Language {
	lang := langFlag

	// Auto-detect from file extension if not specified
	if lang == "" && filename != "" {
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".py":
			lang = "python"
		case ".js", ".mjs":
			lang = "js"
		}
	}

	// Default to python
	if lang == "" {
		lang = "python"
	}

	switch lang {
	case "js", "javascript":
		return javascript.New()
	default:
		return python.New()
	}
}

// --- Run Command ---

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usage) }

	var (
		code         = fs.String("c", "", "Code to execute")
		lang         = fs.String("lang", "", "Language: python, js (default: auto-detect)")
		timeout      = fs.Duration("timeout", 30*time.Second, "Execution timeout")
		noCache      = fs.Bool("no-cache", false, "Disable compilation cache")
		enableKV     = fs.Bool("kv", false, "Enable key-value store")
		allowedHosts stringSlice
		mounts       stringSlice
		// Security limits
		httpMaxURL  = fs.Int("http-max-url", 8192, "Max HTTP URL length")
		httpMaxBody = fs.Int64("http-max-body", 1024*1024, "Max HTTP response body")
		fsMaxFile   = fs.Int64("fs-max-file", 10*1024*1024, "Max file read size")
		fsMaxWrite  = fs.Int64("fs-max-write", 10*1024*1024, "Max file write size")
		fsMaxPath   = fs.Int("fs-max-path", 4096, "Max path length")
	)
	fs.Var(&allowedHosts, "allow-host", "Allowed HTTP host (repeatable)")
	fs.Var(&mounts, "mount", "Mount spec virtual:host:mode (repeatable)")
	fs.Parse(args)

	var source string
	var filename string
	switch {
	case *code != "":
		source = *code
	case fs.NArg() > 0:
		filename = fs.Arg(0)
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	}

	if source == "" {
		fmt.Print(usage)
		os.Exit(1)
	}

	language := getLanguage(*lang, filename)
	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !*noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	var runOpts []executor.Option
	runOpts = append(runOpts, executor.WithTimeout(*timeout))

	// Security limits
	runOpts = append(runOpts,
		executor.WithHTTPMaxURLLength(*httpMaxURL),
		executor.WithHTTPMaxBodySize(*httpMaxBody),
		executor.WithFSMaxFileSize(*fsMaxFile),
		executor.WithFSMaxWriteSize(*fsMaxWrite),
		executor.WithFSMaxPathLength(*fsMaxPath),
	)

	if len(allowedHosts) > 0 {
		runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
	}

	if *enableKV {
		runOpts = append(runOpts, executor.WithKV())
	}

	for _, spec := range mounts {
		m, err := parseMount(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		runOpts = append(runOpts, executor.WithMount(m.VirtualPath, m.HostPath, m.Mode))
	}

	result := exec.Run(context.Background(), language, source, runOpts...)
	fmt.Print(result.Output)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Error)
		os.Exit(1)
	}
}

// --- REPL Command ---

func replCmd(args []string) {
	fs := flag.NewFlagSet("repl", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usage) }

	var (
		lang     = fs.String("lang", "python", "Language: python, js")
		packages = fs.String("packages", "", "Path to packages directory (Python only)")
		noCache  = fs.Bool("no-cache", false, "Disable compilation cache")
		enableKV = fs.Bool("kv", false, "Enable key-value store")
	)
	fs.Parse(args)

	language := getLanguage(*lang, "")
	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !*noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}
	execOpts = append(execOpts, executor.WithPrecompile(language))

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	var sessionOpts []executor.SessionOption
	if *packages != "" {
		sessionOpts = append(sessionOpts, executor.WithPackages(*packages))
	}
	if *enableKV {
		sessionOpts = append(sessionOpts, executor.WithSessionKV())
	}

	session, err := exec.NewSession(language, sessionOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error starting session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	langName := language.Name()
	fmt.Fprintf(os.Stderr, "goru %s REPL (type 'exit' to quit)\n", langName)

	reader := bufio.NewReader(os.Stdin)
	prompt := ">>> "

	for {
		fmt.Print(prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				break
			}
			fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		result := session.Run(context.Background(), line)
		if result.Output != "" {
			fmt.Print(result.Output)
			// Ensure output ends with newline
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		}
	}
}

// --- Deps Command ---

const depsUsage = `goru deps - Manage Python packages for sandboxed code

Note: JavaScript packages not supported. Use bundling for JS.

Usage:
  goru deps install <packages...>    Install packages to .goru/python/packages
  goru deps list                     List installed packages
  goru deps remove <packages...>     Remove packages
  goru deps cache clear              Clear download cache

Options:
  -dir string    Package directory (default ".goru/python/packages")

Examples:
  goru deps install pydantic requests
  goru deps list
  goru deps remove pydantic
`

func depsCmd(args []string) {
	if len(args) == 0 {
		fmt.Print(depsUsage)
		return
	}

	fs := flag.NewFlagSet("deps", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(depsUsage) }
	pkgDir := fs.String("dir", ".goru/python/packages", "Package directory")

	subCmd := args[0]
	args = args[1:]

	switch subCmd {
	case "install":
		fs.Parse(args)
		packages := fs.Args()
		if len(packages) == 0 {
			fmt.Fprintln(os.Stderr, "error: no packages specified")
			os.Exit(1)
		}
		depsInstall(*pkgDir, packages)

	case "list":
		fs.Parse(args)
		depsList(*pkgDir)

	case "remove":
		fs.Parse(args)
		packages := fs.Args()
		if len(packages) == 0 {
			fmt.Fprintln(os.Stderr, "error: no packages specified")
			os.Exit(1)
		}
		depsRemove(*pkgDir, packages)

	case "cache":
		fs.Parse(args)
		if len(fs.Args()) > 0 && fs.Args()[0] == "clear" {
			depsCacheClear()
		} else {
			fmt.Fprintln(os.Stderr, "error: unknown cache command (use 'cache clear')")
			os.Exit(1)
		}

	case "-h", "--help", "help":
		fmt.Print(depsUsage)

	default:
		fmt.Fprintf(os.Stderr, "error: unknown deps command %q\n", subCmd)
		fmt.Print(depsUsage)
		os.Exit(1)
	}
}

func depsInstall(pkgDir string, packages []string) {
	// Ensure package directory exists
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create package dir: %v\n", err)
		os.Exit(1)
	}

	// Use pip to install packages to target directory
	args := append([]string{"install", "--target", pkgDir}, packages...)
	cmd := exec.Command("pip", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Installing packages to %s...\n", pkgDir)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: pip install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Done.")
}

func depsList(pkgDir string) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No packages installed.")
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No packages installed.")
		return
	}

	fmt.Printf("Packages in %s:\n", pkgDir)
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasSuffix(entry.Name(), ".dist-info") && !strings.HasPrefix(entry.Name(), "__") {
			fmt.Printf("  %s\n", entry.Name())
		}
	}
}

func depsRemove(pkgDir string, packages []string) {
	for _, pkg := range packages {
		// Remove package directory
		pkgPath := filepath.Join(pkgDir, pkg)
		if err := os.RemoveAll(pkgPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: failed to remove %s: %v\n", pkg, err)
			continue
		}

		// Also remove dist-info if present
		entries, _ := os.ReadDir(pkgDir)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), pkg) && strings.HasSuffix(entry.Name(), ".dist-info") {
				distInfoPath := filepath.Join(pkgDir, entry.Name())
				os.RemoveAll(distInfoPath)
			}
		}

		fmt.Printf("Removed %s\n", pkg)
	}
}

func depsCacheClear() {
	cacheDir := filepath.Join(".goru", "cache")
	if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: failed to clear cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Cache cleared.")
}

// --- Serve Command ---

type sessionManager struct {
	sessions map[string]*serverSession
	mu       sync.RWMutex
	ttl      time.Duration
}

type serverSession struct {
	session  *executor.Session
	lastUsed time.Time
}

func newSessionManager(ttl time.Duration) *sessionManager {
	sm := &sessionManager{
		sessions: make(map[string]*serverSession),
		ttl:      ttl,
	}
	go sm.cleanup()
	return sm
}

func (sm *sessionManager) create(exec *executor.Executor, lang executor.Language, opts ...executor.SessionOption) (string, error) {
	session, err := exec.NewSession(lang, opts...)
	if err != nil {
		return "", err
	}

	id := generateSessionID()
	sm.mu.Lock()
	sm.sessions[id] = &serverSession{
		session:  session,
		lastUsed: time.Now(),
	}
	sm.mu.Unlock()
	return id, nil
}

func (sm *sessionManager) get(id string) (*executor.Session, bool) {
	sm.mu.RLock()
	ss, ok := sm.sessions[id]
	sm.mu.RUnlock()
	if !ok {
		return nil, false
	}

	sm.mu.Lock()
	ss.lastUsed = time.Now()
	sm.mu.Unlock()
	return ss.session, true
}

func (sm *sessionManager) close(id string) bool {
	sm.mu.Lock()
	ss, ok := sm.sessions[id]
	if ok {
		ss.session.Close()
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()
	return ok
}

func (sm *sessionManager) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, ss := range sm.sessions {
			if now.Sub(ss.lastUsed) > sm.ttl {
				ss.session.Close()
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

func (sm *sessionManager) closeAll() {
	sm.mu.Lock()
	for id, ss := range sm.sessions {
		ss.session.Close()
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

type executeRequest struct {
	Code      string `json:"code"`
	Lang      string `json:"lang,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

type executeResponse struct {
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type createSessionRequest struct {
	Lang string `json:"lang,omitempty"`
}

type createSessionResponse struct {
	SessionID string `json:"session_id"`
}

type sessionExecRequest struct {
	Code    string `json:"code"`
	Timeout string `json:"timeout,omitempty"`
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usage) }

	var (
		port         = fs.Int("port", 8080, "Port to listen on")
		defaultLang  = fs.String("lang", "python", "Default language: python, js")
		timeout      = fs.Duration("timeout", 30*time.Second, "Default execution timeout")
		noCache      = fs.Bool("no-cache", false, "Disable compilation cache")
		allowedHosts stringSlice
		mounts       stringSlice
		// Security limits
		httpMaxURL = fs.Int("http-max-url", 8192, "Max HTTP URL length")
		httpMaxBody = fs.Int64("http-max-body", 1024*1024, "Max HTTP response body")
		fsMaxFile  = fs.Int64("fs-max-file", 10*1024*1024, "Max file read size")
		fsMaxWrite = fs.Int64("fs-max-write", 10*1024*1024, "Max file write size")
		fsMaxPath  = fs.Int("fs-max-path", 4096, "Max path length")
	)
	fs.Var(&allowedHosts, "allow-host", "Allowed HTTP host (repeatable)")
	fs.Var(&mounts, "mount", "Mount spec virtual:host:mode (repeatable)")
	fs.Parse(args)

	// Parse mounts upfront
	var parsedMounts []hostfunc.Mount
	for _, spec := range mounts {
		m, err := parseMount(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		parsedMounts = append(parsedMounts, m)
	}

	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !*noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}
	// Precompile the default language
	execOpts = append(execOpts, executor.WithPrecompile(getLanguage(*defaultLang, "")))

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	// Session management with 15-minute TTL
	sessions := newSessionManager(15 * time.Minute)
	defer sessions.closeAll()

	// POST /sessions - Create a new session
	http.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req createSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		reqLang := req.Lang
		if reqLang == "" {
			reqLang = *defaultLang
		}
		language := getLanguage(reqLang, "")

		sessionID, err := sessions.create(exec, language)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create session: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createSessionResponse{SessionID: sessionID})
	})

	// POST /sessions/{id}/exec - Execute code in session
	// DELETE /sessions/{id} - Close session
	http.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		// Parse session ID from path: /sessions/{id} or /sessions/{id}/exec
		path := strings.TrimPrefix(r.URL.Path, "/sessions/")
		parts := strings.SplitN(path, "/", 2)
		sessionID := parts[0]

		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}

		// DELETE /sessions/{id}
		if r.Method == http.MethodDelete {
			if sessions.close(sessionID) {
				w.WriteHeader(http.StatusNoContent)
			} else {
				http.Error(w, "session not found", http.StatusNotFound)
			}
			return
		}

		// POST /sessions/{id}/exec
		if r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "exec" {
			session, ok := sessions.get(sessionID)
			if !ok {
				http.Error(w, "session not found", http.StatusNotFound)
				return
			}

			var req sessionExecRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}

			if req.Code == "" {
				http.Error(w, "code required", http.StatusBadRequest)
				return
			}

			ctx := r.Context()
			if req.Timeout != "" {
				if d, err := time.ParseDuration(req.Timeout); err == nil {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, d)
					defer cancel()
				}
			}

			start := time.Now()
			result := session.Run(ctx, req.Code)
			duration := time.Since(start)

			resp := executeResponse{
				Output:     result.Output,
				DurationMs: duration.Milliseconds(),
			}
			if result.Error != nil {
				resp.Error = result.Error.Error()
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// POST /execute - Stateless execution (backward compatible)
	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req executeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if req.Code == "" {
			http.Error(w, "code required", http.StatusBadRequest)
			return
		}

		execTimeout := *timeout
		if req.Timeout != "" {
			if d, err := time.ParseDuration(req.Timeout); err == nil {
				execTimeout = d
			}
		}

		var runOpts []executor.Option
		runOpts = append(runOpts, executor.WithTimeout(execTimeout))

		// Security limits
		runOpts = append(runOpts,
			executor.WithHTTPMaxURLLength(*httpMaxURL),
			executor.WithHTTPMaxBodySize(*httpMaxBody),
			executor.WithFSMaxFileSize(*fsMaxFile),
			executor.WithFSMaxWriteSize(*fsMaxWrite),
			executor.WithFSMaxPathLength(*fsMaxPath),
		)

		if len(allowedHosts) > 0 {
			runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
		}

		for _, m := range parsedMounts {
			runOpts = append(runOpts, executor.WithMount(m.VirtualPath, m.HostPath, m.Mode))
		}

		// Use request language or default
		reqLang := req.Lang
		if reqLang == "" {
			reqLang = *defaultLang
		}
		language := getLanguage(reqLang, "")

		result := exec.Run(r.Context(), language, req.Code, runOpts...)

		resp := executeResponse{
			Output:     result.Output,
			DurationMs: result.Duration.Milliseconds(),
		}
		if result.Error != nil {
			resp.Error = result.Error.Error()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", *port)
	fmt.Fprintf(os.Stderr, "goru server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
