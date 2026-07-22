package ai

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

type streamOptions struct {
	maxRetries int
	provider   string
}

func streamSSE(req *http.Request, onToken func(string), parseLine func(line string) (text string, done bool)) (string, error) {
	return streamSSEWithRetry(req, onToken, parseLine, streamOptions{maxRetries: 2, provider: "ai"})
}

func streamSSEWithRetry(req *http.Request, onToken func(string), parseLine func(line string) (text string, done bool), opts streamOptions) (string, error) {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return "", &AIError{
				Provider: opts.provider,
				Kind:     ErrKindNetwork,
				Message:  fmt.Sprintf("failed to read request body: %s", err.Error()),
			}
		}
	}

	var lastErr error
	for attempt := 0; attempt <= opts.maxRetries; attempt++ {
		if bodyBytes != nil {
			req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		}

		text, err, receivedTokens := streamSSEOnce(req, onToken, parseLine, opts.provider)
		if err == nil {
			return text, nil
		}
		lastErr = err

		if receivedTokens {
			return text, err
		}

		if !IsRetryable(err) {
			return text, err
		}

		if ctx := req.Context(); ctx != nil && ctx.Err() != nil {
			return text, err
		}

		if attempt < opts.maxRetries {
			delay := time.Duration(1<<uint(attempt)) * time.Second
			if ctx := req.Context(); ctx != nil {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return text, ctx.Err()
				}
			} else {
				time.Sleep(delay)
			}
		}
	}

	return "", lastErr
}

func streamSSEOnce(req *http.Request, onToken func(string), parseLine func(line string) (text string, done bool), provider string) (string, error, bool) {
	resp, err := httpclient.Streaming().Do(req)
	if err != nil {
		return "", &AIError{
			Provider: provider,
			Kind:     ErrKindNetwork,
			Message:  err.Error(),
		}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := httpclient.ReadResponse(resp)
		return "", parseAPIError(provider, resp.StatusCode, string(raw)), false
	}

	var fullText strings.Builder
	receivedTokens := false
	limitedBody := &io.LimitedReader{R: resp.Body, N: httpclient.DefaultMaxResponseBytes + 1}
	scanner := bufio.NewScanner(limitedBody)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		text, done := parseLine(line)
		if text != "" {
			fullText.WriteString(text)
			receivedTokens = true
			if onToken != nil {
				onToken(text)
			}
		}
		if done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fullText.String(), &AIError{
			Provider: provider,
			Kind:     ErrKindNetwork,
			Message:  fmt.Sprintf("stream interrupted: %s", err.Error()),
		}, receivedTokens
	}
	if limitedBody.N <= 0 {
		return fullText.String(), &AIError{
			Provider: provider,
			Kind:     ErrKindNetwork,
			Message:  fmt.Sprintf("stream response exceeds %d-byte limit", httpclient.DefaultMaxResponseBytes),
		}, receivedTokens
	}

	text := fullText.String()
	if text == "" {
		return "", &AIError{
			Provider: provider,
			Kind:     ErrKindEmptyResponse,
			Message:  "empty streaming response",
		}, false
	}
	return text, nil, receivedTokens
}
