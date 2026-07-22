// Package httpclient provides HTTP clients with connection pooling and retry with exponential backoff.
package httpclient

import (
	"net"
	"net/http"
	"time"
)

var (
	DefaultTimeout   = 30 * time.Second
	StreamingTimeout = 120 * time.Second
)

func New(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

func Default() *http.Client {
	return New(DefaultTimeout)
}

func Streaming() *http.Client {
	return New(StreamingTimeout)
}
