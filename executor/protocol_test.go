package executor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCallRequestJSON(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantFn  string
		wantID  string
		wantErr bool
	}{
		{
			name:    "sync call",
			jsonStr: `{"fn":"time_now","args":{}}`,
			wantFn:  "time_now",
			wantID:  "",
		},
		{
			name:    "async call",
			jsonStr: `{"id":"1","fn":"http_request","args":{"method":"GET","url":"https://example.com"}}`,
			wantFn:  "http_request",
			wantID:  "1",
		},
		{
			name:    "invalid json",
			jsonStr: `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req callRequest
			err := json.Unmarshal([]byte(tt.jsonStr), &req)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Fn != tt.wantFn {
				t.Errorf("fn = %q, want %q", req.Fn, tt.wantFn)
			}
			if req.ID != tt.wantID {
				t.Errorf("id = %q, want %q", req.ID, tt.wantID)
			}
		})
	}
}

func TestCallResponseJSON(t *testing.T) {
	tests := []struct {
		name     string
		resp     callResponse
		wantJSON string
	}{
		{
			name:     "success response",
			resp:     callResponse{Data: "value"},
			wantJSON: `"data":"value"`,
		},
		{
			name:     "error response",
			resp:     callResponse{Error: "something failed"},
			wantJSON: `"error":"something failed"`,
		},
		{
			name:     "async response",
			resp:     callResponse{ID: "42", Data: "result"},
			wantJSON: `"id":"42"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.resp)
			if !strings.Contains(string(data), tt.wantJSON) {
				t.Errorf("json = %q, want to contain %q", string(data), tt.wantJSON)
			}
		})
	}
}

func TestProtocolSplitFrameAndStderr(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	p := newSessionProtocol(context.Background(), nil, writer)
	for _, ch := range []byte("before\x00GORU_READY\x00after\x00GORU_DONE\x00") {
		if _, err := p.Write([]byte{ch}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-p.readyCh:
	default:
		t.Fatal("split ready frame was lost")
	}
	select {
	case <-p.doneCh:
	default:
		t.Fatal("split done frame was lost")
	}
	if p.Stderr() != "beforeafter" {
		t.Fatalf("stderr = %q", p.Stderr())
	}
}

func TestProtocolFrameLimit(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	p := newProtocolHandler(context.Background(), nil, writer)
	p.Write([]byte("\x00GORU:"))
	for i := 0; i < maxFrameBytes; i++ {
		if _, err := p.Write([]byte("x")); err != nil {
			if !errors.Is(err, errProtocolLimit) || len(p.frame) > maxFrameBytes {
				t.Fatalf("unexpected frame failure: %v", err)
			}
			return
		}
	}
	t.Fatal("oversized frame was retained")
}

func TestProtocolPendingLimit(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	p := newProtocolHandler(context.Background(), nil, writer)
	frame := []byte("\x00GORU:{\"id\":\"1\",\"fn\":\"test\",\"args\":{}}\x00")
	for i := 0; i < maxPendingCalls; i++ {
		if _, err := p.Write(frame); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := p.Write(frame); !errors.Is(err, errProtocolLimit) {
		t.Fatalf("pending limit: %v", err)
	}
}

func TestProtocolOutputLimits(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	p := newProtocolHandler(context.Background(), nil, writer)
	chunk := []byte(strings.Repeat("x", maxCaptureBytes+1))
	var stdout cappedOutput
	stdout.Write(chunk)
	p.Write(chunk)
	if len(stdout.String()) != maxCaptureBytes || len(p.Stderr()) != maxCaptureBytes {
		t.Fatal("output was not capped")
	}
	data := encodeResponse(callResponse{ID: "1", Data: map[string]any{"text": strings.Repeat("x", maxResponseBytes)}})
	var reply callResponse
	if err := json.Unmarshal(data, &reply); err != nil || reply.ID != "1" || reply.Error == "" {
		t.Fatalf("response limit: %q (%v)", data[:min(100, len(data))], err)
	}
}
