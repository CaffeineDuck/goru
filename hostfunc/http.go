package hostfunc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxURLLength   = 8192
	DefaultMaxBodySize    = 1 << 20 // 1MB
	DefaultRequestTimeout = 30 * time.Second
)

type HTTPConfig struct {
	AllowedHosts   []string
	MaxBodySize    int64
	MaxURLLength   int
	RequestTimeout time.Duration
}

type HTTP struct {
	cfg    HTTPConfig
	client *http.Client
}

func NewHTTP(cfg HTTPConfig) *HTTP {
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = DefaultMaxBodySize
	}
	if cfg.MaxURLLength == 0 {
		cfg.MaxURLLength = DefaultMaxURLLength
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = DefaultRequestTimeout
	}

	return &HTTP{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	}
}

func (h *HTTP) Request(ctx context.Context, args map[string]any) (any, error) {
	method, _ := args["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	rawURL, ok := args["url"].(string)
	if !ok || rawURL == "" {
		return nil, fmt.Errorf("url required")
	}

	if len(rawURL) > h.cfg.MaxURLLength {
		return nil, fmt.Errorf("url exceeds max length")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("scheme must be http or https")
	}

	if len(h.cfg.AllowedHosts) == 0 {
		return nil, fmt.Errorf("http not enabled")
	}

	host := parsed.Hostname()
	if !h.isHostAllowed(host) {
		return nil, fmt.Errorf("host not allowed: %s", host)
	}

	var body io.Reader
	if bodyStr, ok := args["body"].(string); ok && bodyStr != "" {
		if int64(len(bodyStr)) > h.cfg.MaxBodySize {
			return nil, fmt.Errorf("request body exceeds max size")
		}
		body = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if headers, ok := args["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, h.cfg.MaxBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return map[string]any{
		"status":  resp.StatusCode,
		"body":    string(respBody),
		"headers": respHeaders,
	}, nil
}

func (h *HTTP) isHostAllowed(host string) bool {
	for _, allowed := range h.cfg.AllowedHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

func NewHTTPGet(cfg HTTPConfig) Func {
	h := NewHTTP(cfg)
	return func(ctx context.Context, args map[string]any) (any, error) {
		args["method"] = "GET"
		return h.Request(ctx, args)
	}
}
