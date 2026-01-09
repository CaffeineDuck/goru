package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/caffeineduck/goru/hostfunc"
)

// Protocol constants - used by language stdlibs to communicate with the host.
// Format: \x00GORU:{json}\x00
const (
	protocolPrefix = "\x00GORU:"
	protocolSuffix = "\x00"
)

type callRequest struct {
	Fn   string         `json:"fn"`
	Args map[string]any `json:"args"`
}

type callResponse struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// protocolHandler intercepts stderr to handle host function calls.
// Regular stderr output passes through; protocol messages trigger host calls.
type protocolHandler struct {
	ctx         context.Context
	registry    *hostfunc.Registry
	stdinWriter *io.PipeWriter
	realStderr  bytes.Buffer
	buf         bytes.Buffer
	mu          sync.Mutex
}

func newProtocolHandler(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *protocolHandler {
	return &protocolHandler{
		ctx:         ctx,
		registry:    registry,
		stdinWriter: stdinWriter,
	}
}

func (p *protocolHandler) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buf.Write(data)

	for {
		content := p.buf.String()
		startIdx := strings.Index(content, protocolPrefix)
		if startIdx == -1 {
			p.realStderr.WriteString(content)
			p.buf.Reset()
			break
		}

		p.realStderr.WriteString(content[:startIdx])

		endIdx := strings.Index(content[startIdx+len(protocolPrefix):], protocolSuffix)
		if endIdx == -1 {
			p.buf.Reset()
			p.buf.WriteString(content[startIdx:])
			break
		}

		jsonStr := content[startIdx+len(protocolPrefix) : startIdx+len(protocolPrefix)+endIdx]
		p.buf.Reset()
		p.buf.WriteString(content[startIdx+len(protocolPrefix)+endIdx+1:])

		var req callRequest
		if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
			p.respond(callResponse{Error: "invalid call format"})
			continue
		}

		resp := p.handleCall(req)
		p.respond(resp)
	}

	return len(data), nil
}

func (p *protocolHandler) respond(resp callResponse) {
	data, _ := json.Marshal(resp)
	go p.stdinWriter.Write(append(data, '\n'))
}

func (p *protocolHandler) handleCall(req callRequest) callResponse {
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

func (p *protocolHandler) Stderr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.realStderr.String()
}
