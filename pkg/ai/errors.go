package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ErrorKind int

const (
	ErrKindUnknown ErrorKind = iota
	ErrKindAuth
	ErrKindRateLimit
	ErrKindServerError
	ErrKindNetwork
	ErrKindInvalidRequest
	ErrKindContextCanceled
	ErrKindEmptyResponse
)

type AIError struct {
	Provider   string
	StatusCode int
	Kind       ErrorKind
	Message    string
	Raw        string
}

func (e *AIError) Error() string {
	prefix := e.Provider
	if e.StatusCode > 0 {
		prefix = fmt.Sprintf("%s (HTTP %d)", e.Provider, e.StatusCode)
	}
	kindStr := ""
	switch e.Kind {
	case ErrKindAuth:
		kindStr = "authentication"
	case ErrKindRateLimit:
		kindStr = "rate limit"
	case ErrKindServerError:
		kindStr = "server error"
	case ErrKindNetwork:
		kindStr = "network"
	case ErrKindInvalidRequest:
		kindStr = "invalid request"
	case ErrKindContextCanceled:
		kindStr = "cancelled"
	case ErrKindEmptyResponse:
		kindStr = "empty response"
	}
	if kindStr != "" {
		return fmt.Sprintf("%s: %s: %s", prefix, kindStr, e.Message)
	}
	return fmt.Sprintf("%s: %s", prefix, e.Message)
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*AIError)
	if !ok {
		return true
	}
	switch ae.Kind {
	case ErrKindAuth, ErrKindInvalidRequest, ErrKindContextCanceled, ErrKindEmptyResponse:
		return false
	default:
		return true
	}
}

func parseAPIError(provider string, statusCode int, body string) *AIError {
	ae := &AIError{
		Provider:   provider,
		StatusCode: statusCode,
		Raw:        body,
	}

	msg := extractErrorMessage(body)
	if msg == "" {
		msg = truncateBody(body, 300)
	}
	if msg == "" {
		switch statusCode {
		case 401, 403:
			msg = "authentication failed — check your API key"
		case 404:
			msg = "resource not found — check model name and base URL"
		case 429:
			msg = "rate limit exceeded — too many requests"
		default:
			msg = fmt.Sprintf("unexpected status %d", statusCode)
		}
	}
	ae.Message = msg

	switch {
	case statusCode == 401 || statusCode == 403:
		ae.Kind = ErrKindAuth
	case statusCode == 429:
		ae.Kind = ErrKindRateLimit
	case statusCode >= 500:
		ae.Kind = ErrKindServerError
	case statusCode >= 400:
		ae.Kind = ErrKindInvalidRequest
	default:
		ae.Kind = ErrKindUnknown
	}

	return ae
}

func extractErrorMessage(body string) string {
	if body == "" {
		return ""
	}

	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &raw) == nil {
		if errVal, ok := raw["error"]; ok {
			var errStr string
			if json.Unmarshal(errVal, &errStr) == nil && errStr != "" {
				return errStr
			}
			var errObj struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}
			if json.Unmarshal(errVal, &errObj) == nil && errObj.Message != "" {
				return errObj.Message
			}
		}
		if msgVal, ok := raw["message"]; ok {
			var msgStr string
			if json.Unmarshal(msgVal, &msgStr) == nil && msgStr != "" {
				return msgStr
			}
		}
	}

	if strings.Contains(body, "model not found") || strings.Contains(body, "model does not exist") {
		return "model not found — check the model name in your config"
	}

	return ""
}

func truncateBody(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
