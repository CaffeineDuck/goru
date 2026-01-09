package sandbox

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

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

type Options struct {
	Timeout      time.Duration
	AllowedHosts []string // empty = no http allowed
}

func DefaultOptions() Options {
	return Options{Timeout: 30 * time.Second}
}

type hostCall struct {
	Fn   string         `json:"fn"`
	Args map[string]any `json:"args"`
}

type hostResponse struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func RunPython(code string, opts Options) Result {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	rtConfig := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	var stdout bytes.Buffer
	stdinReader, stdinWriter := io.Pipe()
	stderr := &protocolInterceptor{
		opts:        opts,
		stdinWriter: stdinWriter,
	}

	fullCode := stdlibPy + "\n" + code

	config := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(stderr).
		WithStdin(stdinReader).
		WithArgs("python", "-c", fullCode).
		WithName("python")

	errCh := make(chan error, 1)
	go func() {
		_, err := rt.InstantiateWithConfig(ctx, pythonWasm, config)
		stdinWriter.Close()
		errCh <- err
	}()

	err := <-errCh

	result := Result{
		Output:   stdout.String() + stderr.RealStderr(),
		Duration: time.Since(start),
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Errorf("timeout after %v", opts.Timeout)
		} else {
			result.Error = fmt.Errorf("execution failed: %w", err)
		}
	}

	return result
}

type protocolInterceptor struct {
	opts        Options
	stdinWriter *io.PipeWriter
	realStderr  bytes.Buffer
	buf         bytes.Buffer
	mu          sync.Mutex
}

func (p *protocolInterceptor) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buf.Write(data)

	for {
		content := p.buf.String()
		startIdx := strings.Index(content, "\x00GORU:")
		if startIdx == -1 {
			p.realStderr.WriteString(content)
			p.buf.Reset()
			break
		}

		p.realStderr.WriteString(content[:startIdx])

		endIdx := strings.Index(content[startIdx+6:], "\x00")
		if endIdx == -1 {
			p.buf.Reset()
			p.buf.WriteString(content[startIdx:])
			break
		}

		jsonStr := content[startIdx+6 : startIdx+6+endIdx]
		p.buf.Reset()
		p.buf.WriteString(content[startIdx+6+endIdx+1:])

		var call hostCall
		if err := json.Unmarshal([]byte(jsonStr), &call); err != nil {
			p.respond(hostResponse{Error: "invalid call format"})
			continue
		}

		resp := p.handleCall(call)
		p.respond(resp)
	}

	return len(data), nil
}

func (p *protocolInterceptor) respond(resp hostResponse) {
	data, _ := json.Marshal(resp)
	go p.stdinWriter.Write(append(data, '\n'))
}

func (p *protocolInterceptor) handleCall(call hostCall) hostResponse {
	switch call.Fn {
	case "http_get":
		return p.httpGet(call.Args)
	case "kv_get":
		return p.kvGet(call.Args)
	case "kv_set":
		return p.kvSet(call.Args)
	default:
		return hostResponse{Error: fmt.Sprintf("unknown function: %s", call.Fn)}
	}
}

func (p *protocolInterceptor) httpGet(args map[string]any) hostResponse {
	url, ok := args["url"].(string)
	if !ok {
		return hostResponse{Error: "url required"}
	}

	if len(p.opts.AllowedHosts) > 0 {
		allowed := false
		for _, host := range p.opts.AllowedHosts {
			if strings.Contains(url, host) {
				allowed = true
				break
			}
		}
		if !allowed {
			return hostResponse{Error: fmt.Sprintf("host not allowed: %s", url)}
		}
	} else {
		return hostResponse{Error: "http not enabled"}
	}

	resp, err := http.Get(url)
	if err != nil {
		return hostResponse{Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
	if err != nil {
		return hostResponse{Error: err.Error()}
	}

	return hostResponse{Data: map[string]any{
		"status": resp.StatusCode,
		"body":   string(body),
	}}
}

var kvStore = make(map[string]string)
var kvMu sync.RWMutex

func (p *protocolInterceptor) kvGet(args map[string]any) hostResponse {
	key, ok := args["key"].(string)
	if !ok {
		return hostResponse{Error: "key required"}
	}

	kvMu.RLock()
	val, exists := kvStore[key]
	kvMu.RUnlock()

	if !exists {
		return hostResponse{Data: nil}
	}
	return hostResponse{Data: val}
}

func (p *protocolInterceptor) kvSet(args map[string]any) hostResponse {
	key, ok := args["key"].(string)
	if !ok {
		return hostResponse{Error: "key required"}
	}
	val, ok := args["value"].(string)
	if !ok {
		return hostResponse{Error: "value required"}
	}

	kvMu.Lock()
	kvStore[key] = val
	kvMu.Unlock()

	return hostResponse{Data: "ok"}
}

func (p *protocolInterceptor) RealStderr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.realStderr.String()
}
