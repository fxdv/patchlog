// Package jira provides a Jira Cloud/Server API client for fetching
// issue details concurrently with caching support.
package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/httpclient"
	"github.com/fxdv/patchlog/pkg/internal/pattern"
	"github.com/fxdv/patchlog/pkg/internal/truncate"
)

const cacheNamespace = "jira"

type Config struct {
	BaseURL        string `yaml:"base_url"`
	Email          string `yaml:"email"`
	APIToken       string `yaml:"api_token"`
	ProjectKey     string `yaml:"project_key"`
	MaxConcurrency int    `yaml:"max_concurrency"`
}

// Issue holds enriched Jira ticket data for release notes.
type Issue struct {
	Key         string   `json:"key"`
	Summary     string   `json:"summary"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	Type        string   `json:"type"`
	Labels      []string `json:"labels"`
	URL         string   `json:"url"`
	EpicKey     string   `json:"epic_key"`
	FixVersions []string `json:"fix_versions"`
	Components  []string `json:"components"`
	Description string   `json:"description"`
	Assignee    string   `json:"assignee"`
}

// Client is a Jira API client with caching and concurrent fetching.
type Client struct {
	baseURL        string
	email          string
	apiToken       string
	projectKey     string
	maxConcurrency int
	memo           map[string]*Issue
	mu             sync.RWMutex
	httpClient     *http.Client
	fileCache      *cache.Cache
}

func NewClient(cfg Config) *Client {
	maxConc := cfg.MaxConcurrency
	if maxConc <= 0 {
		maxConc = 5
	}
	return &Client{
		baseURL:        strings.TrimRight(cfg.BaseURL, "/"),
		email:          cfg.Email,
		apiToken:       cfg.APIToken,
		projectKey:     cfg.ProjectKey,
		maxConcurrency: maxConc,
		memo:           make(map[string]*Issue),
		httpClient:     httpclient.Default(),
	}
}

func (c *Client) SetCache(fc *cache.Cache) {
	c.fileCache = fc
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.apiToken != ""
}

func ExtractKeys(text string) []string {
	return pattern.ExtractKeys(text)
}

func (c *Client) FilterKeys(keys []string) []string {
	if c.projectKey == "" {
		return keys
	}
	prefix := strings.ToUpper(c.projectKey) + "-"
	var filtered []string
	for _, k := range keys {
		if strings.HasPrefix(strings.ToUpper(k), prefix) {
			filtered = append(filtered, k)
		}
	}
	return filtered
}

func (c *Client) FetchIssue(ctx context.Context, key string) (*Issue, error) {
	c.mu.RLock()
	if issue, ok := c.memo[key]; ok {
		c.mu.RUnlock()
		return issue, nil
	}
	c.mu.RUnlock()

	if c.fileCache != nil {
		var issue Issue
		if ok, _ := c.fileCache.Get(cacheNamespace, key, &issue); ok {
			c.mu.Lock()
			c.memo[key] = &issue
			c.mu.Unlock()
			return &issue, nil
		}
	}

	url := c.baseURL + "/rest/api/2/issue/" + key + "?fields=summary,priority,status,labels,issuetype,parent,fixVersions,components,description,assignee,subtasks"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("jira request: %w", err)
	}

	if c.email != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.email+":"+c.apiToken)))
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("jira fetch %s: %w", key, err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("jira read %s: %w", key, err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("jira status %d for %s: %s", resp.StatusCode, key, truncate.String(string(raw), 200))
	}

	var result struct {
		Key    string `json:"key"`
		Fields struct {
			Summary  string `json:"summary"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
			Status struct {
				Name string `json:"name"`
			} `json:"status"`
			IssueType struct {
				Name string `json:"name"`
			} `json:"issuetype"`
			Labels []string `json:"labels"`
			Parent struct {
				Key string `json:"key"`
			} `json:"parent"`
			FixVersions []struct {
				Name string `json:"name"`
			} `json:"fixVersions"`
			Components []struct {
				Name string `json:"name"`
			} `json:"components"`
			Description string `json:"description"`
			Assignee    struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
		} `json:"fields"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("jira decode %s: %w", key, err)
	}

	var fixVersions []string
	for _, fv := range result.Fields.FixVersions {
		fixVersions = append(fixVersions, fv.Name)
	}

	var components []string
	for _, comp := range result.Fields.Components {
		components = append(components, comp.Name)
	}

	issue := &Issue{
		Key:         result.Key,
		Summary:     result.Fields.Summary,
		Priority:    result.Fields.Priority.Name,
		Status:      result.Fields.Status.Name,
		Type:        result.Fields.IssueType.Name,
		Labels:      result.Fields.Labels,
		URL:         c.baseURL + "/browse/" + result.Key,
		EpicKey:     result.Fields.Parent.Key,
		FixVersions: fixVersions,
		Components:  components,
		Description: truncate.String(result.Fields.Description, 500),
		Assignee:    result.Fields.Assignee.DisplayName,
	}

	c.mu.Lock()
	c.memo[key] = issue
	c.mu.Unlock()

	if c.fileCache != nil {
		_ = c.fileCache.Set(cacheNamespace, key, *issue)
	}

	return issue, nil
}

func (c *Client) SearchByFixVersion(ctx context.Context, projectKey, fixVersion string) ([]*Issue, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("jira not configured")
	}

	jql := fmt.Sprintf("project = %s AND fixVersion = \"%s\"", projectKey, fixVersion)
	if projectKey == "" {
		jql = fmt.Sprintf("fixVersion = \"%s\"", fixVersion)
	}

	u := fmt.Sprintf("%s/rest/api/2/search?jql=%s&fields=summary,priority,status,issuetype,parent,fixVersions,components,assignee&maxResults=100",
		c.baseURL, url.QueryEscape(jql))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	if c.email != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.email+":"+c.apiToken)))
	} else {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("jira search read: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("jira search status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	var result struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary  string `json:"summary"`
				Priority struct {
					Name string `json:"name"`
				} `json:"priority"`
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
				Type struct {
					Name string `json:"name"`
				} `json:"issuetype"`
				FixVersions []struct {
					Name string `json:"name"`
				} `json:"fixVersions"`
				Assignee struct {
					DisplayName string `json:"displayName"`
				} `json:"assignee"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("jira search decode: %w", err)
	}

	var issues []*Issue
	for _, item := range result.Issues {
		issue := &Issue{
			Key:      item.Key,
			Summary:  item.Fields.Summary,
			Priority: item.Fields.Priority.Name,
			Status:   item.Fields.Status.Name,
			Type:     item.Fields.Type.Name,
			Assignee: item.Fields.Assignee.DisplayName,
			URL:      c.baseURL + "/browse/" + item.Key,
		}
		for _, fv := range item.Fields.FixVersions {
			issue.FixVersions = append(issue.FixVersions, fv.Name)
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (c *Client) EnrichKeys(ctx context.Context, keys []string) map[string]*Issue {
	result := make(map[string]*Issue)
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.maxConcurrency)

	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			issue, err := c.FetchIssue(ctx, k)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: jira fetch %s failed: %v\n", k, err)
				return
			}
			mu.Lock()
			result[k] = issue
			mu.Unlock()
		}(key)
	}
	wg.Wait()

	return result
}
