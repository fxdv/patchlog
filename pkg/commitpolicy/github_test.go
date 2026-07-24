package commitpolicy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubVerifierProvesRequiredCheckForExactCommit(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	var requestedCommit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/owner/repo/branches/main/protection/required_status_checks":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contexts": []string{"quality"},
				"checks":   []map[string]any{{"context": "quality", "app_id": 15368}},
			})
		case r.URL.Path == "/repos/owner/repo/rules/branches/main":
			_ = json.NewEncoder(w).Encode([]any{})
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			requestedCommit = strings.Split(r.URL.Path, "/")[5]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 1,
				"check_runs": []map[string]any{{
					"name": "quality", "status": "completed", "conclusion": "success",
					"app": map[string]any{"id": 15368},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"statuses": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	verifier, err := NewGitHub(GitHubConfig{
		Repository: "owner/repo",
		BaseURL:    server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Repository: "owner/repo", Branch: "main", Commit: commit}
	evidence, err := verifier.Verify(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if requestedCommit != commit {
		t.Fatalf("checked commit = %q, want %q", requestedCommit, commit)
	}
	if evidence.Commit != commit || evidence.Provider != "github" ||
		len(evidence.RequiredChecks) != 1 || evidence.RequiredChecks[0].Context != "quality" {
		t.Fatalf("evidence = %#v", evidence)
	}
}

func TestGitHubVerifierRejectsFailedRequiredCheck(t *testing.T) {
	server := githubPolicyServer(t, "failure")
	defer server.Close()
	verifier, err := NewGitHub(GitHubConfig{Repository: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifier.Verify(context.Background(), Request{
		Repository: "owner/repo",
		Branch:     "main",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	})
	if err == nil || !strings.Contains(err.Error(), "not completed successfully") {
		t.Fatalf("failed check error = %v", err)
	}
}

func TestGitHubVerifierRejectsMissingRequiredPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/protection/required_status_checks"):
			http.NotFound(w, r)
		case strings.Contains(r.URL.Path, "/rules/branches/"):
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	verifier, err := NewGitHub(GitHubConfig{Repository: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifier.Verify(context.Background(), Request{
		Repository: "owner/repo",
		Branch:     "main",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	})
	if err == nil || !strings.Contains(err.Error(), "no required status checks") {
		t.Fatalf("missing policy error = %v", err)
	}
}

func TestGitHubVerifierReadsRepositoryRulesets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/protection/required_status_checks"):
			http.NotFound(w, r)
		case strings.Contains(r.URL.Path, "/rules/branches/"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"type": "required_status_checks",
				"parameters": map[string]any{
					"required_status_checks": []map[string]any{{
						"context": "quality", "integration_id": 15368,
					}},
				},
			}})
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 1,
				"check_runs": []map[string]any{{
					"name": "quality", "status": "completed", "conclusion": "neutral",
					"app": map[string]any{"id": 15368},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"statuses": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	verifier, err := NewGitHub(GitHubConfig{Repository: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(context.Background(), Request{
		Repository: "owner/repo",
		Branch:     "main",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubVerifierFailsClosedForRequiredWorkflowRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/protection/required_status_checks"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contexts": []string{"quality"},
				"checks":   []map[string]any{{"context": "quality", "app_id": 15368}},
			})
		case strings.Contains(r.URL.Path, "/rules/branches/"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{"type": "workflows"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	verifier, err := NewGitHub(GitHubConfig{Repository: "owner/repo", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = verifier.Verify(context.Background(), Request{
		Repository: "owner/repo",
		Branch:     "main",
		Commit:     "0123456789abcdef0123456789abcdef01234567",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot yet prove") {
		t.Fatalf("required workflow policy error = %v", err)
	}
}

func TestEvidenceNormalizationIsStableAndRejectsIdentityMismatch(t *testing.T) {
	evidence := Evidence{
		Provider:   " github ",
		Repository: "owner/repo",
		Branch:     "main",
		Commit:     "abc",
		RequiredChecks: []RequiredCheck{
			{Context: "z"},
			{Context: "a", IntegrationID: 2},
		},
	}.Normalize()
	if evidence.SchemaVersion != EvidenceSchemaVersion ||
		evidence.RequiredChecks[0].Context != "a" {
		t.Fatalf("normalized evidence = %#v", evidence)
	}
	err := evidence.Validate(Request{Repository: "owner/repo", Branch: "main", Commit: "different"})
	if err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("identity mismatch error = %v", err)
	}
}

func githubPolicyServer(t *testing.T, conclusion string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/protection/required_status_checks"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"contexts": []string{"quality"},
				"checks":   []map[string]any{{"context": "quality", "app_id": 15368}},
			})
		case strings.Contains(r.URL.Path, "/rules/branches/"):
			_ = json.NewEncoder(w).Encode([]any{})
		case strings.HasSuffix(r.URL.Path, "/check-runs"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 1,
				"check_runs": []map[string]any{{
					"name": "quality", "status": "completed", "conclusion": conclusion,
					"app": map[string]any{"id": 15368},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"statuses": []any{}})
		default:
			http.NotFound(w, r)
		}
	}))
}
