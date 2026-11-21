package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/caffeineduck/goru/hostfunc"
)

const (
	protocolPrefix      = "GORU:"
	protocolFlushPrefix = "GORU_FLUSH:"
	maxFrameBytes       = 64 << 10
	maxPendingCalls     = 16
	maxResponseBytes    = 4 << 20
	maxCaptureBytes     = 1 << 20
)

// ErrProtocolLimit indicates that execution stopped at a guest protocol limit.
var ErrProtocolLimit = errors.New("protocol-limit: goru protocol limit exceeded")

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

type cappedOutput struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (o *cappedOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Write(data[:min(len(data), maxCaptureBytes-o.buf.Len())])
	return len(data), nil
}

func (o *cappedOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func (o *cappedOutput) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Reset()
}

type protocolJob struct {
	request callRequest
	valid   bool
}

type protocolHandler struct {
	ctx         context.Context
	registry    *hostfunc.Registry
	stdinWriter *io.PipeWriter
	realStderr  cappedOutput
	frame       []byte
	framing     bool
	matched     bool
	pending     []callRequest
	cancel      context.CancelFunc
	jobs        chan protocolJob
	workers     sync.Once
	session     *sessionProtocol
	mu          sync.Mutex
	writeMu     sync.Mutex
	err         error
}

func newProtocolHandler(ctx context.Context, registry *hostfunc.Registry, stdinWriter *io.PipeWriter) *protocolHandler {
	return &protocolHandler{
		ctx: ctx, registry: registry, stdinWriter: stdinWriter,
		jobs: make(chan protocolJob, maxPendingCalls),
	}
}

func (p *protocolHandler) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(data)
	for len(data) > 0 && p.err == nil {
		if p.framing && !p.matched {
			ch := data[0]
			if ch != 0 {
				p.frame = append(p.frame, ch)
				data = data[1:]
				if p.matchPrefix() {
					continue
				}
			}
			p.realStderr.Write([]byte{0})
			p.realStderr.Write(p.frame)
			p.frame = p.frame[:0]
			p.framing = false
			continue
		}
		n := bytes.IndexByte(data, 0)
		if n < 0 {
			n = len(data)
		}
		if p.framing {
			if len(p.frame)+n > maxFrameBytes {
				p.fail(ErrProtocolLimit)
				break
			}
			p.frame = append(p.frame, data[:n]...)
			if n < len(data) {
				p.handleFrame(string(p.frame))
				p.frame = p.frame[:0]
				p.matched = false
			}
		} else {
			p.realStderr.Write(data[:n])
		}
		if n == len(data) {
			break
		}
		p.framing = !p.framing
		data = data[n+1:]
	}
	return total, p.err
}

func (p *protocolHandler) matchPrefix() bool {
	candidate := string(p.frame)
	for _, prefix := range [...]string{protocolPrefix, protocolFlushPrefix,
		"GORU_READY", "GORU_DONE", "GORU_ERROR:", "GORU_COMPLETE", "GORU_INCOMPLETE"} {
		if strings.HasPrefix(prefix, candidate) {
			p.matched = prefix == candidate
			return true
		}
	}
	return false
}

func (p *protocolHandler) fail(err error) {
	if p.err != nil {
		return
	}
	p.err = err
	p.stdinWriter.CloseWithError(err)
	if p.session != nil {
		close(p.session.failed)
	}
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *protocolHandler) handleFrame(frame string) {
	if payload, ok := strings.CutPrefix(frame, protocolPrefix); ok {
		var req callRequest
		if err := json.Unmarshal([]byte(payload), &req); err != nil {
			p.submit(callRequest{}, false)
			return
		}
		if req.ID != "" && len(p.pending) >= maxPendingCalls {
			p.fail(ErrProtocolLimit)
			return
		}
		if req.ID == "" {
			p.submit(req, true)
			return
		}
		p.pending = append(p.pending, req)
		return
	}
	if payload, ok := strings.CutPrefix(frame, protocolFlushPrefix); ok {
		count, err := strconv.Atoi(payload)
		if err != nil || count <= 0 {
			return
		}
		for _, req := range p.pending[:min(count, len(p.pending))] {
			if p.err != nil {
				break
			}
			p.submit(req, true)
		}
		count = min(count, len(p.pending))
		clear(p.pending[:count])
		p.pending = p.pending[count:]
		return
	}
	if p.session != nil && p.session.signal(frame) {
		return
	}
	p.realStderr.Write([]byte{0})
	p.realStderr.Write([]byte(frame))
	p.realStderr.Write([]byte{0})
}

func (p *protocolHandler) submit(req callRequest, valid bool) {
	p.workers.Do(func() {
		for i := 0; i < maxPendingCalls; i++ {
			go p.serveJobs()
		}
	})
	select {
	case p.jobs <- protocolJob{request: req, valid: valid}:
	default:
		p.fail(ErrProtocolLimit)
	}
}

func (p *protocolHandler) serveJobs() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case job := <-p.jobs:
			if p.ctx.Err() != nil {
				return
			}
			p.reply(job.request, job.valid)
		}
	}
}

func (p *protocolHandler) reply(req callRequest, valid bool) {
	resp := callResponse{ID: req.ID, Error: "invalid call format"}
	if valid {
		resp = callResponse{ID: req.ID}
		if fn, ok := p.registry.Get(req.Fn); ok {
			result, err := fn(p.ctx, req.Args)
			if err != nil {
				resp.Error = err.Error()
			} else {
				resp.Data = result
			}
		} else {
			resp.Error = "unknown function: " + req.Fn
		}
	}
	data := encodeResponse(resp)
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	p.stdinWriter.Write(append(data, '\n'))
}

func (p *protocolHandler) Stderr() string {
	return p.realStderr.String()
}
