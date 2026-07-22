package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fxdv/patchlog/pkg/ai"
	"github.com/fxdv/patchlog/pkg/confluence"
	"github.com/fxdv/patchlog/pkg/jira"
	"github.com/fxdv/patchlog/pkg/render"
)

func TestJiraFetchIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/2/issue/PROJ-123" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"key": "PROJ-123",
			"fields": map[string]any{
				"summary":   "Add login endpoint",
				"priority":  map[string]any{"name": "High"},
				"status":    map[string]any{"name": "In Progress"},
				"issuetype": map[string]any{"name": "Story"},
				"labels":    []string{"backend", "api"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "test-token",
	})

	issue, err := client.FetchIssue(context.Background(), "PROJ-123")
	if err != nil {
		t.Fatal(err)
	}
	if issue.Key != "PROJ-123" {
		t.Errorf("key: got %q", issue.Key)
	}
	if issue.Summary != "Add login endpoint" {
		t.Errorf("summary: got %q", issue.Summary)
	}
	if issue.Priority != "High" {
		t.Errorf("priority: got %q", issue.Priority)
	}
	if issue.Status != "In Progress" {
		t.Errorf("status: got %q", issue.Status)
	}
	if len(issue.Labels) != 2 {
		t.Errorf("labels: got %v", issue.Labels)
	}
}

func TestJiraFetchIssueCached(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary":   "Test",
				"priority":  map[string]any{"name": "Medium"},
				"status":    map[string]any{"name": "Open"},
				"issuetype": map[string]any{"name": "Task"},
				"labels":    []string{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "tok",
	})

	_, _ = client.FetchIssue(context.Background(), "PROJ-1")
	_, _ = client.FetchIssue(context.Background(), "PROJ-1")

	if calls != 1 {
		t.Errorf("expected 1 API call (cached), got %d", calls)
	}
}

func TestJiraAuth(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		resp := map[string]any{
			"key": "PROJ-1",
			"fields": map[string]any{
				"summary": "T", "priority": map[string]any{"name": "M"},
				"status": map[string]any{"name": "O"}, "issuetype": map[string]any{"name": "T"},
				"labels": []string{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "secret-token",
	})
	_, _ = client.FetchIssue(context.Background(), "PROJ-1")

	if authHeader == "" || authHeader[:6] != "Basic " {
		t.Errorf("expected Basic auth, got %q", authHeader)
	}
}

func TestJiraEnrichKeysParallel(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		key := r.URL.Path[len("/rest/api/2/issue/"):]
		resp := map[string]any{
			"key": key,
			"fields": map[string]any{
				"summary": key + " summary", "priority": map[string]any{"name": "Medium"},
				"status": map[string]any{"name": "Open"}, "issuetype": map[string]any{"name": "Task"},
				"labels": []string{},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "tok",
	})

	keys := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	result := client.EnrichKeys(context.Background(), keys)

	if len(result) != 3 {
		t.Errorf("expected 3 enriched keys, got %d", len(result))
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 API calls, got %d", calls.Load())
	}
}

func TestJiraFetchEnhancedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"key": "PROJ-100",
			"fields": map[string]any{
				"summary":   "Implement OAuth2 login",
				"priority":  map[string]any{"name": "Critical"},
				"status":    map[string]any{"name": "In Review"},
				"issuetype": map[string]any{"name": "Story"},
				"labels":    []string{"security", "auth"},
				"parent":    map[string]any{"key": "EPIC-42"},
				"fixVersions": []map[string]any{
					{"name": "v2.0.0"},
				},
				"components": []map[string]any{
					{"name": "Authentication"},
					{"name": "API"},
				},
				"description": "Add OAuth2 support for Google and GitHub providers",
				"assignee":    map[string]any{"displayName": "Alice Chen"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := jira.NewClient(jira.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "tok",
	})

	issue, err := client.FetchIssue(context.Background(), "PROJ-100")
	if err != nil {
		t.Fatal(err)
	}
	if issue.EpicKey != "EPIC-42" {
		t.Errorf("epic key: got %q, want EPIC-42", issue.EpicKey)
	}
	if len(issue.FixVersions) != 1 || issue.FixVersions[0] != "v2.0.0" {
		t.Errorf("fix versions: got %v", issue.FixVersions)
	}
	if len(issue.Components) != 2 {
		t.Errorf("components: got %d, want 2", len(issue.Components))
	}
	if issue.Components[0] != "Authentication" {
		t.Errorf("component[0]: got %q", issue.Components[0])
	}
	if issue.Description == "" {
		t.Error("description should not be empty")
	}
	if issue.Assignee != "Alice Chen" {
		t.Errorf("assignee: got %q", issue.Assignee)
	}
}

func TestOllamaGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req["stream"] == true {
			flusher, _ := w.(http.Flusher)
			for _, chunk := range []string{"Hello", " world"} {
				resp := map[string]any{
					"message": map[string]string{"content": chunk},
					"done":    false,
				}
				data, _ := json.Marshal(resp)
				w.Write(data)
				w.Write([]byte("\n"))
				flusher.Flush()
			}
			done := map[string]any{"message": map[string]string{"content": ""}, "done": true}
			data, _ := json.Marshal(done)
			w.Write(data)
			w.Write([]byte("\n"))
			flusher.Flush()
		} else {
			resp := map[string]any{
				"message": map[string]string{"content": "Hello from Ollama"},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client, err := ai.NewClient(ai.Config{Provider: "ollama", BaseURL: server.URL, Model: "test"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("non-streaming", func(t *testing.T) {
		text, err := client.Generate(context.Background(), "test prompt")
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hello from Ollama" {
			t.Errorf("got %q", text)
		}
	})

	t.Run("streaming", func(t *testing.T) {
		var tokens []string
		text, err := client.StreamGenerate(context.Background(), "test prompt", func(token string) {
			tokens = append(tokens, token)
		})
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hello world" {
			t.Errorf("got %q, want %q", text, "Hello world")
		}
		if len(tokens) != 2 {
			t.Errorf("expected 2 token callbacks, got %d", len(tokens))
		}
	})
}

func TestOpenAIGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(401)
			return
		}

		var req map[string]any
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req["stream"] == true {
			flusher, _ := w.(http.Flusher)
			chunks := []string{
				`data: {"choices":[{"delta":{"content":"Hi "}}]}`,
				`data: {"choices":[{"delta":{"content":"there"}}]}`,
				`data: [DONE]`,
			}
			for _, c := range chunks {
				w.Write([]byte(c + "\n"))
				flusher.Flush()
			}
		} else {
			resp := map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": "OpenAI response"}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client, err := ai.NewClient(ai.Config{Provider: "openai", BaseURL: server.URL, Model: "test", APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("non-streaming", func(t *testing.T) {
		text, err := client.Generate(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}
		if text != "OpenAI response" {
			t.Errorf("got %q", text)
		}
	})

	t.Run("streaming", func(t *testing.T) {
		var tokens []string
		text, err := client.StreamGenerate(context.Background(), "test", func(tok string) {
			tokens = append(tokens, tok)
		})
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hi there" {
			t.Errorf("got %q, want %q", text, "Hi there")
		}
	})
}

func TestAnthropicGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			w.WriteHeader(401)
			return
		}

		var req map[string]any
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req["stream"] == true {
			flusher, _ := w.(http.Flusher)
			events := []string{
				"event: content_block_start\n",
				"data: {\"type\":\"content_block_start\"}\n\n",
				"event: content_block_delta\n",
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
				"event: content_block_delta\n",
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"!\"}}\n\n",
				"event: message_stop\n",
				"data: {\"type\":\"message_stop\"}\n\n",
			}
			for _, e := range events {
				w.Write([]byte(e))
				flusher.Flush()
			}
		} else {
			maxTokens, _ := req["max_tokens"].(float64)
			if maxTokens != 4096 {
				t.Errorf("expected max_tokens 4096, got %v", maxTokens)
			}
			resp := map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "Anthropic response"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client, err := ai.NewClient(ai.Config{Provider: "anthropic", BaseURL: server.URL, Model: "test", APIKey: "test-key", MaxTokens: 4096})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("non-streaming", func(t *testing.T) {
		text, err := client.Generate(context.Background(), "test")
		if err != nil {
			t.Fatal(err)
		}
		if text != "Anthropic response" {
			t.Errorf("got %q", text)
		}
	})

	t.Run("streaming", func(t *testing.T) {
		var tokens []string
		text, err := client.StreamGenerate(context.Background(), "test", func(tok string) {
			tokens = append(tokens, tok)
		})
		if err != nil {
			t.Fatal(err)
		}
		if text != "Hello!" {
			t.Errorf("got %q, want %q", text, "Hello!")
		}
	})
}

func TestConfluenceCreatePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/rest/api/content":
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
		case r.Method == "POST" && r.URL.Path == "/rest/api/content":
			var req map[string]any
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)

			if req["type"] != "page" {
				t.Errorf("expected type page, got %v", req["type"])
			}
			title, _ := req["title"].(string)
			if title == "" {
				t.Error("title should not be empty")
			}

			resp := map[string]any{
				"id":     "99999",
				"title":  title,
				"_links": map[string]string{"webui": "/spaces/ENG/pages/99999"},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := confluence.NewClient(confluence.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	report := render.Report{Version: "1.0.0", Sections: []render.Section{
		{Heading: "Features", Items: []render.Item{{Description: "test"}}},
	}}
	title := fmt.Sprintf("Release Notes — 1.0.0 (2024-01-15)")
	body := confluence.RenderStorageFormat(report)

	page, updated, err := client.PublishOrUpdate(context.Background(), title, body)
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Error("new page should not be marked as updated")
	}
	if page.ID != "99999" {
		t.Errorf("page ID: got %q", page.ID)
	}
}

func TestConfluenceUpdateExistingPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/rest/api/content":
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "88888", "title": "Existing", "type": "page",
						"_links": map[string]string{"webui": "/spaces/ENG/pages/88888"}},
				},
			})
		case r.Method == "GET" && r.URL.Path == "/rest/api/content/88888":
			json.NewEncoder(w).Encode(map[string]any{
				"version": map[string]any{"number": 3},
			})
		case r.Method == "PUT" && r.URL.Path == "/rest/api/content/88888":
			var req map[string]any
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &req)

			version, _ := req["version"].(map[string]any)
			if version["number"].(float64) != 4 {
				t.Errorf("expected version 4, got %v", version["number"])
			}

			resp := map[string]any{
				"id":     "88888",
				"title":  req["title"],
				"_links": map[string]string{"webui": "/spaces/ENG/pages/88888"},
			}
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	client := confluence.NewClient(confluence.Config{
		BaseURL:  server.URL,
		Email:    "dev@test.com",
		APIToken: "tok",
		SpaceKey: "ENG",
	})

	report := render.Report{Version: "1.0.0"}
	title := "Release Notes — 1.0.0 (2024-01-15)"
	body := confluence.RenderStorageFormat(report)

	page, updated, err := client.PublishOrUpdate(context.Background(), title, body)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Error("existing page should be marked as updated")
	}
	if page.ID != "88888" {
		t.Errorf("page ID: got %q", page.ID)
	}
}
