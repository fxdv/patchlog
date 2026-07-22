package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxdv/patchlog/pkg/cache"
)

func TestExtractKeys(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"single key", "PROJ-123", []string{"PROJ-123"}},
		{"multiple keys", "PROJ-123 and ENG-456", []string{"PROJ-123", "ENG-456"}},
		{"dedup", "PROJ-123 PROJ-123", []string{"PROJ-123"}},
		{"no keys", "no tickets here", nil},
		{"in sentence", "feat: add login PROJ-789 for users", []string{"PROJ-789"}},
		{"multi-digit", "ABC-12345 and X-1", []string{"ABC-12345"}},
		{"lowercase excluded", "proj-123 not matched", nil},
		{"mixed", "PROJ-123 and proj-456", []string{"PROJ-123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractKeys(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i, k := range got {
				if k != tt.want[i] {
					t.Errorf("key[%d]: got %s, want %s", i, k, tt.want[i])
				}
			}
		})
	}
}

func TestFilterKeys(t *testing.T) {
	client := NewClient(Config{ProjectKey: "PROJ"})
	keys := []string{"PROJ-123", "ENG-456", "PROJ-789"}
	filtered := client.FilterKeys(keys)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "PROJ-123" || filtered[1] != "PROJ-789" {
		t.Errorf("unexpected filtered keys: %v", filtered)
	}
}

func TestFilterKeysNoProjectKey(t *testing.T) {
	client := NewClient(Config{})
	keys := []string{"PROJ-123", "ENG-456"}
	filtered := client.FilterKeys(keys)
	if len(filtered) != 2 {
		t.Errorf("without project key, all should pass: %v", filtered)
	}
}

func TestClientConfigured(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   bool
	}{
		{"fully configured", Config{BaseURL: "https://example.com", APIToken: "tok"}, true},
		{"missing base URL", Config{APIToken: "tok"}, false},
		{"missing token", Config{BaseURL: "https://example.com"}, false},
		{"empty config", Config{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config)
			if client.Configured() != tt.want {
				t.Errorf("Configured() = %v, want %v", client.Configured(), tt.want)
			}
		})
	}
}

func TestClientTrailingSlashStripped(t *testing.T) {
	client := NewClient(Config{BaseURL: "https://example.com/"})
	if client.baseURL != "https://example.com" {
		t.Errorf("trailing slash not stripped: %q", client.baseURL)
	}
}

func TestFilterKeysCaseInsensitive(t *testing.T) {
	client := NewClient(Config{ProjectKey: "proj"})
	keys := []string{"PROJ-123", "ENG-456"}
	filtered := client.FilterKeys(keys)
	if len(filtered) != 1 || filtered[0] != "PROJ-123" {
		t.Errorf("filter should be case-insensitive: %v", filtered)
	}
}

func jiraMockServer(t *testing.T, issues map[string]struct {
	Summary  string
	Priority string
	Status   string
	Type     string
	Labels   []string
}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/2/issue/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/rest/api/2/issue/"):]
		if idx := len(key); idx > 0 {
			key = key[:idx]
		}
		for k := range issues {
			if r.URL.Path == "/rest/api/2/issue/"+k {
				issue := issues[k]
				resp := map[string]any{
					"key": k,
					"fields": map[string]any{
						"summary":   issue.Summary,
						"priority":  map[string]string{"name": issue.Priority},
						"status":    map[string]string{"name": issue.Status},
						"issuetype": map[string]string{"name": issue.Type},
						"labels":    issue.Labels,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
		}
		http.Error(w, `{"errorMessages":["Issue does not exist"]}`, 404)
	})
	return httptest.NewServer(mux)
}

func TestFetchIssueHTTP(t *testing.T) {
	srv := jiraMockServer(t, map[string]struct {
		Summary  string
		Priority string
		Status   string
		Type     string
		Labels   []string
	}{
		"PROJ-100": {"Add login", "High", "In Progress", "Story", []string{"frontend"}},
	})
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	issue, err := client.FetchIssue(context.Background(), "PROJ-100")
	if err != nil {
		t.Fatalf("FetchIssue: %v", err)
	}
	if issue.Key != "PROJ-100" {
		t.Errorf("Key = %q, want PROJ-100", issue.Key)
	}
	if issue.Summary != "Add login" {
		t.Errorf("Summary = %q, want Add login", issue.Summary)
	}
	if issue.Priority != "High" {
		t.Errorf("Priority = %q, want High", issue.Priority)
	}
	if issue.URL != srv.URL+"/browse/PROJ-100" {
		t.Errorf("URL = %q", issue.URL)
	}
}

func TestFetchIssueNotFound(t *testing.T) {
	srv := jiraMockServer(t, nil)
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	_, err := client.FetchIssue(context.Background(), "NOPE-1")
	if err == nil {
		t.Error("expected error for 404 issue")
	}
}

func TestFetchIssueMemoHit(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary":   "test",
				"priority":  map[string]string{"name": "Medium"},
				"status":    map[string]string{"name": "Open"},
				"issuetype": map[string]string{"name": "Bug"},
				"labels":    []string{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})

	_, err := client.FetchIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	_, err = client.FetchIssue(context.Background(), "PROJ-1")
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls.Load())
	}
}

