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

// Protocol constants - used by language stdlibs to communicate with the host.
// Format: \x00GORU:{json}\x00 for calls, \x00GORU_FLUSH:N\x00 for async batch
const (
	protocolPrefix      = "\x00GORU:"
	protocolFlushPrefix = "\x00GORU_FLUSH:"
	protocolSuffix      = "\x00"
)

type callRequest struct {
	ID   string         `json:"id,omitempty"` // For async calls
	Fn   string         `json:"fn"`
	Args map[string]any `json:"args"`
}

type callResponse struct {
	ID    string `json:"id,omitempty"` // For async calls
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// protocolHandler intercepts stderr to handle host function calls.
// Regular stderr output passes through; protocol messages trigger host calls.
// Supports both sync (immediate response) and async (batched) execution.
type protocolHandler struct {
	ctx         context.Context
	registry    *hostfunc.Registry
	stdinWriter *io.PipeWriter
	realStderr  bytes.Buffer
	buf         bytes.Buffer
	pending     []callRequest // Pending async requests
	mu          sync.Mutex
	writeMu     sync.Mutex // Separate mutex for stdin writes
}

func newProtocolHandler(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *protocolHandler {
	return &protocolHandler{
		ctx:         ctx,
		registry:    registry,
		stdinWriter: stdinWriter,
		pending:     make([]callRequest, 0),
	}
}

func (p *protocolHandler) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buf.Write(data)

	for {
		content := p.buf.String()

		// Check for FLUSH command first
		flushIdx := strings.Index(content, protocolFlushPrefix)
		callIdx := strings.Index(content, protocolPrefix)

		// Determine which comes first
		var nextIdx int
		var isFlush bool
		if flushIdx == -1 && callIdx == -1 {
			// No protocol messages, write everything to stderr
			p.realStderr.WriteString(content)
			p.buf.Reset()
			break
		} else if flushIdx == -1 {
			nextIdx = callIdx
			isFlush = false
		} else if callIdx == -1 {
			nextIdx = flushIdx
			isFlush = true
		} else if flushIdx < callIdx {
			nextIdx = flushIdx
			isFlush = true
		} else {
			nextIdx = callIdx
			isFlush = false
		}

		// Write any content before the protocol message to stderr
		if nextIdx > 0 {
			p.realStderr.WriteString(content[:nextIdx])
		}

		if isFlush {
			// Handle FLUSH command
			prefix := protocolFlushPrefix
			endIdx := strings.Index(content[nextIdx+len(prefix):], protocolSuffix)
			if endIdx == -1 {
				p.buf.Reset()
				p.buf.WriteString(content[nextIdx:])
				break
			}

			countStr := content[nextIdx+len(prefix) : nextIdx+len(prefix)+endIdx]
			p.buf.Reset()
			p.buf.WriteString(content[nextIdx+len(prefix)+endIdx+1:])

			count, err := strconv.Atoi(countStr)
			if err != nil || count <= 0 {
				continue
			}

			p.handleFlush(count)
		} else {
			// Handle regular call
			prefix := protocolPrefix
			endIdx := strings.Index(content[nextIdx+len(prefix):], protocolSuffix)
			if endIdx == -1 {
				p.buf.Reset()
				p.buf.WriteString(content[nextIdx:])
				break
			}

			jsonStr := content[nextIdx+len(prefix) : nextIdx+len(prefix)+endIdx]
			p.buf.Reset()
			p.buf.WriteString(content[nextIdx+len(prefix)+endIdx+1:])

			var req callRequest
			if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
				p.respond(callResponse{Error: "invalid call format"})
				continue
			}

			if req.ID != "" {
				// Async call - queue it
				p.pending = append(p.pending, req)
			} else {
				// Sync call - handle immediately
				resp := p.handleCall(req)
				p.respond(resp)
			}
		}
	}

	return len(data), nil
}

// handleFlush processes pending async requests concurrently.
func (p *protocolHandler) handleFlush(count int) {
	// Take up to 'count' pending requests
	if count > len(p.pending) {
		count = len(p.pending)
	}
	if count == 0 {
		return
	}

	requests := p.pending[:count]
	p.pending = p.pending[count:]

	// Process all requests concurrently
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
	data, _ := json.Marshal(resp)
	// Write async to avoid deadlock - WASM may still be blocked in stderr write
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
