package hostfunc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type HTTPConfig struct {
	AllowedHosts []string
	MaxBodySize  int64
}

func NewHTTPGet(cfg HTTPConfig) Func {
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 1024 * 1024 // 1MB default
	}

	return func(ctx context.Context, args map[string]any) (any, error) {
		rawURL, ok := args["url"].(string)
		if !ok {
			return nil, errors.New("url required")
		}

		if len(cfg.AllowedHosts) == 0 {
			return nil, errors.New("http not enabled")
		}

		parsed, err := url.Parse(rawURL)
		if err != nil {
			return nil, errors.New("invalid url")
		}

		host := parsed.Hostname()
		allowed := false
		for _, allowedHost := range cfg.AllowedHosts {
			if host == allowedHost || strings.HasSuffix(host, "."+allowedHost) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errors.New("host not allowed: " + host)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxBodySize))
		if err != nil {
			return nil, err
		}

		return map[string]any{
			"status": resp.StatusCode,
			"body":   string(body),
		}, nil
	}
}