func TestFetchIssueFileCacheHit(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"key": "PROJ-2",
			"fields": map[string]any{
				"summary":   "cached issue",
				"priority":  map[string]string{"name": "Low"},
				"status":    map[string]string{"name": "Done"},
				"issuetype": map[string]string{"name": "Task"},
				"labels":    []string{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client1 := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client1.SetCache(fc)

	_, err := client1.FetchIssue(context.Background(), "PROJ-2")
	if err != nil {
		t.Fatalf("first client fetch: %v", err)
	}

	client2 := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client2.SetCache(fc)

	issue, err := client2.FetchIssue(context.Background(), "PROJ-2")
	if err != nil {
		t.Fatalf("second client fetch (from file cache): %v", err)
	}
	if issue.Summary != "cached issue" {
		t.Errorf("Summary = %q, want cached issue", issue.Summary)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d (file cache should serve second request)", calls.Load())
	}
}

func TestFetchIssueDisabledCache(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"key": "PROJ-3",
			"fields": map[string]any{
				"summary":   "no cache",
				"priority":  map[string]string{"name": "Medium"},
				"status":    map[string]string{"name": "Open"},
				"issuetype": map[string]string{"name": "Bug"},
				"labels":    []string{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir, cache.WithEnabled(false))

	client1 := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client1.SetCache(fc)

	_, err := client1.FetchIssue(context.Background(), "PROJ-3")
	if err != nil {
		t.Fatalf("client1 fetch: %v", err)
	}

	client2 := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client2.SetCache(fc)

	_, err = client2.FetchIssue(context.Background(), "PROJ-3")
	if err != nil {
		t.Fatalf("client2 fetch: %v", err)
	}

	if calls.Load() != 2 {
		t.Errorf("expected 2 HTTP calls with disabled file cache (each client has its own memo), got %d", calls.Load())
	}
}

func TestFetchIssueFileCacheWriteBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"key": "PROJ-4",
			"fields": map[string]any{
				"summary":   "persist me",
				"priority":  map[string]string{"name": "High"},
				"status":    map[string]string{"name": "Open"},
				"issuetype": map[string]string{"name": "Story"},
				"labels":    []string{"backend"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client.SetCache(fc)

	_, err := client.FetchIssue(context.Background(), "PROJ-4")
	if err != nil {
		t.Fatalf("FetchIssue: %v", err)
	}

	var cached Issue
	ok, _ := fc.Get("jira", "PROJ-4", &cached)
	if !ok {
		t.Fatal("expected issue to be persisted in file cache")
	}
	if cached.Key != "PROJ-4" || cached.Summary != "persist me" {
		t.Errorf("cached = %+v, want key=PROJ-4 summary=persist me", cached)
	}
	if len(cached.Labels) != 1 || cached.Labels[0] != "backend" {
		t.Errorf("cached labels = %v, want [backend]", cached.Labels)
	}
}

func TestEnrichKeysWithCache(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		key := r.URL.Path[len("/rest/api/2/issue/"):]
		resp := map[string]any{
			"key": key,
			"fields": map[string]any{
				"summary":   fmt.Sprintf("summary for %s", key),
				"priority":  map[string]string{"name": "Medium"},
				"status":    map[string]string{"name": "Open"},
				"issuetype": map[string]string{"name": "Task"},
				"labels":    []string{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	fc := cache.New(cacheDir)

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client.SetCache(fc)

	issues := client.EnrichKeys(context.Background(), []string{"PROJ-10", "PROJ-20", "PROJ-30"})
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}
	if issues["PROJ-10"].Summary != "summary for PROJ-10" {
		t.Errorf("PROJ-10 summary = %q", issues["PROJ-10"].Summary)
	}

	firstCallCount := calls.Load()

	client2 := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	client2.SetCache(fc)
	issues2 := client2.EnrichKeys(context.Background(), []string{"PROJ-10", "PROJ-20", "PROJ-30"})
	if len(issues2) != 3 {
		t.Fatalf("expected 3 issues from cache, got %d", len(issues2))
	}

	totalCalls := calls.Load()
	if totalCalls != firstCallCount {
		t.Errorf("expected no additional HTTP calls, got %d total (was %d after first batch)", totalCalls, firstCallCount)
	}
}

func TestFetchIssueNoCacheSet(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		resp := map[string]any{
			"key": "PROJ-5",
			"fields": map[string]any{
				"summary":   "no file cache",
				"priority":  map[string]string{"name": "Low"},
				"status":    map[string]string{"name": "Done"},
				"issuetype": map[string]string{"name": "Bug"},
				"labels":    []string{},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})

	_, err := client.FetchIssue(context.Background(), "PROJ-5")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	_, err = client.FetchIssue(context.Background(), "PROJ-5")
	if err != nil {
		t.Fatalf("second fetch (memo): %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call (memo should serve second), got %d", calls.Load())
	}
}

func TestSetCache(t *testing.T) {
	client := NewClient(Config{})
	if client.fileCache != nil {
		t.Error("fileCache should be nil by default")
	}
	fc := cache.New(t.TempDir())
	client.SetCache(fc)
	if client.fileCache == nil {
		t.Error("SetCache should set fileCache")
	}
}

func TestFetchIssueContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewClient(Config{BaseURL: srv.URL, Email: "test@test.com", APIToken: "tok"})
	_, err := client.FetchIssue(ctx, "PROJ-99")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
