package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

const usage = `goru - WASM Python sandbox

Usage:
  goru [options] [file.py]       Run Python code (default command)
  goru run [options] [file.py]   Run Python code
  goru serve [options]           Start HTTP server

Run Options:
  -c string         Python code to execute
  -timeout dur      Execution timeout (default 30s)
  -allow-host host  Allow HTTP to host (repeatable)
  -mount spec       Mount filesystem (virtual:host:mode, repeatable)
  -no-cache         Disable compilation cache

Serve Options:
  -port int         Port to listen on (default 8080)
  -timeout dur      Default execution timeout (default 30s)
  -session-ttl dur  Session expiry time (default 1h)
  -allow-host host  Allow HTTP to host (repeatable)
  -mount spec       Mount filesystem (virtual:host:mode, repeatable)
  -no-cache         Disable compilation cache

Mount Modes:
  ro   Read-only
  rw   Read-write (existing files only)
  rwc  Read-write-create (can create new files)

Examples:
  goru -c 'print(1+1)'
  goru script.py
  goru -timeout 5s -c 'while True: pass'
  goru -allow-host httpbin.org -c 'print(http_get("https://httpbin.org/get"))'
  goru -mount /data:./input:ro -c 'print(fs_read("/data/config.json"))'
  goru serve -port 8080 -allow-host api.example.com
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

// --- Run Command ---

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.Usage = func() { fmt.Print(usage) }

	var (
		code         = fs.String("c", "", "Python code to execute")
		timeout      = fs.Duration("timeout", 30*time.Second, "Execution timeout")
		noCache      = fs.Bool("no-cache", false, "Disable compilation cache")
		allowedHosts stringSlice
		mounts       stringSlice
	)
	fs.Var(&allowedHosts, "allow-host", "Allowed HTTP host (repeatable)")
	fs.Var(&mounts, "mount", "Mount spec virtual:host:mode (repeatable)")
	fs.Parse(args)

	var source string
	switch {
	case *code != "":
		source = *code
	case fs.NArg() > 0:
		data, err := os.ReadFile(fs.Arg(0))
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

	result := exec.Run(context.Background(), python.New(), source, runOpts...)
	fmt.Print(result.Output)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Error)
		os.Exit(1)
	}
}

// --- Serve Command ---

type sessionStore struct {
	stores map[string]*sessionEntry
	mu     sync.RWMutex
	ttl    time.Duration
}

type sessionEntry struct {
	kv       *hostfunc.KVStore
	lastUsed time.Time
}

func newSessionStore(ttl time.Duration) *sessionStore {
	s := &sessionStore{
		stores: make(map[string]*sessionEntry),
		ttl:    ttl,
	}
	go s.cleanup()
	return s
}

func (s *sessionStore) get(sessionID string) *hostfunc.KVStore {
	if sessionID == "" {
		return hostfunc.NewKVStore()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.stores[sessionID]
	if !ok {
		entry = &sessionEntry{kv: hostfunc.NewKVStore()}
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
		timeout      = fs.Duration("timeout", 30*time.Second, "Default execution timeout")
		sessionTTL   = fs.Duration("session-ttl", time.Hour, "Session expiry time")
		noCache      = fs.Bool("no-cache", false, "Disable compilation cache")
		allowedHosts stringSlice
		mounts       stringSlice
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
	execOpts = append(execOpts, executor.WithPrecompile(python.New()))

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	sessions := newSessionStore(*sessionTTL)

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

		if len(allowedHosts) > 0 {
			runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
		}

		for _, m := range parsedMounts {
			runOpts = append(runOpts, executor.WithMount(m.VirtualPath, m.HostPath, m.Mode))
		}

		result := exec.Run(r.Context(), python.New(), req.Code, runOpts...)

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
