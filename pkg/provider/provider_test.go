package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestGitHubCreateRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(401)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/repos/owner/repo/releases" {
			w.WriteHeader(404)
			return
		}
		var req githubReleaseReq
		json.NewDecoder(r.Body).Decode(&req)
		if req.TagName != "1.0.0" {
			t.Errorf("expected tag_name 1.0.0, got %s", req.TagName)
		}
		if !req.Draft {
			t.Error("expected draft release")
		}
		resp := githubReleaseResp{
			ID:      1,
			HTMLURL: "https://github.com/owner/repo/releases/tag/1.0.0",
			TagName: "1.0.0",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "test-token", Repo: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	release, err := prov.CreateRelease(context.Background(), "1.0.0", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "1.0.0" {
		t.Errorf("tag: got %q", release.TagName)
	}
	if release.URL != "https://github.com/owner/repo/releases/tag/1.0.0" {
		t.Errorf("url: got %q", release.URL)
	}
}

func TestGitHubFetchPR(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/pulls/42" {
			w.WriteHeader(404)
			return
		}
		resp := map[string]any{
			"number":   42,
			"title":    "Add feature",
			"html_url": "https://github.com/owner/repo/pull/42",
			"user":     map[string]string{"login": "alice", "html_url": "https://github.com/alice"},
			"labels":   []map[string]string{{"name": "enhancement"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "test-token", Repo: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	meta, err := prov.FetchPR(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if meta.PRNumber != 42 {
		t.Errorf("number: got %d", meta.PRNumber)
	}
	if meta.Author != "alice" {
		t.Errorf("author: got %q", meta.Author)
	}
	if len(meta.Labels) != 1 || meta.Labels[0] != "enhancement" {
		t.Errorf("labels: got %v", meta.Labels)
	}
}

func TestGitLabCreateRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			w.WriteHeader(401)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(404)
			return
		}
		resp := gitlabReleaseResp{
			TagName: "1.0.0",
		}
		resp.Links.Self = "https://gitlab.com/api/v4/projects/1/releases/1.0.0"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitLab, Token: "test-token", Repo: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	release, err := prov.CreateRelease(context.Background(), "1.0.0", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "1.0.0" {
		t.Errorf("tag: got %q", release.TagName)
	}
}

func TestGiteaCreateRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token test-token" {
			w.WriteHeader(401)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(404)
			return
		}
		resp := giteaReleaseResp{
			ID:      1,
			HTMLURL: "https://gitea.example.com/owner/repo/releases/tag/1.0.0",
			TagName: "1.0.0",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: Gitea, Token: "test-token", Repo: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	release, err := prov.CreateRelease(context.Background(), "1.0.0", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "1.0.0" {
		t.Errorf("tag: got %q", release.TagName)
	}
}

func TestNewProviderRequiresRepo(t *testing.T) {
	_, err := New(Config{Type: GitHub, Token: "tok"})
	if err == nil {
		t.Error("expected error for missing repo")
	}
}

func TestNewProviderUnknownType(t *testing.T) {
	_, err := New(Config{Type: "unknown", Repo: "owner/repo"})
	if err == nil {
		t.Error("expected error for unknown provider type")
	}
}

func TestNewGiteaRequiresBaseURL(t *testing.T) {
	_, err := New(Config{Type: Gitea, Token: "tok", Repo: "owner/repo"})
	if err == nil {
		t.Error("expected error for missing gitea base_url")
	}
}

func TestDraftFlagNilDefaultsTrue(t *testing.T) {
	var gotReq githubReleaseReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		resp := githubReleaseResp{ID: 1, HTMLURL: "u", TagName: "1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prov.CreateRelease(context.Background(), "1.0.0", "notes"); err != nil {
		t.Fatal(err)
	}
	if !gotReq.Draft {
		t.Error("nil draft should default to true")
	}
}

func TestDraftFlagFalse(t *testing.T) {
	draft := false
	var gotReq githubReleaseReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		resp := githubReleaseResp{ID: 1, HTMLURL: "u", TagName: "1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prov.CreateRelease(context.Background(), "1.0.0", "notes"); err != nil {
		t.Fatal(err)
	}
	if gotReq.Draft {
		t.Error("draft:false should produce non-draft release")
	}
}

func TestDraftFlagGitea(t *testing.T) {
	draft := false
	var gotReq giteaReleaseReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		resp := giteaReleaseResp{ID: 1, HTMLURL: "u", TagName: "1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: Gitea, Token: "t", Repo: "o/r", BaseURL: server.URL, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prov.CreateRelease(context.Background(), "1.0.0", "notes"); err != nil {
		t.Fatal(err)
	}
	if gotReq.IsDraft {
		t.Error("gitea draft:false should produce non-draft release")
	}
}

func TestGitHubFetchPRRetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(503)
			return
		}
		resp := map[string]any{
			"number":   42,
			"title":    "Add feature",
			"html_url": "https://github.com/o/r/pull/42",
			"user":     map[string]string{"login": "alice", "html_url": "https://github.com/alice"},
			"labels":   []map[string]string{{"name": "enhancement"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := prov.FetchPR(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if meta.PRNumber != 42 {
		t.Errorf("number: got %d", meta.PRNumber)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", calls.Load())
	}
}

func TestGitHubFetchPRNoRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(404)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = prov.FetchPR(context.Background(), 42)
	if err == nil {
		t.Error("expected error for 404")
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 attempt for 404, got %d", calls.Load())
	}
}

func TestGitLabFetchPRRetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 1 {
			w.WriteHeader(500)
			return
		}
		resp := map[string]any{
			"iid":     42,
			"title":   "Add feature",
			"web_url": "https://gitlab.com/o/r/-/merge_requests/42",
			"author":  map[string]string{"username": "alice", "web_url": "https://gitlab.com/alice"},
			"labels":  []string{"enhancement"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitLab, Token: "t", Repo: "o/r", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	meta, err := prov.FetchPR(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if meta.PRNumber != 42 {
		t.Errorf("number: got %d", meta.PRNumber)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 attempts, got %d", calls.Load())
	}
}

func TestFetchPRContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = prov.FetchPR(ctx, 42)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestDraftFlagTrue(t *testing.T) {
	draft := true
	var gotReq githubReleaseReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotReq)
		resp := githubReleaseResp{ID: 1, HTMLURL: "u", TagName: "1.0.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	prov, err := New(Config{Type: GitHub, Token: "t", Repo: "o/r", BaseURL: server.URL, Draft: &draft})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prov.CreateRelease(context.Background(), "1.0.0", "notes"); err != nil {
		t.Fatal(err)
	}
	if !gotReq.Draft {
		t.Error("draft:true should produce draft release")
	}
}

func TestGitLabCreateReleaseNonDraft(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(404)
			return
		}
		var req gitlabReleaseReq
		json.NewDecoder(r.Body).Decode(&req)
		if req.Ref != "HEAD" {
			t.Errorf("expected ref HEAD, got %q", req.Ref)
		}
		resp := gitlabReleaseResp{TagName: "1.0.0"}
		resp.Links.Self = fmt.Sprintf("%s/projects/1/releases/1.0.0", srv.URL)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	prov, err := New(Config{Type: GitLab, Token: "t", Repo: "o/r", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	release, err := prov.CreateRelease(context.Background(), "1.0.0", "notes")
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "1.0.0" {
		t.Errorf("tag: got %q", release.TagName)
	}
}
