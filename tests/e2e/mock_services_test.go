package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir string, jiraURL, confluenceURL, providerURL, providerType string) {
	t.Helper()
	confSpace := ""
	if confluenceURL != "" || jiraURL != "" {
		confSpace = "ENG"
	}
	cfg := fmt.Sprintf(`ai:
  provider: ollama
  base_url: http://localhost:11111
author:
  show: true
sections:
  feat: Features
  fix: Bug Fixes
  chore: Chores
jira:
  base_url: %s
  email: dev@test.com
  api_token: test-jira-token
  project_key: PROJ
confluence:
  base_url: %s
  email: dev@test.com
  api_token: test-conf-token
  space_key: %s
provider:
  type: %s
  token: test-provider-token
  repo: owner/repo
  base_url: %s
`, jiraURL, confluenceURL, confSpace, providerType, providerURL)
	cfgPath := filepath.Join(dir, "patchlog.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func commitTestConfig(t *testing.T, repo string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "patchlog.yaml"},
		{"commit", "-m", "chore: configure release test"},
		{"push", "origin", "HEAD"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

func newMockJiraServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(401)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/rest/api/2/issue/")
		resp := map[string]any{
			"key": key,
			"fields": map[string]any{
				"summary":   key + " summary from Jira",
				"priority":  map[string]any{"name": "High"},
				"status":    map[string]any{"name": "In Progress"},
				"issuetype": map[string]any{"name": "Story"},
				"labels":    []string{"backend"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
}

func newMockConfluenceServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/rest/api/content":
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/rest/api/content/search"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
		case r.Method == "POST" && r.URL.Path == "/rest/api/content":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			resp := map[string]any{
				"id":     "99999",
				"title":  req["title"],
				"_links": map[string]string{"webui": "/spaces/ENG/pages/99999"},
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/label"):
			json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/restriction"):
			json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == "GET" && strings.Contains(r.URL.RawQuery, "expand=version"):
			json.NewEncoder(w).Encode(map[string]any{"version": map[string]int{"number": 1}})
		default:
			w.WriteHeader(404)
		}
	}))
}

func newMockGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-provider-token" {
			w.WriteHeader(401)
			return
		}
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/releases") {
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			resp := map[string]any{
				"id":       1,
				"html_url": fmt.Sprintf("https://github.com/owner/repo/releases/tag/%s", req["tag_name"]),
				"tag_name": req["tag_name"],
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(404)
	}))
}

func runPatchlog(t *testing.T, bin, repo string, extraArgs ...string) string {
	t.Helper()
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	args := make([]string, 0, len(extraArgs)+9)
	if len(extraArgs) > 0 {
		switch extraArgs[0] {
		case "release", "ai", "confluence", "metrics", "labs":
			args = append(args, extraArgs[0])
			extraArgs = extraArgs[1:]
		}
	}
	args = append(args, "--repo", repo, "--from", "v0.1.0", "--config", cfgPath)
	args = append(args, extraArgs...)
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("patchlog failed: %v\n%s", err, out)
	}
	return string(out)
}

func runPatchlogAllowFail(t *testing.T, bin, repo string, extraArgs ...string) string {
	t.Helper()
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	args := []string{"--repo", repo, "--from", "v0.1.0", "--config", cfgPath}
	args = append(args, extraArgs...)
	cmd := exec.Command(bin, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestE2EJiraEnrichment(t *testing.T) {
	jiraServer := newMockJiraServer(t)
	defer jiraServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add user endpoint PROJ-123",
		"fix: resolve login bug PROJ-456",
	})

	writeConfig(t, repo, jiraServer.URL, "", "", "")
	s := runPatchlog(t, bin, repo, "--format", "json")

	if !strings.Contains(s, "PROJ-123") {
		t.Error("output should contain Jira key PROJ-123")
	}
	if !strings.Contains(s, "PROJ-456") {
		t.Error("output should contain Jira key PROJ-456")
	}
	if !strings.Contains(s, "PROJ-123 summary from Jira") {
		t.Errorf("output should contain Jira summary from mock, got:\n%s", s)
	}
	if !strings.Contains(s, "PROJ-456 summary from Jira") {
		t.Error("output should contain Jira summary for PROJ-456 from mock")
	}
}

