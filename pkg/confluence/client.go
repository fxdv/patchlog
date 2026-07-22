// Package confluence provides a Confluence Cloud/Server API client for
// creating, updating, and enriching release note pages with analytics panels,
// charts, gamification, and trends dashboards.
package confluence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fxdv/patchlog/pkg/cache"
	"github.com/fxdv/patchlog/pkg/httpclient"
	"github.com/fxdv/patchlog/pkg/i18n"
	"github.com/fxdv/patchlog/pkg/internal/truncate"
	"github.com/fxdv/patchlog/pkg/safehtml"
)

const cacheNamespace = "confluence"

var activeLabels = i18n.ConfluenceLabelsFor(i18n.LangRU)

func SetConfluenceLabels(labels i18n.ConfluenceLabels) {
	activeLabels = labels
}

const (
	colorGreen    = "#14892c"
	colorRed      = "#d04437"
	colorYellow   = "#f0a818"
	colorGrey     = "#666"
	colorAccent   = "#4a90d9"
	colorDarkText = "#2c3e50"
	colorMuted    = "#999"
)

const maxItemsBeforeExpand = 8

func Spacer() string {
	return "<p>&nbsp;</p>"
}

func RenderCommandFooter(command string) string {
	if command == "" {
		return ""
	}
	return fmt.Sprintf(`<p>&nbsp;</p><p style="font-size: 11px; color: %s; font-family: monospace; margin: 8px 0;">%s</p>`,
		colorMuted, safehtml.Text(command))
}

type Config struct {
	BaseURL         string
	Email           string
	APIToken        string
	SpaceKey        string
	ParentPageID    string
	Labels          []string
	ViewRestriction []string
	EditRestriction []string
	Template        string
}

// Client is a Confluence Cloud/Server API client for page management.
type Client struct {
	baseURL         string
	email           string
	apiToken        string
	spaceKey        string
	parentPageID    string
	labels          []string
	viewRestriction []string
	editRestriction []string
	httpClient      *http.Client
	fileCache       *cache.Cache
}

type Page struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type pageResult struct {
	Results []pageResultItem `json:"results"`
}

type pageResultItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

type pageBody struct {
	Storage struct {
		Value          string `json:"value"`
		Representation string `json:"representation"`
	} `json:"storage"`
}

type createPageReq struct {
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Space     *spaceKey  `json:"space,omitempty"`
	Ancestors []ancestor `json:"ancestors,omitempty"`
	Body      struct {
		Storage struct {
			Value          string `json:"value"`
			Representation string `json:"representation"`
		} `json:"storage"`
	} `json:"body"`
	Version *versionInfo `json:"version,omitempty"`
}

type spaceKey struct {
	Key string `json:"key"`
}

type ancestor struct {
	ID string `json:"id"`
}

type versionInfo struct {
	Number  int    `json:"number"`
	Message string `json:"message"`
}

func NewClient(cfg Config) *Client {
	return &Client{
		baseURL:         strings.TrimRight(cfg.BaseURL, "/"),
		email:           cfg.Email,
		apiToken:        cfg.APIToken,
		spaceKey:        cfg.SpaceKey,
		parentPageID:    cfg.ParentPageID,
		labels:          cfg.Labels,
		viewRestriction: cfg.ViewRestriction,
		editRestriction: cfg.EditRestriction,
		httpClient:      httpclient.Default(),
	}
}

func (c *Client) SetCache(fc *cache.Cache) {
	c.fileCache = fc
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.apiToken != "" && c.spaceKey != ""
}

