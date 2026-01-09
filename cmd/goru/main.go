package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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
  goru serve [options]           Start HTTP server

Common Options:
  -c string         Code to execute (run only)
  -lang string      Language: python, js (default: auto-detect)
  -timeout dur      Execution timeout (default 30s)
  -allow-host host  Allow HTTP to host (repeatable)
  -mount spec       Mount filesystem (virtual:host:mode, repeatable)
  -no-cache         Disable compilation cache

Security Limits (all commands):
  -kv-max-key int      Max KV key size in bytes (default 1024)
  -kv-max-value int    Max KV value size in bytes (default 1048576)
  -kv-max-entries int  Max KV entries (default 10000)
  -http-max-url int    Max HTTP URL length (default 8192)
  -http-max-body int   Max HTTP response body (default 1048576)
  -fs-max-file int     Max file read size (default 10485760)
  -fs-max-write int    Max file write size (default 10485760)
  -fs-max-path int     Max path length (default 4096)

Serve Options:
  -port int         Port to listen on (default 8080)
  -session-ttl dur  Session expiry time (default 1h)

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
  goru -kv-max-entries 100 -c 'for i in range(200): kv_set(str(i), "x")'
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
		allowedHosts stringSlice
		mounts       stringSlice
		// Security limits
		kvMaxKey     = fs.Int("kv-max-key", 1024, "Max KV key size")
		kvMaxValue   = fs.Int("kv-max-value", 1024*1024, "Max KV value size")
		kvMaxEntries = fs.Int("kv-max-entries", 10000, "Max KV entries")
		httpMaxURL   = fs.Int("http-max-url", 8192, "Max HTTP URL length")
		httpMaxBody  = fs.Int64("http-max-body", 1024*1024, "Max HTTP response body")
		fsMaxFile    = fs.Int64("fs-max-file", 10*1024*1024, "Max file read size")
		fsMaxWrite   = fs.Int64("fs-max-write", 10*1024*1024, "Max file write size")
		fsMaxPath    = fs.Int("fs-max-path", 4096, "Max path length")
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
		executor.WithKVMaxKeySize(*kvMaxKey),
		executor.WithKVMaxValueSize(*kvMaxValue),
		executor.WithKVMaxEntries(*kvMaxEntries),
		executor.WithHTTPMaxURLLength(*httpMaxURL),
		executor.WithHTTPMaxBodySize(*httpMaxBody),
		executor.WithFSMaxFileSize(*fsMaxFile),
		executor.WithFSMaxWriteSize(*fsMaxWrite),
		executor.WithFSMaxPathLength(*fsMaxPath),
	)

	if len(allowedHosts) > 0 {
		runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
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

// --- Serve Command ---

type sessionStore struct {
	stores    map[string]*sessionEntry
	mu        sync.RWMutex
	ttl       time.Duration
	kvOptions []hostfunc.KVOption
}

type sessionEntry struct {
	kv       *hostfunc.KVStore
	lastUsed time.Time
}

func newSessionStore(ttl time.Duration, kvOptions []hostfunc.KVOption) *sessionStore {
	s := &sessionStore{
		stores:    make(map[string]*sessionEntry),
		ttl:       ttl,
		kvOptions: kvOptions,
	}
	go s.cleanup()
	return s
}

func (s *sessionStore) get(sessionID string) *hostfunc.KVStore {
	if sessionID == "" {
		return hostfunc.NewKVStore(s.kvOptions...)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.stores[sessionID]
	if !ok {
		entry = &sessionEntry{kv: hostfunc.NewKVStore(s.kvOptions...)}
		s.stores[sessionID] = entry
	}
	entry.lastUsed = time.Now()
	return entry.kv
}

func (s *sessionStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, entry := range s.stores {
			if now.Sub(entry.lastUsed) > s.ttl {
				delete(s.stores, id)
			}
		}
		s.mu.Unlock()
	}
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

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usage) }

	var (
		port         = fs.Int("port", 8080, "Port to listen on")
		defaultLang  = fs.String("lang", "python", "Default language: python, js")
		timeout      = fs.Duration("timeout", 30*time.Second, "Default execution timeout")
		sessionTTL   = fs.Duration("session-ttl", time.Hour, "Session expiry time")
		noCache      = fs.Bool("no-cache", false, "Disable compilation cache")
		allowedHosts stringSlice
		mounts       stringSlice
		// Security limits
		kvMaxKey     = fs.Int("kv-max-key", 1024, "Max KV key size")
		kvMaxValue   = fs.Int("kv-max-value", 1024*1024, "Max KV value size")
		kvMaxEntries = fs.Int("kv-max-entries", 10000, "Max KV entries")
		httpMaxURL   = fs.Int("http-max-url", 8192, "Max HTTP URL length")
		httpMaxBody  = fs.Int64("http-max-body", 1024*1024, "Max HTTP response body")
		fsMaxFile    = fs.Int64("fs-max-file", 10*1024*1024, "Max file read size")
		fsMaxWrite   = fs.Int64("fs-max-write", 10*1024*1024, "Max file write size")
		fsMaxPath    = fs.Int("fs-max-path", 4096, "Max path length")
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

	kvOpts := []hostfunc.KVOption{
		hostfunc.WithMaxKeySize(*kvMaxKey),
		hostfunc.WithMaxValueSize(*kvMaxValue),
		hostfunc.WithMaxEntries(*kvMaxEntries),
	}
	sessions := newSessionStore(*sessionTTL, kvOpts)

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
		runOpts = append(runOpts, executor.WithKVStore(sessions.get(req.SessionID)))

		// Security limits (KV limits applied at session creation, these are for non-session usage)
		runOpts = append(runOpts,
			executor.WithKVMaxKeySize(*kvMaxKey),
			executor.WithKVMaxValueSize(*kvMaxValue),
			executor.WithKVMaxEntries(*kvMaxEntries),
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
