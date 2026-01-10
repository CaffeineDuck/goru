package hostfunc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPGetBlockedWhenNoHosts(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: nil})
	_, err := fn(context.Background(), map[string]any{"url": "https://example.com"})
	if err == nil || err.Error() != "http not enabled" {
		t.Errorf("expected 'http not enabled', got %v", err)
	}
}

func TestHTTPGetBlockedForUnallowedHost(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"allowed.com"}})
	_, err := fn(context.Background(), map[string]any{"url": "https://evil.com"})
	if err == nil || err.Error() != "host not allowed: evil.com" {
		t.Errorf("expected 'host not allowed', got %v", err)
	}
}

func TestHTTPGetBypassQueryParam(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"allowed.com"}})
	_, err := fn(context.Background(), map[string]any{"url": "https://evil.com/?x=allowed.com"})
	if err == nil || err.Error() != "host not allowed: evil.com" {
		t.Errorf("query param bypass should be blocked, got %v", err)
	}
}

func TestHTTPGetBypassSubdomainSuffix(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"allowed.com"}})
	_, err := fn(context.Background(), map[string]any{"url": "https://allowed.com.evil.com/"})
	if err == nil || err.Error() != "host not allowed: allowed.com.evil.com" {
		t.Errorf("subdomain suffix bypass should be blocked, got %v", err)
	}
}

func TestHTTPGetAllowsExactHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	// Extract host from server URL (e.g., "127.0.0.1:12345")
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"127.0.0.1"}})
	result, err := fn(context.Background(), map[string]any{"url": server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data := result.(map[string]any)
	if data["status"].(int) != 200 {
		t.Errorf("expected status 200, got %v", data["status"])
	}
}

func TestHTTPGetAllowsSubdomain(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"example.com"}})
	// api.example.com should be allowed because it ends with .example.com
	// We can't test actual request without mocking, but we can test the allowlist logic
	// by checking that it doesn't error on URL parsing
	_, err := fn(context.Background(), map[string]any{"url": "https://api.example.com/test"})
	// This will fail with connection error, not "host not allowed"
	if err != nil && err.Error() == "host not allowed: api.example.com" {
		t.Error("subdomain should be allowed")
	}
}

func TestHTTPGetMissingURL(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"example.com"}})
	_, err := fn(context.Background(), map[string]any{})
	if err == nil || err.Error() != "url required" {
		t.Errorf("expected 'url required', got %v", err)
	}
}

func TestHTTPGetInvalidURL(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"example.com"}})
	_, err := fn(context.Background(), map[string]any{"url": "://invalid"})
	if err == nil || err.Error() != "invalid url" {
		t.Errorf("expected 'invalid url', got %v", err)
	}
}

// Security tests

func TestHTTPGetURLTooLong(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{
		AllowedHosts: []string{"example.com"},
		MaxURLLength: 100,
	})

	longURL := "https://example.com/" + string(make([]byte, 200))
	_, err := fn(context.Background(), map[string]any{"url": longURL})
	if err == nil {
		t.Error("expected long URL to be rejected")
	}
	if err.Error() != "url exceeds max length" {
		t.Errorf("expected 'url exceeds max length' error, got %v", err)
	}
}

func TestHTTPGetDefaultMaxURLLength(t *testing.T) {
	fn := NewHTTPGet(HTTPConfig{AllowedHosts: []string{"example.com"}})

	longURL := "https://example.com/" + string(make([]byte, 10*1024))
	_, err := fn(context.Background(), map[string]any{"url": longURL})
	if err == nil {
		t.Error("expected long URL to be rejected by default")
	}
	if err.Error() != "url exceeds max length" {
		t.Errorf("expected 'url exceeds max length' error, got %v", err)
	}
}

func TestHTTPGetIPv6Normalization(t *testing.T) {
	h := NewHTTP(HTTPConfig{AllowedHosts: []string{"::1"}})

	tests := []struct {
		host    string
		allowed bool
	}{
		{"::1", true},
		{"0:0:0:0:0:0:0:1", true}, // expanded form
		{"::2", false},
		{"example.com", false}, // domain shouldn't match IP
	}

	for _, tc := range tests {
		got := h.isHostAllowed(tc.host)
		if got != tc.allowed {
			t.Errorf("isHostAllowed(%q) = %v, want %v", tc.host, got, tc.allowed)
		}
	}
}

func TestHTTPGetIPv6NoSubdomainBypass(t *testing.T) {
	h := NewHTTP(HTTPConfig{AllowedHosts: []string{"example.com"}})

	// IP addresses should not match domain allowlists via subdomain logic
	tests := []string{
		"::1",
		"127.0.0.1",
		"192.168.1.1",
		"2001:db8::1",
	}

	for _, host := range tests {
		if h.isHostAllowed(host) {
			t.Errorf("IP %q should not match domain allowlist", host)
		}
	}
}

func TestHTTPGetIPv4Matching(t *testing.T) {
	h := NewHTTP(HTTPConfig{AllowedHosts: []string{"192.168.1.1"}})

	tests := []struct {
		host    string
		allowed bool
	}{
		{"192.168.1.1", true},
		{"192.168.1.2", false},
		{"example.com", false},
	}

	for _, tc := range tests {
		got := h.isHostAllowed(tc.host)
		if got != tc.allowed {
			t.Errorf("isHostAllowed(%q) = %v, want %v", tc.host, got, tc.allowed)
		}
	}
}
