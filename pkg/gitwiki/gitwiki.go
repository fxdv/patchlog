// Package gitwiki provides a GitLab wiki API client for changelog accumulation.
package gitwiki

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

type Config struct {
	Token   string
	Repo    string
	BaseURL string
	Slug    string
}

type Client struct {
	token      string
	repo       string
	baseURL    string
	slug       string
	httpClient *http.Client
}

type Page struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"web_url"`
}

func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://gitlab.com/api/v4"
	}
	base = strings.TrimRight(base, "/")
	slug := cfg.Slug
	if slug == "" {
		slug = "changelog"
	}
	return &Client{
		token:      cfg.Token,
		repo:       cfg.Repo,
		baseURL:    base,
		slug:       slug,
		httpClient: httpclient.Default(),
	}
}

func (c *Client) Configured() bool {
	return c.token != "" && c.repo != ""
}

func (c *Client) projectPath() string {
	return url.PathEscape(c.repo)
}

func (c *Client) GetPage(ctx context.Context, slug string) (*Page, error) {
	if slug == "" {
		slug = c.slug
	}
	u := fmt.Sprintf("%s/projects/%s/wikis/%s", c.baseURL, c.projectPath(), url.PathEscape(slug))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("gitwiki get: %w", err)
	}
	c.setAuth(req)

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("gitwiki get: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("gitwiki read: %w", err)
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitwiki status %d: %s", resp.StatusCode, string(raw))
	}

	return decodeWikiPage(raw)
}

func (c *Client) CreatePage(ctx context.Context, title, content string) (*Page, error) {
	data, err := encodeWikiBody(title, content)
	if err != nil {
		return nil, fmt.Errorf("gitwiki create: %w", err)
	}

	u := fmt.Sprintf("%s/projects/%s/wikis", c.baseURL, c.projectPath())
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gitwiki create: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("gitwiki create: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("gitwiki read: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitwiki status %d: %s", resp.StatusCode, string(raw))
	}

	return decodeWikiPage(raw)
}

func (c *Client) UpdatePage(ctx context.Context, slug, title, content string) (*Page, error) {
	if slug == "" {
		slug = c.slug
	}
	data, err := encodeWikiBody(title, content)
	if err != nil {
		return nil, fmt.Errorf("gitwiki update: %w", err)
	}

	u := fmt.Sprintf("%s/projects/%s/wikis/%s", c.baseURL, c.projectPath(), url.PathEscape(slug))
	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gitwiki update: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("gitwiki update: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("gitwiki read: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitwiki status %d: %s", resp.StatusCode, string(raw))
	}

	return decodeWikiPage(raw)
}

func (c *Client) AccumulatePage(ctx context.Context, title, newContent string) (*Page, bool, error) {
	existing, err := c.GetPage(ctx, c.slug)
	if err != nil {
		return nil, false, err
	}

	combined := newContent
	if existing != nil && existing.Content != "" {
		combined = newContent + "\n---\n\n" + existing.Content
		page, err := c.UpdatePage(ctx, c.slug, title, combined)
		if err != nil {
			return nil, false, err
		}
		return page, true, nil
	}

	page, err := c.CreatePage(ctx, title, combined)
	if err != nil {
		return nil, false, err
	}
	return page, false, nil
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")
	if c.token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.token)
	}
}

func decodeWikiPage(raw []byte) (*Page, error) {
	var apiResp struct {
		Slug    string `json:"slug"`
		Title   string `json:"title"`
		Content string `json:"content"`
		WebURL  string `json:"web_url"`
	}
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("gitwiki decode: %w", err)
	}
	return &Page{
		Slug:    apiResp.Slug,
		Title:   apiResp.Title,
		Content: apiResp.Content,
		URL:     apiResp.WebURL,
	}, nil
}

func encodeWikiBody(title, content string) ([]byte, error) {
	body := map[string]string{
		"title":   title,
		"content": content,
	}
	return json.Marshal(body)
}
