package sandbox

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/caffeineduck/goru/hostfunc"
)

func TestProtocolParsesValidCall(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("echo", func(ctx context.Context, args map[string]any) (any, error) {
		return args["msg"], nil
	})

	_, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	handler.Write([]byte("\x00GORU:{\"fn\":\"echo\",\"args\":{\"msg\":\"hello\"}}\x00"))

	stderr := handler.Stderr()
	if stderr != "" {
		t.Errorf("expected no stderr output, got %q", stderr)
	}
}

func TestProtocolPassesThroughNonProtocolData(t *testing.T) {
	registry := hostfunc.NewRegistry()
	_, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	handler.Write([]byte("normal stderr output"))

	stderr := handler.Stderr()
	if stderr != "normal stderr output" {
		t.Errorf("expected 'normal stderr output', got %q", stderr)
	}
}

func TestProtocolHandlesMixedContent(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("noop", func(ctx context.Context, args map[string]any) (any, error) {
		return nil, nil
	})

	_, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	handler.Write([]byte("before\x00GORU:{\"fn\":\"noop\",\"args\":{}}\x00after"))

	stderr := handler.Stderr()
	if stderr != "beforeafter" {
		t.Errorf("expected 'beforeafter', got %q", stderr)
	}
}

func TestProtocolHandlesMalformedJSON(t *testing.T) {
	registry := hostfunc.NewRegistry()
	_, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	handler.Write([]byte("\x00GORU:{invalid}\x00continue"))

	stderr := handler.Stderr()
	if stderr != "continue" {
		t.Errorf("expected 'continue', got %q", stderr)
	}
}

func TestProtocolHandlesUnknownFunction(t *testing.T) {
	registry := hostfunc.NewRegistry()
	stdinReader, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	go func() {
		handler.Write([]byte("\x00GORU:{\"fn\":\"unknown\",\"args\":{}}\x00"))
	}()

	buf := make([]byte, 1024)
	n, _ := stdinReader.Read(buf)
	response := string(buf[:n])

	if !strings.Contains(response, "unknown function") {
		t.Errorf("expected 'unknown function' error, got %q", response)
	}
}

func TestProtocolHandlesPartialMessage(t *testing.T) {
	registry := hostfunc.NewRegistry()
	_, stdinWriter := io.Pipe()
	handler := newProtocolHandler(context.Background(), registry, stdinWriter)

	// Send partial message in chunks
	handler.Write([]byte("prefix\x00GORU:{\"fn\":"))
	handler.Write([]byte("\"test\",\"args\":{}}\x00suffix"))

	stderr := handler.Stderr()
	if stderr != "prefixsuffix" {
		t.Errorf("expected 'prefixsuffix', got %q", stderr)
	}
}
