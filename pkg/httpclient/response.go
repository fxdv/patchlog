package httpclient

import (
	"fmt"
	"io"
	"net/http"
)

const DefaultMaxResponseBytes int64 = 10 << 20

// ReadResponse closes neither the response nor its body. It rejects oversized
// payloads instead of silently truncating JSON or error responses.
func ReadResponse(resp *http.Response) ([]byte, error) {
	return ReadResponseLimit(resp, DefaultMaxResponseBytes)
}

func ReadResponseLimit(resp *http.Response, maxBytes int64) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("HTTP response body is nil")
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("HTTP response limit must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("HTTP response exceeds %d-byte limit", maxBytes)
	}
	return data, nil
}
