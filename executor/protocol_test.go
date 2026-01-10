package executor

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFindNextMessage(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantIdx     int
		wantMsgType messageType
	}{
		{"no message", "hello world", -1, messageNone},
		{"call message", "prefix\x00GORU:{}\x00suffix", 6, messageCall},
		{"flush message", "prefix\x00GORU_FLUSH:5\x00suffix", 6, messageFlush},
		{"call before flush", "\x00GORU:{}\x00\x00GORU_FLUSH:1\x00", 0, messageCall},
		{"flush before call", "\x00GORU_FLUSH:1\x00\x00GORU:{}\x00", 0, messageFlush},
		{"empty content", "", -1, messageNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, msgType := findNextMessage(tt.content)
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
			if msgType != tt.wantMsgType {
				t.Errorf("msgType = %d, want %d", msgType, tt.wantMsgType)
			}
		})
	}
}

func TestExtractMessage(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		idx           int
		prefix        string
		wantPayload   string
		wantRemaining string
		wantOK        bool
	}{
		{
			name:          "valid call",
			content:       `prefix` + "\x00GORU:{\"fn\":\"test\"}\x00" + `suffix`,
			idx:           6,
			prefix:        protocolPrefix,
			wantPayload:   `{"fn":"test"}`,
			wantRemaining: "suffix",
			wantOK:        true,
		},
		{
			name:          "incomplete message",
			content:       "prefix\x00GORU:{partial",
			idx:           6,
			prefix:        protocolPrefix,
			wantPayload:   "",
			wantRemaining: "\x00GORU:{partial",
			wantOK:        false,
		},
		{
			name:          "valid flush",
			content:       "\x00GORU_FLUSH:10\x00remaining",
			idx:           0,
			prefix:        protocolFlushPrefix,
			wantPayload:   "10",
			wantRemaining: "remaining",
			wantOK:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, remaining, ok := extractMessage(tt.content, tt.idx, tt.prefix)
			if payload != tt.wantPayload {
				t.Errorf("payload = %q, want %q", payload, tt.wantPayload)
			}
			if remaining != tt.wantRemaining {
				t.Errorf("remaining = %q, want %q", remaining, tt.wantRemaining)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

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