func (c *Client) FindPage(ctx context.Context, title string) (*Page, error) {
	if c.fileCache != nil {
		var page Page
		if ok, _ := c.fileCache.Get(cacheNamespace, title, &page); ok {
			return &page, nil
		}
	}

	u := fmt.Sprintf("%s/rest/api/content?spaceKey=%s&title=%s&type=page&limit=1",
		c.baseURL, c.spaceKey, url.QueryEscape(title))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence find: %w", err)
	}
	c.setAuth(req)

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("confluence find: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("confluence read: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("confluence status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	var result pageResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("confluence decode: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	item := result.Results[0]
	page := &Page{
		ID:    item.ID,
		Title: item.Title,
		URL:   c.baseURL + item.Links.WebUI,
	}

	if c.fileCache != nil {
		_ = c.fileCache.Set(cacheNamespace, title, *page)
	}

	return page, nil
}

func (c *Client) CreatePage(ctx context.Context, title, body string) (*Page, error) {
	reqBody := createPageReq{
		Type:  "page",
		Title: title,
		Space: &spaceKey{Key: c.spaceKey},
	}
	if c.parentPageID != "" {
		reqBody.Ancestors = []ancestor{{ID: c.parentPageID}}
	}
	reqBody.Body.Storage.Value = body
	reqBody.Body.Storage.Representation = "storage"

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("confluence marshal: %w", err)
	}

	u := c.baseURL + "/rest/api/content"
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("confluence create: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("confluence create: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("confluence read: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("confluence status %d: %s", resp.StatusCode, truncate.String(string(raw), 300))
	}

	var result struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("confluence decode: %w", err)
	}

	page := &Page{
		ID:    result.ID,
		Title: result.Title,
		URL:   c.baseURL + result.Links.WebUI,
	}

	if c.fileCache != nil {
		_ = c.fileCache.Set(cacheNamespace, title, *page)
	}

	return page, nil
}

func (c *Client) UpdatePage(ctx context.Context, pageID, title, body string) (*Page, error) {
	currentVersion, err := c.getPageVersion(ctx, pageID)
	if err != nil {
		return nil, err
	}

	reqBody := createPageReq{
		Type:  "page",
		Title: title,
		Version: &versionInfo{
			Number:  currentVersion + 1,
			Message: "Updated by patchlog",
		},
	}
	reqBody.Body.Storage.Value = body
	reqBody.Body.Storage.Representation = "storage"

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("confluence marshal: %w", err)
	}

	u := c.baseURL + "/rest/api/content/" + pageID
	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("confluence update: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("confluence update: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("confluence read: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("confluence status %d: %s", resp.StatusCode, truncate.String(string(raw), 300))
	}

	var result struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("confluence decode: %w", err)
	}

	page := &Page{
		ID:    result.ID,
		Title: result.Title,
		URL:   c.baseURL + result.Links.WebUI,
	}

	if c.fileCache != nil {
		_ = c.fileCache.Invalidate(cacheNamespace, title)
	}

	return page, nil
}

func (c *Client) getPageVersion(ctx context.Context, pageID string) (int, error) {
	u := c.baseURL + "/rest/api/content/" + pageID + "?expand=version"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return 0, err
	}
	c.setAuth(req)

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, _ := httpclient.ReadResponse(resp)

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("confluence get version status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	var result struct {
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, fmt.Errorf("confluence decode version: %w", err)
	}
	return result.Version.Number, nil
}

func (c *Client) PublishOrUpdate(ctx context.Context, title, body string) (*Page, bool, error) {
	existing, err := c.FindPage(ctx, title)
	if err != nil {
		return nil, false, err
	}

	if existing != nil {
		page, err := c.UpdatePage(ctx, existing.ID, title, body)
		if err != nil {
			return nil, false, err
		}
		return page, true, nil
	}

	page, err := c.CreatePage(ctx, title, body)
	if err != nil {
		return nil, false, err
	}
	return page, false, nil
}

func (c *Client) GetPageBody(ctx context.Context, title string) (string, *Page, error) {
	page, err := c.FindPage(ctx, title)
	if err != nil {
		return "", nil, err
	}
	if page == nil {
		return "", nil, nil
	}

	u := c.baseURL + "/rest/api/content/" + page.ID + "?expand=body.storage"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", nil, fmt.Errorf("confluence get body: %w", err)
	}
	c.setAuth(req)

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return "", nil, fmt.Errorf("confluence get body: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return "", nil, fmt.Errorf("confluence read: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("confluence status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	var result struct {
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", nil, fmt.Errorf("confluence decode: %w", err)
	}

	return result.Body.Storage.Value, page, nil
}

