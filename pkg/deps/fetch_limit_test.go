package deps

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

type registryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f registryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestRegistryFetchersRejectOversizedResponses(t *testing.T) {
	client := &http.Client{Transport: registryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", int(httpclient.DefaultMaxResponseBytes)+1))),
			Request:    req,
		}, nil
	})}
	tests := []struct {
		name  string
		fetch func(*Change)
	}{
		{name: "npm", fetch: func(c *Change) {
			fetchNPM(context.Background(), client, c, FetchOptions{NPMRegistry: "https://registry.example"})
		}},
		{name: "crates", fetch: func(c *Change) {
			fetchCrates(context.Background(), client, c, FetchOptions{CratesRegistry: "https://registry.example"})
		}},
		{name: "pypi", fetch: func(c *Change) {
			fetchPyPI(context.Background(), client, c, FetchOptions{PyPIRegistry: "https://registry.example"})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			change := &Change{Name: "dependency", NewVersion: "1.0.0"}
			tc.fetch(change)
			if change.Changelog != "" {
				t.Fatalf("oversized response populated changelog: %q", change.Changelog)
			}
		})
	}
}
