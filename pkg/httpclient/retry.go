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
	retryable := retryableRequest(req)
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
			if !retryable || attempt >= cfg.MaxAttempts-1 {
				return nil, err
			}
			select {
			case <-time.After(backoff(cfg, attempt)):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			continue
		}

		shouldRetry := retryable && (resp.StatusCode >= 500 || resp.StatusCode == 429)
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

// retryableRequest is deliberately conservative. Unsafe methods are retried
// only when the caller supplies an idempotency key understood by the server.
func retryableRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	default:
		return req.Header.Get("Idempotency-Key") != ""
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
