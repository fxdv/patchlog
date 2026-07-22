// Package httpclient provides HTTP clients with connection pooling and retry with exponential backoff.
package httpclient

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

var (
	DefaultTimeout   = 30 * time.Second
	StreamingTimeout = 120 * time.Second
)

func New(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: sameOriginRedirect,
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

// sameOriginRedirect prevents credentials in custom headers from crossing an
// origin boundary. Go protects a few standard headers, but integrations also
// use headers such as PRIVATE-TOKEN and x-api-key.
func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if !sameOrigin(via[0].URL, req.URL) {
		return fmt.Errorf("refusing cross-origin redirect from %s to %s", via[0].URL.Redacted(), req.URL.Redacted())
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Scheme == b.Scheme && a.Host == b.Host
}

func Default() *http.Client {
	return New(DefaultTimeout)
}

func Streaming() *http.Client {
	return New(StreamingTimeout)
}
