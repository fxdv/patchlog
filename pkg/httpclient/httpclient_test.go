package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	client := New(5 * time.Second)
	if client.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", client.Timeout)
	}
	if client.Transport == nil {
		t.Error("expected non-nil transport")
	}
}

func TestDefault(t *testing.T) {
	client := Default()
	if client.Timeout != DefaultTimeout {
		t.Errorf("expected %v, got %v", DefaultTimeout, client.Timeout)
	}
}

func TestStreaming(t *testing.T) {
	client := Streaming()
	if client.Timeout != StreamingTimeout {
		t.Errorf("expected %v, got %v", StreamingTimeout, client.Timeout)
	}
}

func TestReadResponseLimitRejectsOversizedBody(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("123456"))}
	if _, err := ReadResponseLimit(resp, 5); err == nil {
		t.Fatal("expected oversized response to fail")
	}
}

func TestDoWithRetrySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := DoWithRetry(client, req, DefaultRetry)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDoWithRetryRetries5xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("GET", server.URL, nil)
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetryExhausted(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("GET", server.URL, nil)
	cfg := RetryConfig{MaxAttempts: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoWithRetry429(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("GET", server.URL, nil)
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDoWithRetryNoRetry4xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("GET", server.URL, nil)
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on 400), got %d", attempts)
	}
}

func TestDoWithRetryNetworkError(t *testing.T) {
	client := Default()
	req, _ := http.NewRequest("GET", "http://127.0.0.1:0/nonexistent", nil)
	cfg := RetryConfig{MaxAttempts: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	_, err := DoWithRetry(client, req, cfg)
	if err == nil {
		t.Error("expected network error")
	}
}

func TestDoWithRetryContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := Default()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	cfg := RetryConfig{MaxAttempts: 5, BaseDelay: 1 * time.Second, MaxDelay: 2 * time.Second}
	_, err := DoWithRetry(client, req, cfg)
	if err == nil {
		t.Error("expected error due to cancelled context")
	}
}

func TestDoWithRetryRequestBody(t *testing.T) {
	bodyReceived := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyReceived = append(bodyReceived, string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("POST", server.URL, bytes.NewReader([]byte("payload")))
	cfg := RetryConfig{MaxAttempts: 1, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(bodyReceived) != 1 || bodyReceived[0] != "payload" {
		t.Errorf("expected body 'payload', got %v", bodyReceived)
	}
}

func TestDoWithRetryRequestBodyReplayed(t *testing.T) {
	bodyReceived := []string{}
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyReceived = append(bodyReceived, string(body))
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := Default()
	req, _ := http.NewRequest("POST", server.URL, bytes.NewReader([]byte("payload")))
	req.Header.Set("Idempotency-Key", "release-123")
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}
	resp, err := DoWithRetry(client, req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(bodyReceived) != 3 {
		t.Fatalf("expected 3 body reads, got %d", len(bodyReceived))
	}
	for i, b := range bodyReceived {
		if b != "payload" {
			t.Errorf("attempt %d: expected 'payload', got %q", i, b)
		}
	}
}

func TestDoWithRetryDoesNotRetryUnsafeRequestWithoutIdempotencyKey(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader("payload"))
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}
	resp, err := DoWithRetry(Default(), req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if attempts != 1 {
		t.Fatalf("POST attempts = %d, want 1", attempts)
	}
}

func TestClientRejectsCrossOriginRedirect(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirect.Close()

	_, err := Default().Get(redirect.URL)
	if err == nil || !strings.Contains(err.Error(), "cross-origin redirect") {
		t.Fatalf("cross-origin redirect error = %v", err)
	}
}

func TestBackoff(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 1 * time.Second, MaxDelay: 10 * time.Second}
	d0 := backoff(cfg, 0)
	d1 := backoff(cfg, 1)
	if d0 < cfg.BaseDelay {
		t.Errorf("backoff(0) should be >= BaseDelay, got %v", d0)
	}
	if d1 < cfg.BaseDelay*2 {
		t.Errorf("backoff(1) should be >= 2*BaseDelay, got %v", d1)
	}
}

func TestBackoffMaxDelay(t *testing.T) {
	cfg := RetryConfig{BaseDelay: 1 * time.Second, MaxDelay: 2 * time.Second}
	for i := 0; i < 20; i++ {
		d := backoff(cfg, 10)
		if d > cfg.MaxDelay+cfg.MaxDelay/2+1 {
			t.Errorf("backoff should not exceed MaxDelay+jitter significantly, got %v for attempt %d", d, i)
		}
	}
}
