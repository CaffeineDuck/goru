package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/caffeineduck/goru/hostfunc"
)

const (
	protocolPrefix      = "\x00GORU:"
	protocolFlushPrefix = "\x00GORU_FLUSH:"
	protocolSuffix      = "\x00"
)

type callRequest struct {
	ID   string         `json:"id,omitempty"`
	Fn   string         `json:"fn"`
	Args map[string]any `json:"args"`
}

type callResponse struct {
	ID    string `json:"id,omitempty"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

type protocolHandler struct {
	ctx         context.Context
	registry    *hostfunc.Registry
	stdinWriter *io.PipeWriter
	realStderr  bytes.Buffer
	buf         bytes.Buffer
	pending     []callRequest
	mu          sync.Mutex
	writeMu     sync.Mutex
}

func newProtocolHandler(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *protocolHandler {
	return &protocolHandler{
		ctx:         ctx,
		registry:    registry,
		stdinWriter: stdinWriter,
		pending:     make([]callRequest, 0),
	}
}

type messageType int

const (
	messageNone messageType = iota
	messageCall
	messageFlush
)

func findNextMessage(content string) (idx int, msgType messageType) {
	flushIdx := strings.Index(content, protocolFlushPrefix)
	callIdx := strings.Index(content, protocolPrefix)

	switch {
	case flushIdx == -1 && callIdx == -1:
		return -1, messageNone
	case flushIdx == -1:
		return callIdx, messageCall
	case callIdx == -1:
		return flushIdx, messageFlush
	case flushIdx < callIdx:
		return flushIdx, messageFlush
	default:
		return callIdx, messageCall
	}
}

func extractMessage(content string, idx int, prefix string) (payload, remaining string, ok bool) {
	afterPrefix := content[idx+len(prefix):]
	payload, remaining, ok = strings.Cut(afterPrefix, protocolSuffix)
	if !ok {
		return "", content[idx:], false
	}
	return payload, remaining, true
}

func (p *protocolHandler) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buf.Write(data)

	for {
		content := p.buf.String()
		idx, msgType := findNextMessage(content)

		if msgType == messageNone {
			p.realStderr.WriteString(content)
			p.buf.Reset()
			break
		}

		if idx > 0 {
			p.realStderr.WriteString(content[:idx])
		}

		switch msgType {
		case messageFlush:
			if !p.processFlushMessage(content, idx) {
				return len(data), nil
			}
		case messageCall:
			if !p.processCallMessage(content, idx) {
				return len(data), nil
			}
		}
	}

	return len(data), nil
}

func (p *protocolHandler) processFlushMessage(content string, idx int) bool {
	payload, remaining, ok := extractMessage(content, idx, protocolFlushPrefix)
	if !ok {
		p.buf.Reset()
		p.buf.WriteString(remaining)
		return false
	}

	p.buf.Reset()
	p.buf.WriteString(remaining)

	count, err := strconv.Atoi(payload)
	if err != nil || count <= 0 {
		return true
	}

	p.handleFlush(count)
	return true
}

func (p *protocolHandler) processCallMessage(content string, idx int) bool {
	payload, remaining, ok := extractMessage(content, idx, protocolPrefix)
	if !ok {
		p.buf.Reset()
		p.buf.WriteString(remaining)
		return false
	}

	p.buf.Reset()
	p.buf.WriteString(remaining)

	var req callRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		p.respond(callResponse{Error: "invalid call format"})
		return true
	}

	if req.ID != "" {
		p.pending = append(p.pending, req)
	} else {
		p.respond(p.handleCall(req))
	}
	return true
}

func (p *protocolHandler) handleFlush(count int) {
	if count > len(p.pending) {
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
			resp := p.handleCall(r)
			resp.ID = r.ID
			p.respond(resp)
		}(req)
	}

	wg.Wait()
}

func (p *protocolHandler) respond(resp callResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		data = []byte(`{"error":"internal: failed to marshal response"}`)
	}
	go func() {
		p.writeMu.Lock()
		defer p.writeMu.Unlock()
		p.stdinWriter.Write(append(data, '\n'))
	}()
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
