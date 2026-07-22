package gitwiki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type wikiMock struct {
	t      *testing.T
	server *httptest.Server
	pages  map[string]*wikiPage
	calls  atomic.Int32
}

type wikiPage struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
	WebURL  string `json:"web_url"`
}

func newWikiMock(t *testing.T) *wikiMock {
	t.Helper()
	m := &wikiMock{
		t:     t,
		pages: make(map[string]*wikiPage),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		m.calls.Add(1)
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			w.WriteHeader(401)
			return
		}
		path := r.URL.Path
		if !strings.Contains(path, "/wikis/") {
			if r.Method == "POST" && strings.HasSuffix(path, "/wikis") {
				m.handleCreate(w, r)
				return
			}
			w.WriteHeader(404)
			return
		}
		switch r.Method {
		case "GET":
			m.handleGet(w, r)
		case "PUT":
			m.handleUpdate(w, r)
		default:
			w.WriteHeader(405)
		}
	})
	m.server = httptest.NewServer(mux)
	return m
}

func (m *wikiMock) Close()      { m.server.Close() }
func (m *wikiMock) URL() string { return m.server.URL }

func (m *wikiMock) handleGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/wikis/")
	if len(parts) < 2 {
		w.WriteHeader(404)
		return
	}
	slug := parts[1]
	if p, ok := m.pages[slug]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}
	w.WriteHeader(404)
}

func (m *wikiMock) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	slug := strings.ToLower(strings.ReplaceAll(req.Title, " ", "-"))
	p := &wikiPage{
		Slug:    slug,
		Title:   req.Title,
		Content: req.Content,
		WebURL:  fmt.Sprintf("%s/mygroup/myrepo/-/wikis/%s", m.server.URL, slug),
	}
	m.pages[slug] = p

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(p)
}

func (m *wikiMock) handleUpdate(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/wikis/")
	if len(parts) < 2 {
		w.WriteHeader(404)
		return
	}
	slug := parts[1]

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if p, ok := m.pages[slug]; ok {
		p.Title = req.Title
		p.Content = req.Content
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}
	w.WriteHeader(404)
}

func TestGetPageExists(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	mock.pages["changelog"] = &wikiPage{
		Slug:    "changelog",
		Title:   "Changelog",
		Content: "# Changelog\n\nold content",
		WebURL:  mock.URL() + "/mygroup/myrepo/-/wikis/changelog",
	}

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, err := client.GetPage(context.Background(), "changelog")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page == nil {
		t.Fatal("expected page to exist")
	}
	if page.Content != "# Changelog\n\nold content" {
		t.Errorf("content: got %q", page.Content)
	}
}

func TestGetPageMissing(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, err := client.GetPage(context.Background(), "changelog")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page != nil {
		t.Error("expected nil for missing page")
	}
}

func TestCreatePage(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, err := client.CreatePage(context.Background(), "Changelog", "# Changelog\n\nnew content")
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if page.Slug != "changelog" {
		t.Errorf("slug: got %q", page.Slug)
	}
	if page.Content != "# Changelog\n\nnew content" {
		t.Errorf("content: got %q", page.Content)
	}
}

func TestUpdatePage(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	mock.pages["changelog"] = &wikiPage{
		Slug:    "changelog",
		Title:   "Changelog",
		Content: "old",
		WebURL:  mock.URL() + "/mygroup/myrepo/-/wikis/changelog",
	}

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, err := client.UpdatePage(context.Background(), "changelog", "Changelog", "updated content")
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if page.Content != "updated content" {
		t.Errorf("content: got %q", page.Content)
	}
}

func TestAccumulatePageCreate(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, updated, err := client.AccumulatePage(context.Background(), "Changelog", "## [1.0.0]\n\nnew release")
	if err != nil {
		t.Fatalf("AccumulatePage: %v", err)
	}
	if updated {
		t.Error("expected created, not updated")
	}
	if page == nil {
		t.Fatal("expected page")
	}
}

func TestAccumulatePageUpdate(t *testing.T) {
	mock := newWikiMock(t)
	defer mock.Close()

	mock.pages["changelog"] = &wikiPage{
		Slug:    "changelog",
		Title:   "Changelog",
		Content: "# Changelog\n\nAll notable changes.\n\n---\n\n## [1.0.0]\n\nold content",
		WebURL:  mock.URL() + "/mygroup/myrepo/-/wikis/changelog",
	}

	client := NewClient(Config{
		Token:   "test-token",
		Repo:    "mygroup/myrepo",
		BaseURL: mock.URL(),
		Slug:    "changelog",
	})

	page, updated, err := client.AccumulatePage(context.Background(), "Changelog", "## [2.0.0]\n\nnew release")
	if err != nil {
		t.Fatalf("AccumulatePage: %v", err)
	}
	if !updated {
		t.Error("expected updated=true")
	}
	if page == nil {
		t.Fatal("expected page")
	}
	if !strings.Contains(page.Content, "## [2.0.0]") {
		t.Error("should contain new release")
	}
	if !strings.Contains(page.Content, "## [1.0.0]") {
		t.Error("should preserve old release")
	}
	if !strings.Contains(page.Content, "---") {
		t.Error("should contain separator")
	}
}

func TestConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"fully configured", Config{Token: "tok", Repo: "group/repo"}, true},
		{"missing token", Config{Repo: "group/repo"}, false},
		{"missing repo", Config{Token: "tok"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.cfg)
			if c.Configured() != tt.want {
				t.Errorf("Configured() = %v, want %v", c.Configured(), tt.want)
			}
		})
	}
}

func TestDefaultBaseURL(t *testing.T) {
	c := NewClient(Config{Token: "tok", Repo: "group/repo"})
	if c.baseURL != "https://gitlab.com/api/v4" {
		t.Errorf("default base URL: got %q", c.baseURL)
	}
}

func TestDefaultSlug(t *testing.T) {
	c := NewClient(Config{Token: "tok", Repo: "group/repo"})
	if c.slug != "changelog" {
		t.Errorf("default slug: got %q", c.slug)
	}
}

func TestDecodeWikiPageInvalidJSON(t *testing.T) {
	_, err := decodeWikiPage([]byte("{invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeWikiPageValid(t *testing.T) {
	raw := []byte(`{"slug":"test","title":"Test","content":"body","web_url":"http://example.com"}`)
	page, err := decodeWikiPage(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Slug != "test" || page.Title != "Test" || page.Content != "body" {
		t.Errorf("decoded page: %+v", page)
	}
}

func TestEncodeWikiBody(t *testing.T) {
	data, err := encodeWikiBody("Title", "Content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Title != "Title" || body.Content != "Content" {
		t.Errorf("encoded body: %+v", body)
	}
}
