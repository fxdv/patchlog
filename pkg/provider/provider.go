package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

type CommitMeta struct {
	PRNumber  int
	PRTitle   string
	Labels    []string
	Author    string
	AuthorURL string
	URL       string
}

type Release struct {
	ID      int64
	URL     string
	TagName string
}

type Type string

const (
	GitHub Type = "github"
	GitLab Type = "gitlab"
	Gitea  Type = "gitea"
)

type Config struct {
	Type    Type   `yaml:"type"`
	Token   string `yaml:"token"`
	Repo    string `yaml:"repo"`
	BaseURL string `yaml:"base_url"`
	Draft   *bool  `yaml:"draft"`
}

type Provider interface {
	FetchPR(ctx context.Context, prNumber int) (*CommitMeta, error)
	CreateRelease(ctx context.Context, version, notes string) (*Release, error)
}

func New(cfg Config) (Provider, error) {
	if cfg.Repo == "" {
		return nil, fmt.Errorf("provider: repo is required (owner/name)")
	}

	switch cfg.Type {
	case GitHub:
		base := cfg.BaseURL
		if base == "" {
			base = "https://api.github.com"
		}
		return &githubProvider{token: cfg.Token, repo: cfg.Repo, baseURL: base, draft: draftFlag(cfg.Draft), httpClient: httpclient.Default()}, nil
	case GitLab:
		base := cfg.BaseURL
		if base == "" {
			base = "https://gitlab.com"
		}
		base = strings.TrimRight(base, "/") + "/api/v4"
		return &gitlabProvider{token: cfg.Token, repo: cfg.Repo, baseURL: base, httpClient: httpclient.Default()}, nil
	case Gitea:
		base := cfg.BaseURL
		if base == "" {
			return nil, fmt.Errorf("gitea requires base_url")
		}
		return &giteaProvider{token: cfg.Token, repo: cfg.Repo, baseURL: base, draft: draftFlag(cfg.Draft), httpClient: httpclient.Default()}, nil
	default:
		return nil, fmt.Errorf("unknown provider type %q", cfg.Type)
	}
}

