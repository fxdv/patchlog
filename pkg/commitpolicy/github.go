package commitpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

const maxPolicyResponseBytes int64 = 2 << 20

type GitHubConfig struct {
	Token      string
	Repository string
	BaseURL    string
}

type GitHubVerifier struct {
	token      string
	repository string
	baseURL    string
	client     *http.Client
}

func NewGitHub(cfg GitHubConfig) (*GitHubVerifier, error) {
	repository := strings.TrimSpace(cfg.Repository)
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("GitHub commit-policy verifier requires repository as owner/name")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("GitHub commit-policy verifier has invalid base URL %q", baseURL)
	}
	return &GitHubVerifier{
		token:      cfg.Token,
		repository: repository,
		baseURL:    baseURL,
		client:     httpclient.Default(),
	}, nil
}

type githubRequiredStatusChecks struct {
	Contexts []string `json:"contexts"`
	Checks   []struct {
		Context string `json:"context"`
		AppID   int64  `json:"app_id"`
	} `json:"checks"`
}

type githubRule struct {
	Type       string `json:"type"`
	Parameters struct {
		RequiredStatusChecks []struct {
			Context       string `json:"context"`
			IntegrationID int64  `json:"integration_id"`
		} `json:"required_status_checks"`
	} `json:"parameters"`
}

type githubCheckRuns struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		App        struct {
			ID int64 `json:"id"`
		} `json:"app"`
	} `json:"check_runs"`
}

type githubCombinedStatus struct {
	Statuses []struct {
		Context string `json:"context"`
		State   string `json:"state"`
	} `json:"statuses"`
}

func (v *GitHubVerifier) Verify(ctx context.Context, req Request) (Evidence, error) {
	if v == nil {
		return Evidence{}, fmt.Errorf("GitHub commit-policy verifier is nil")
	}
	if strings.TrimSpace(req.Repository) == "" {
		req.Repository = v.repository
	}
	if req.Repository != v.repository {
		return Evidence{}, fmt.Errorf("commit-policy repository %s does not match configured GitHub repository %s", req.Repository, v.repository)
	}
	if strings.TrimSpace(req.Branch) == "" || strings.TrimSpace(req.Commit) == "" {
		return Evidence{}, fmt.Errorf("commit-policy verification requires an exact branch and commit")
	}

	required, err := v.requiredChecks(ctx, req.Branch)
	if err != nil {
		return Evidence{}, err
	}
	if len(required) == 0 {
		return Evidence{}, fmt.Errorf("GitHub protected branch %s has no required status checks", req.Branch)
	}

	runs, err := v.checkRuns(ctx, req.Commit)
	if err != nil {
		return Evidence{}, err
	}
	statuses, err := v.commitStatuses(ctx, req.Commit)
	if err != nil {
		return Evidence{}, err
	}
	for _, check := range required {
		if err := verifyRequiredGitHubCheck(check, runs, statuses); err != nil {
			return Evidence{}, fmt.Errorf("GitHub required check %q is not satisfied for %s: %w", check.Context, req.Commit, err)
		}
	}

	evidence := Evidence{
		SchemaVersion:  EvidenceSchemaVersion,
		Provider:       "github",
		Repository:     req.Repository,
		Branch:         req.Branch,
		Commit:         req.Commit,
		RequiredChecks: required,
	}.Normalize()
	if err := evidence.Validate(req); err != nil {
		return Evidence{}, err
	}
	return evidence, nil
}

func (v *GitHubVerifier) requiredChecks(ctx context.Context, branch string) ([]RequiredCheck, error) {
	checks := make(map[string]RequiredCheck)
	add := func(context string, integrationID int64) {
		context = strings.TrimSpace(context)
		if context == "" {
			return
		}
		key := fmt.Sprintf("%s\x00%d", context, integrationID)
		checks[key] = RequiredCheck{Context: context, IntegrationID: integrationID}
	}

	classicPath := fmt.Sprintf(
		"/repos/%s/branches/%s/protection/required_status_checks",
		v.repository,
		url.PathEscape(branch),
	)
	status, raw, err := v.get(ctx, classicPath)
	if err != nil {
		return nil, err
	}
	switch status {
	case http.StatusOK:
		var policy githubRequiredStatusChecks
		if err := json.Unmarshal(raw, &policy); err != nil {
			return nil, fmt.Errorf("decode GitHub branch status-check policy: %w", err)
		}
		for _, check := range policy.Checks {
			add(check.Context, check.AppID)
		}
		// Older GitHub and GitHub Enterprise responses may expose contexts
		// without app-bound check records.
		for _, context := range policy.Contexts {
			if _, appBound := checks[fmt.Sprintf("%s\x00%d", context, int64(0))]; !appBound {
				hasContext := false
				for _, check := range checks {
					if check.Context == context {
						hasContext = true
						break
					}
				}
				if !hasContext {
					add(context, 0)
				}
			}
		}
	case http.StatusNotFound:
	default:
		return nil, githubPolicyStatusError(status, raw)
	}

	rulesPath := fmt.Sprintf("/repos/%s/rules/branches/%s", v.repository, url.PathEscape(branch))
	status, raw, err = v.get(ctx, rulesPath)
	if err != nil {
		return nil, err
	}
	switch status {
	case http.StatusOK:
		var rules []githubRule
		if err := json.Unmarshal(raw, &rules); err != nil {
			return nil, fmt.Errorf("decode GitHub repository rules: %w", err)
		}
		for _, rule := range rules {
			if rule.Type == "workflows" {
				return nil, fmt.Errorf("GitHub branch policy contains required workflows that this verifier cannot yet prove by workflow identity")
			}
			if rule.Type != "required_status_checks" {
				continue
			}
			for _, check := range rule.Parameters.RequiredStatusChecks {
				add(check.Context, check.IntegrationID)
			}
		}
	case http.StatusNotFound:
	default:
		return nil, githubPolicyStatusError(status, raw)
	}

	result := make([]RequiredCheck, 0, len(checks))
	for _, check := range checks {
		result = append(result, check)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Context == result[j].Context {
			return result[i].IntegrationID < result[j].IntegrationID
		}
		return result[i].Context < result[j].Context
	})
	return result, nil
}