func TestE2EJiraProjectFilter(t *testing.T) {
	jiraServer := newMockJiraServer(t)
	defer jiraServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add feature PROJ-123 and ENG-789",
	})

	writeConfig(t, repo, jiraServer.URL, "", "", "")
	s := runPatchlog(t, bin, repo, "--format", "json")

	if !strings.Contains(s, "PROJ-123") {
		t.Error("output should contain PROJ-123 (matches project key)")
	}
}

func TestE2EConfluencePublish(t *testing.T) {
	confluenceServer := newMockConfluenceServer(t)
	defer confluenceServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add feature for Confluence",
	})

	writeConfig(t, repo, "", confluenceServer.URL, "", "")
	s := runPatchlog(t, bin, repo, "confluence")

	if !strings.Contains(s, "Created Confluence page") {
		t.Errorf("output should indicate Confluence page creation, got:\n%s", s)
	}
	if !strings.Contains(s, "/spaces/ENG/pages/99999") {
		t.Error("output should contain Confluence page URL from mock")
	}
}

func TestE2EGitHubPublish(t *testing.T) {
	githubServer := newMockGitHubServer(t)
	defer githubServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add feature for GitHub release",
	})

	writeConfig(t, repo, "", "", githubServer.URL, "github")
	commitTestConfig(t, repo)
	s := runApprovedCommand(t, bin,
		"release", "direct",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", filepath.Join(repo, "patchlog.yaml"),
		"--bump", "patch",
		"--tag",
		"--push",
		"--publish",
	)

	if !strings.Contains(s, "Publishing release draft") {
		t.Errorf("output should indicate publishing, got:\n%s", s)
	}
	if !strings.Contains(s, "github.com/owner/repo/releases") {
		t.Error("output should contain release URL from mock GitHub")
	}
}

func TestE2EFocusedProviderAndConfluenceWorkflows(t *testing.T) {
	jiraServer := newMockJiraServer(t)
	defer jiraServer.Close()

	confluenceServer := newMockConfluenceServer(t)
	defer confluenceServer.Close()

	githubServer := newMockGitHubServer(t)
	defer githubServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat(api): add user endpoint PROJ-100",
		"fix: resolve null pointer PROJ-200",
		"feat(ui): add dashboard",
	})

	writeConfig(t, repo, jiraServer.URL, confluenceServer.URL, githubServer.URL, "github")
	commitTestConfig(t, repo)

	cfgPath := filepath.Join(repo, "patchlog.yaml")
	providerOutput := runApprovedCommand(t, bin,
		"release", "direct",
		"--repo", repo,
		"--from", "v0.1.0",
		"--config", cfgPath,
		"--bump", "auto",
		"--tag",
		"--push",
		"--publish",
	)
	confluenceOutput := runPatchlog(t, bin, repo, "confluence")
	s := providerOutput + confluenceOutput

	if !strings.Contains(s, "PROJ-100 summary from Jira") {
		t.Error("should enrich PROJ-100 from mock Jira")
	}
	if !strings.Contains(s, "PROJ-200 summary from Jira") {
		t.Error("should enrich PROJ-200 from mock Jira")
	}
	if !strings.Contains(s, "Confluence page") {
		t.Error("should publish to mock Confluence")
	}
	if !strings.Contains(s, "github.com/owner/repo/releases") {
		t.Error("should publish to mock GitHub")
	}

	versionData, err := os.ReadFile(filepath.Join(repo, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	v := strings.TrimSpace(string(versionData))
	if v != "0.2.0" {
		t.Errorf("expected auto-bump to 0.2.0 (minor for feat), got %s", v)
	}
}

func TestE2EJiraAuthRejected(t *testing.T) {
	jiraServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer jiraServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: add feature PROJ-999",
	})

	cfg := fmt.Sprintf(`ai:
  provider: ollama
  base_url: http://localhost:11111
jira:
  base_url: %s
  email: wrong@test.com
  api_token: bad-token
  project_key: PROJ
`, jiraServer.URL)
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	os.WriteFile(cfgPath, []byte(cfg), 0644)

	s := runPatchlogAllowFail(t, bin, repo, "--quiet")

	if strings.Contains(s, "PROJ-999 summary from Jira") {
		t.Error("should not enrich when Jira auth fails")
	}
	if !strings.Contains(s, "add feature") {
		t.Error("should still render the commit even without Jira enrichment")
	}
}

