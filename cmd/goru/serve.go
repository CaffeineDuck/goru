package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP server for code execution",
	Long: `Start an HTTP server that provides REST endpoints for code execution.

Endpoints:
  POST   /execute              Execute code (stateless)
  POST   /sessions             Create session, returns {"session_id":"..."}
  POST   /sessions/{id}/exec   Execute in session (state persists)
  DELETE /sessions/{id}        Close session
  GET    /health               Health check`,
	Run: runServe,
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port to listen on")
	serveCmd.Flags().StringP("lang", "l", "", "Language: python, js (required)")
	serveCmd.Flags().Duration("timeout", 30*time.Second, "Default execution timeout")
	serveCmd.Flags().StringSlice("allow-host", nil, "Allow HTTP to host (repeatable)")
	serveCmd.Flags().StringSlice("mount", nil, "Mount filesystem virtual:host:mode (repeatable)")

	serveCmd.Flags().Int("http-max-url", 8192, "Max HTTP URL length")
	serveCmd.Flags().Int64("http-max-body", 1024*1024, "Max HTTP response body size")
	serveCmd.Flags().Int64("fs-max-file", 10*1024*1024, "Max file read size")
	serveCmd.Flags().Int64("fs-max-write", 10*1024*1024, "Max file write size")
	serveCmd.Flags().Int("fs-max-path", 4096, "Max path length")

	rootCmd.AddCommand(serveCmd)
}

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
	Code    string `json:"code"`
	Lang    string `json:"lang,omitempty"`
	Timeout string `json:"timeout,omitempty"`
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

func runServe(cmd *cobra.Command, args []string) {
	port, _ := cmd.Flags().GetInt("port")
	defaultLang, _ := cmd.Flags().GetString("lang")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	noCache, _ := cmd.Root().PersistentFlags().GetBool("no-cache")
	allowedHosts, _ := cmd.Flags().GetStringSlice("allow-host")
	mounts, _ := cmd.Flags().GetStringSlice("mount")

	httpMaxURL, _ := cmd.Flags().GetInt("http-max-url")
	httpMaxBody, _ := cmd.Flags().GetInt64("http-max-body")
	fsMaxFile, _ := cmd.Flags().GetInt64("fs-max-file")
	fsMaxWrite, _ := cmd.Flags().GetInt64("fs-max-write")
	fsMaxPath, _ := cmd.Flags().GetInt("fs-max-path")

	var parsedMounts []hostfunc.Mount
	for _, spec := range mounts {
		m, err := parseMount(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		parsedMounts = append(parsedMounts, m)
	}

	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}
	defaultLanguage, langErr := getLanguage(defaultLang, "")
	if langErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", langErr)
		os.Exit(1)
	}
	execOpts = append(execOpts, executor.WithPrecompile(defaultLanguage))

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	sessions := newSessionManager(15 * time.Minute)
	defer sessions.closeAll()

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
			reqLang = defaultLang
		}
		language, langErr := getLanguage(reqLang, "")
		if langErr != nil {
			http.Error(w, langErr.Error(), http.StatusBadRequest)
			return
		}

		sessionID, err := sessions.create(exec, language)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create session: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createSessionResponse{SessionID: sessionID})
	})

	http.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/sessions/")
		parts := strings.SplitN(path, "/", 2)
		sessionID := parts[0]

		if sessionID == "" {
			http.Error(w, "session_id required", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodDelete {
			if sessions.close(sessionID) {
				w.WriteHeader(http.StatusNoContent)
			} else {
				http.Error(w, "session not found", http.StatusNotFound)
			}
			return
		}

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

		execTimeout := timeout
		if req.Timeout != "" {
			if d, err := time.ParseDuration(req.Timeout); err == nil {
				execTimeout = d
			}
		}

		var runOpts []executor.Option
		runOpts = append(runOpts, executor.WithTimeout(execTimeout))

		runOpts = append(runOpts,
			executor.WithHTTPMaxURLLength(httpMaxURL),
			executor.WithHTTPMaxBodySize(httpMaxBody),
			executor.WithFSMaxFileSize(fsMaxFile),
			executor.WithFSMaxWriteSize(fsMaxWrite),
			executor.WithFSMaxPathLength(fsMaxPath),
		)

		if len(allowedHosts) > 0 {
			runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
		}

		for _, m := range parsedMounts {
			runOpts = append(runOpts, executor.WithMount(m.VirtualPath, m.HostPath, m.Mode))
		}

		reqLang := req.Lang
		if reqLang == "" {
			reqLang = defaultLang
		}
		language, langErr := getLanguage(reqLang, "")
		if langErr != nil {
			http.Error(w, langErr.Error(), http.StatusBadRequest)
			return
		}

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

	addr := fmt.Sprintf(":%d", port)
	fmt.Fprintf(os.Stderr, "goru server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