func draftFlag(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func newRequest(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	return req, nil
}

func doGet(ctx context.Context, httpClient *http.Client, req *http.Request) ([]byte, error) {
	resp, err := httpclient.DoWithRetry(httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

type githubProvider struct {
	token      string
	repo       string
	baseURL    string
	draft      bool
	httpClient *http.Client
}

type githubPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login   string `json:"login"`
		HTMLURL string `json:"html_url"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HTMLURL string `json:"html_url"`
}

type githubReleaseReq struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	Draft   bool   `json:"draft"`
}

type githubReleaseResp struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	TagName string `json:"tag_name"`
}

func (p *githubProvider) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	u := p.baseURL + "/repos/" + p.repo + path
	req, err := newRequest(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()
	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("github read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func (p *githubProvider) FetchPR(ctx context.Context, prNumber int) (*CommitMeta, error) {
	u := fmt.Sprintf("%s/repos/%s/pulls/%d", p.baseURL, p.repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	raw, err := doGet(ctx, p.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	var pr githubPR
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("github decode: %w", err)
	}
	meta := &CommitMeta{
		PRNumber:  pr.Number,
		PRTitle:   pr.Title,
		Author:    pr.User.Login,
		AuthorURL: pr.User.HTMLURL,
		URL:       pr.HTMLURL,
	}
	for _, l := range pr.Labels {
		meta.Labels = append(meta.Labels, l.Name)
	}
	return meta, nil
}

func (p *githubProvider) CreateRelease(ctx context.Context, version, notes string) (*Release, error) {
	body := githubReleaseReq{
		TagName: version,
		Name:    version,
		Body:    notes,
		Draft:   p.draft,
	}
	data, _ := json.Marshal(body)
	raw, err := p.do(ctx, "POST", "/releases", data)
	if err != nil {
		return nil, err
	}
	var resp githubReleaseResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("github decode: %w", err)
	}
	return &Release{
		ID:      resp.ID,
		URL:     resp.HTMLURL,
		TagName: resp.TagName,
	}, nil
}

type gitlabProvider struct {
	token      string
	repo       string
	baseURL    string
	httpClient *http.Client
}

type gitlabMergeRequest struct {
	IID    int    `json:"iid"`
	Title  string `json:"title"`
	Author struct {
		Username string `json:"username"`
		WebURL   string `json:"web_url"`
	} `json:"author"`
	Labels []string `json:"labels"`
	WebURL string   `json:"web_url"`
}

type gitlabReleaseReq struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	Description string `json:"description"`
	Ref         string `json:"ref"`
}

type gitlabReleaseResp struct {
	TagName string `json:"tag_name"`
	Links   struct {
		Self string `json:"self"`
	} `json:"_links"`
}

func (p *gitlabProvider) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	u := p.baseURL + "/projects/" + url.PathEscape(p.repo) + path
	req, err := newRequest(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("PRIVATE-TOKEN", p.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %w", err)
	}
	defer resp.Body.Close()
	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("gitlab read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func (p *gitlabProvider) FetchPR(ctx context.Context, prNumber int) (*CommitMeta, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests/%d", p.baseURL, url.PathEscape(p.repo), prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("PRIVATE-TOKEN", p.token)
	}
	raw, err := doGet(ctx, p.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %w", err)
	}
	var mr gitlabMergeRequest
	if err := json.Unmarshal(raw, &mr); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}
	return &CommitMeta{
		PRNumber:  mr.IID,
		PRTitle:   mr.Title,
		Labels:    mr.Labels,
		Author:    mr.Author.Username,
		AuthorURL: mr.Author.WebURL,
		URL:       mr.WebURL,
	}, nil
}

func (p *gitlabProvider) CreateRelease(ctx context.Context, version, notes string) (*Release, error) {
	body := gitlabReleaseReq{
		Name:        version,
		TagName:     version,
		Description: notes,
		Ref:         "HEAD",
	}
	data, _ := json.Marshal(body)
	raw, err := p.do(ctx, "POST", "/releases", data)
	if err != nil {
		return nil, err
	}
	var resp gitlabReleaseResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}
	return &Release{
		URL:     resp.Links.Self,
		TagName: resp.TagName,
	}, nil
}

type giteaProvider struct {
	token      string
	repo       string
	baseURL    string
	draft      bool
	httpClient *http.Client
}

type giteaPR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login   string `json:"login"`
		HTMLURL string `json:"html_url"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	HTMLURL string `json:"html_url"`
}

type giteaReleaseReq struct {
	TagName string `json:"tag_name"`
	Title   string `json:"title"`
	Note    string `json:"note"`
	IsDraft bool   `json:"is_draft"`
}

type giteaReleaseResp struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	TagName string `json:"tag_name"`
}

func (p *giteaProvider) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	u := p.baseURL + "/api/v1/repos/" + p.repo + path
	req, err := newRequest(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("Authorization", "token "+p.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitea: %w", err)
	}
	defer resp.Body.Close()
	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("gitea read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitea status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func (p *giteaProvider) FetchPR(ctx context.Context, prNumber int) (*CommitMeta, error) {
	u := fmt.Sprintf("%s/api/v1/repos/%s/pulls/%d", p.baseURL, p.repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("gitea: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")
	if p.token != "" {
		req.Header.Set("Authorization", "token "+p.token)
	}
	raw, err := doGet(ctx, p.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("gitea: %w", err)
	}
	var pr giteaPR
	if err := json.Unmarshal(raw, &pr); err != nil {
		return nil, fmt.Errorf("gitea decode: %w", err)
	}
	meta := &CommitMeta{
		PRNumber:  pr.Number,
		PRTitle:   pr.Title,
		Author:    pr.User.Login,
		AuthorURL: pr.User.HTMLURL,
		URL:       pr.HTMLURL,
	}
	for _, l := range pr.Labels {
		meta.Labels = append(meta.Labels, l.Name)
	}
	return meta, nil
}

func (p *giteaProvider) CreateRelease(ctx context.Context, version, notes string) (*Release, error) {
	body := giteaReleaseReq{
		TagName: version,
		Title:   version,
		Note:    notes,
		IsDraft: p.draft,
	}
	data, _ := json.Marshal(body)
	raw, err := p.do(ctx, "POST", "/releases", data)
	if err != nil {
		return nil, err
	}
	var resp giteaReleaseResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("gitea decode: %w", err)
	}
	return &Release{
		ID:      resp.ID,
		URL:     resp.HTMLURL,
		TagName: resp.TagName,
	}, nil
}