func (c *Client) AccumulatePage(ctx context.Context, title, newSectionBody string) (*Page, bool, error) {
	existingBody, existingPage, err := c.GetPageBody(ctx, title)
	if err != nil {
		return nil, false, err
	}

	if existingPage != nil {
		combined := newSectionBody
		if existingBody != "" {
			combined = newSectionBody + "<hr/>" + existingBody
		}
		page, err := c.UpdatePage(ctx, existingPage.ID, title, combined)
		if err != nil {
			return nil, false, err
		}
		return page, true, nil
	}

	page, err := c.CreatePage(ctx, title, newSectionBody)
	if err != nil {
		return nil, false, err
	}
	return page, false, nil
}

func (c *Client) AddLabels(ctx context.Context, pageID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}

	var labelPayloads []map[string]string
	for _, l := range labels {
		labelPayloads = append(labelPayloads, map[string]string{"prefix": "global", "name": l})
	}

	data, err := json.Marshal(labelPayloads)
	if err != nil {
		return fmt.Errorf("confluence labels marshal: %w", err)
	}

	u := c.baseURL + "/rest/api/content/" + pageID + "/label"
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("confluence labels: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return fmt.Errorf("confluence labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := httpclient.ReadResponse(resp)
		return fmt.Errorf("confluence labels status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	return nil
}

func (c *Client) SetRestrictions(ctx context.Context, pageID string, viewUsers, editUsers []string) error {
	if len(viewUsers) == 0 && len(editUsers) == 0 {
		return nil
	}

	restrictions := map[string]any{
		"view": map[string]any{"user": []any{}, "group": []any{}},
		"edit": map[string]any{"user": []any{}, "group": []any{}},
	}

	for _, u := range viewUsers {
		restrictions["view"].(map[string]any)["user"] = append(restrictions["view"].(map[string]any)["user"].([]any), map[string]string{"username": u})
	}
	for _, u := range editUsers {
		restrictions["edit"].(map[string]any)["user"] = append(restrictions["edit"].(map[string]any)["user"].([]any), map[string]string{"username": u})
	}

	data, err := json.Marshal(restrictions)
	if err != nil {
		return fmt.Errorf("confluence restrictions marshal: %w", err)
	}

	u := c.baseURL + "/rest/api/content/" + pageID + "/restriction"
	req, err := http.NewRequestWithContext(ctx, "PUT", u, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("confluence restrictions: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return fmt.Errorf("confluence restrictions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		raw, _ := httpclient.ReadResponse(resp)
		return fmt.Errorf("confluence restrictions status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	return nil
}

type SiblingPage struct {
	ID    string
	Title string
	URL   string
}

func (c *Client) FindSiblingPages(ctx context.Context, titlePrefix string) ([]SiblingPage, error) {
	cql := fmt.Sprintf("space=%s AND type=page AND title~\"%s*\"", c.spaceKey, titlePrefix)
	u := fmt.Sprintf("%s/rest/api/content/search?cql=%s&limit=50", c.baseURL, url.QueryEscape(cql))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("confluence sibling search: %w", err)
	}
	c.setAuth(req)

	resp, err := httpclient.DoWithRetry(c.httpClient, req, httpclient.DefaultRetry)
	if err != nil {
		return nil, fmt.Errorf("confluence sibling search: %w", err)
	}
	defer resp.Body.Close()

	raw, err := httpclient.ReadResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("confluence sibling read: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("confluence sibling status %d: %s", resp.StatusCode, truncate.String(string(raw), 200))
	}

	var result pageResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("confluence sibling decode: %w", err)
	}

	var pages []SiblingPage
	for _, item := range result.Results {
		pages = append(pages, SiblingPage{
			ID:    item.ID,
			Title: item.Title,
			URL:   c.baseURL + item.Links.WebUI,
		})
	}

	return pages, nil
}

func (c *Client) setAuth(req *http.Request) {
	if c.email != "" && c.apiToken != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.email+":"+c.apiToken)))
	} else if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}
	req.Header.Set("Accept", "application/json")
}