func TestE2EConfluenceUpdateExisting(t *testing.T) {
	var gotVersion any
	confluenceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(401)
			return
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/rest/api/content":
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "88888", "title": "Existing", "type": "page",
						"_links": map[string]string{"webui": "/spaces/ENG/pages/88888"}},
				},
			})
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/rest/api/content/search"):
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
		case r.Method == "GET" && r.URL.Path == "/rest/api/content/88888":
			json.NewEncoder(w).Encode(map[string]any{
				"version": map[string]any{"number": 3},
			})
		case r.Method == "PUT" && r.URL.Path == "/rest/api/content/88888":
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			version, _ := req["version"].(map[string]any)
			gotVersion = version["number"]
			resp := map[string]any{
				"id":     "88888",
				"title":  req["title"],
				"_links": map[string]string{"webui": "/spaces/ENG/pages/88888"},
			}
			json.NewEncoder(w).Encode(resp)
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/label"):
			json.NewEncoder(w).Encode(map[string]any{})
		case r.Method == "PUT" && strings.HasSuffix(r.URL.Path, "/restriction"):
			json.NewEncoder(w).Encode(map[string]any{})
		default:
			w.WriteHeader(404)
		}
	}))
	defer confluenceServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: update to existing Confluence page",
	})

	writeConfig(t, repo, "", confluenceServer.URL, "", "")
	s := runPatchlog(t, bin, repo, "confluence")

	if gotVersion != nil && gotVersion.(float64) != 4 {
		t.Errorf("expected version 4, got %v", gotVersion)
	}
	if !strings.Contains(s, "Updated Confluence page") {
		t.Errorf("should indicate update (not create), got:\n%s", s)
	}
}

func TestE2EQuietSuppressesBanner(t *testing.T) {
	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: quiet mode test",
	})

	cfgPath := filepath.Join(repo, "patchlog.yaml")
	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--config", cfgPath, "--quiet")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if strings.Contains(s, "patchlog") && strings.Contains(s, "auto-generate") {
		t.Error("quiet mode should suppress banner")
	}
	if strings.Contains(s, "Release Summary") {
		t.Error("quiet mode should suppress summary")
	}
	if !strings.Contains(s, "quiet mode test") {
		t.Error("quiet mode should still output release notes")
	}
}

func TestE2EEnvVarExpansion(t *testing.T) {
	jiraServer := newMockJiraServer(t)
	defer jiraServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: env var test PROJ-555",
	})

	cfg := fmt.Sprintf(`ai:
  provider: ollama
  base_url: http://localhost:11111
jira:
  base_url: %s
  email: dev@test.com
  api_token: ${PATCHLOG_TEST_JIRA_TOKEN}
  project_key: PROJ
`, jiraServer.URL)
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	os.WriteFile(cfgPath, []byte(cfg), 0644)

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--config", cfgPath, "--format", "json")
	cmd.Env = append(os.Environ(), "PATCHLOG_TEST_JIRA_TOKEN=test-jira-token")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "PROJ-555 summary from Jira") {
		t.Errorf("${VAR} expansion should work, got:\n%s", s)
	}
}

func TestE2EEnvVarDefaultExpansion(t *testing.T) {
	jiraServer := newMockJiraServer(t)
	defer jiraServer.Close()

	bin := buildBinary(t)
	repo := createTestRepo(t, []string{
		"feat: env var default test PROJ-666",
	})

	cfg := fmt.Sprintf(`ai:
  provider: ollama
  base_url: http://localhost:11111
jira:
  base_url: %s
  email: dev@test.com
  api_token: ${MISSING_JIRA_TOKEN:-test-jira-token}
  project_key: PROJ
`, jiraServer.URL)
	cfgPath := filepath.Join(repo, "patchlog.yaml")
	os.WriteFile(cfgPath, []byte(cfg), 0644)

	cmd := exec.Command(bin, "--repo", repo, "--from", "v0.1.0", "--config", cfgPath, "--format", "json")
	cmd.Env = append(os.Environ(), "MISSING_JIRA_TOKEN=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}

	s := string(out)
	if !strings.Contains(s, "PROJ-666 summary from Jira") {
		t.Errorf("${VAR:-default} expansion should use default when var is empty, got:\n%s", s)
	}
}