func (v *GitHubVerifier) checkRuns(ctx context.Context, commit string) (githubCheckRuns, error) {
	path := fmt.Sprintf("/repos/%s/commits/%s/check-runs?per_page=100&filter=latest", v.repository, url.PathEscape(commit))
	status, raw, err := v.get(ctx, path)
	if err != nil {
		return githubCheckRuns{}, err
	}
	if status != http.StatusOK {
		return githubCheckRuns{}, githubPolicyStatusError(status, raw)
	}
	var runs githubCheckRuns
	if err := json.Unmarshal(raw, &runs); err != nil {
		return githubCheckRuns{}, fmt.Errorf("decode GitHub check runs: %w", err)
	}
	if runs.TotalCount > len(runs.CheckRuns) {
		return githubCheckRuns{}, fmt.Errorf("GitHub returned %d check runs; refusing incomplete policy verification above the 100-run response limit", runs.TotalCount)
	}
	return runs, nil
}

func (v *GitHubVerifier) commitStatuses(ctx context.Context, commit string) (githubCombinedStatus, error) {
	path := fmt.Sprintf("/repos/%s/commits/%s/status?per_page=100", v.repository, url.PathEscape(commit))
	status, raw, err := v.get(ctx, path)
	if err != nil {
		return githubCombinedStatus{}, err
	}
	if status != http.StatusOK {
		return githubCombinedStatus{}, githubPolicyStatusError(status, raw)
	}
	var statuses githubCombinedStatus
	if err := json.Unmarshal(raw, &statuses); err != nil {
		return githubCombinedStatus{}, fmt.Errorf("decode GitHub commit statuses: %w", err)
	}
	if len(statuses.Statuses) == 100 {
		return githubCombinedStatus{}, fmt.Errorf("GitHub returned at least 100 commit statuses; refusing potentially incomplete policy verification")
	}
	return statuses, nil
}

func verifyRequiredGitHubCheck(required RequiredCheck, runs githubCheckRuns, statuses githubCombinedStatus) error {
	runSeen := false
	runPassed := false
	for _, run := range runs.CheckRuns {
		if run.Name != required.Context {
			continue
		}
		if required.IntegrationID > 0 && run.App.ID != required.IntegrationID {
			continue
		}
		runSeen = true
		if run.Status == "completed" && successfulGitHubConclusion(run.Conclusion) {
			runPassed = true
		}
		break
	}

	statusSeen := false
	statusPassed := false
	for _, status := range statuses.Statuses {
		if status.Context != required.Context {
			continue
		}
		statusSeen = true
		statusPassed = status.State == "success"
		break
	}

	if required.IntegrationID > 0 {
		if !runSeen {
			return fmt.Errorf("missing check run from required integration %d", required.IntegrationID)
		}
		if !runPassed {
			return fmt.Errorf("check run is not completed successfully")
		}
		if statusSeen && !statusPassed {
			return fmt.Errorf("same-named commit status is not successful")
		}
		return nil
	}
	if !runSeen && !statusSeen {
		return fmt.Errorf("check result is missing")
	}
	if runSeen && !runPassed {
		return fmt.Errorf("check run is not completed successfully")
	}
	if statusSeen && !statusPassed {
		return fmt.Errorf("commit status is not successful")
	}
	return nil
}

func successfulGitHubConclusion(conclusion string) bool {
	switch conclusion {
	case "success", "neutral", "skipped":
		return true
	default:
		return false
	}
}

func (v *GitHubVerifier) get(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create GitHub policy request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "patchlog")
	if v.token != "" {
		req.Header.Set("Authorization", "Bearer "+v.token)
	}
	resp, err := httpclient.DoWithRetry(v.client, req, httpclient.DefaultRetry)
	if err != nil {
		return 0, nil, fmt.Errorf("GitHub commit-policy request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := httpclient.ReadResponseLimit(resp, maxPolicyResponseBytes)
	if err != nil {
		return 0, nil, fmt.Errorf("read GitHub commit-policy response: %w", err)
	}
	return resp.StatusCode, raw, nil
}

func githubPolicyStatusError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300] + "…"
	}
	if status == http.StatusForbidden {
		return fmt.Errorf("GitHub status %d reading required checks; token needs read access to administration and checks: %s", status, message)
	}
	return fmt.Errorf("GitHub status %d reading required checks: %s", status, message)
}
