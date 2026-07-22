package httpclient

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

var DefaultRetry = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    10 * time.Second,
}

func DoWithRetry(client *http.Client, req *http.Request, cfg RetryConfig) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}

	for attempt := 0; ; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx := req.Context(); ctx != nil && ctx.Err() != nil {
				return nil, err
			}
			if attempt >= cfg.MaxAttempts-1 {
				return nil, err
			}
			select {
			case <-time.After(backoff(cfg, attempt)):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			continue
		}

		shouldRetry := resp.StatusCode >= 500 || resp.StatusCode == 429
		if !shouldRetry || attempt >= cfg.MaxAttempts-1 {
			return resp, nil
		}

		resp.Body.Close()
		if ctx := req.Context(); ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		select {
		case <-time.After(backoff(cfg, attempt)):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
}

func backoff(cfg RetryConfig, attempt int) time.Duration {
	delay := cfg.BaseDelay * time.Duration(1<<uint(attempt))
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	jitter := time.Duration(rand.Int63n(int64(delay/2 + 1)))
	return delay + jitter
}
