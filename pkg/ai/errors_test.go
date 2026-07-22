package ai

import (
	"testing"
)

func TestParseAPIErrorAuth(t *testing.T) {
	ae := parseAPIError("openai", 401, `{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`)
	if ae.Kind != ErrKindAuth {
		t.Errorf("expected ErrKindAuth, got %d", ae.Kind)
	}
	if ae.Message != "Invalid API key" {
		t.Errorf("message: got %q", ae.Message)
	}
	if ae.Provider != "openai" {
		t.Errorf("provider: got %q", ae.Provider)
	}
}

func TestParseAPIErrorRateLimit(t *testing.T) {
	ae := parseAPIError("openai", 429, `{"error":{"message":"Rate limit reached","type":"rate_limit_error"}}`)
	if ae.Kind != ErrKindRateLimit {
		t.Errorf("expected ErrKindRateLimit, got %d", ae.Kind)
	}
}

func TestParseAPIErrorServerError(t *testing.T) {
	ae := parseAPIError("anthropic", 500, `{"error":{"message":"Internal server error"}}`)
	if ae.Kind != ErrKindServerError {
		t.Errorf("expected ErrKindServerError, got %d", ae.Kind)
	}
}

func TestParseAPIErrorOllamaStringError(t *testing.T) {
	ae := parseAPIError("ollama", 404, `{"error":"model not found"}`)
	if ae.Kind != ErrKindInvalidRequest {
		t.Errorf("expected ErrKindInvalidRequest, got %d", ae.Kind)
	}
	if ae.Message != "model not found" {
		t.Errorf("message: got %q", ae.Message)
	}
}

func TestParseAPIErrorEmptyBody(t *testing.T) {
	ae := parseAPIError("openai", 503, "")
	if ae.Kind != ErrKindServerError {
		t.Errorf("expected ErrKindServerError, got %d", ae.Kind)
	}
	if ae.Message == "" {
		t.Error("should have a fallback message")
	}
}

func TestParseAPIErrorMessageField(t *testing.T) {
	ae := parseAPIError("anthropic", 400, `{"message":"Invalid request body"}`)
	if ae.Message != "Invalid request body" {
		t.Errorf("message: got %q", ae.Message)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{&AIError{Kind: ErrKindAuth}, false},
		{&AIError{Kind: ErrKindInvalidRequest}, false},
		{&AIError{Kind: ErrKindContextCanceled}, false},
		{&AIError{Kind: ErrKindEmptyResponse}, false},
		{&AIError{Kind: ErrKindRateLimit}, true},
		{&AIError{Kind: ErrKindServerError}, true},
		{&AIError{Kind: ErrKindNetwork}, true},
		{nil, false},
	}
	for _, tt := range tests {
		if got := IsRetryable(tt.err); got != tt.expected {
			t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.expected)
		}
	}
}

func TestAIErrorString(t *testing.T) {
	ae := &AIError{
		Provider:   "openai",
		StatusCode: 401,
		Kind:       ErrKindAuth,
		Message:    "invalid key",
	}
	s := ae.Error()
	if s == "" {
		t.Error("error string should not be empty")
	}
}
