package deps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fxdv/patchlog/pkg/httpclient"
)

func FetchChangelogs(ctx context.Context, changes []Change, opts FetchOptions) []Change {
	client := httpclient.Default()

	limit := opts.MaxDependencies
	if limit <= 0 {
		limit = len(changes)
	}

	fetched := 0
	for i := range changes {
		if fetched >= limit {
			break
		}
		c := &changes[i]
		switch c.Ecosystem {
		case EcosystemNPM:
			fetchNPM(ctx, client, c, opts)
			fetched++
		case EcosystemCargo:
			fetchCrates(ctx, client, c, opts)
			fetched++
		case EcosystemPyPI:
			fetchPyPI(ctx, client, c, opts)
			fetched++
		case EcosystemGo:
			fetchGo(ctx, client, c, opts)
			fetched++
		}
	}
	return changes
}

func SetChangelogURLs(changes []Change) {
	for i := range changes {
		c := &changes[i]
		switch c.Ecosystem {
		case EcosystemNPM:
			c.ChangelogURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", c.Name, stripVersionPrefix(c.NewVersion))
		case EcosystemCargo:
			c.ChangelogURL = fmt.Sprintf("https://crates.io/crates/%s/%s", c.Name, stripVersionPrefix(c.NewVersion))
		case EcosystemPyPI:
			c.ChangelogURL = fmt.Sprintf("https://pypi.org/project/%s/%s/", c.Name, stripVersionPrefix(c.NewVersion))
		case EcosystemGo:
			c.ChangelogURL = fmt.Sprintf("https://pkg.go.dev/%s", c.Name)
		}
	}
}

func fetchNPM(ctx context.Context, client *http.Client, c *Change, opts FetchOptions) {
	registry := opts.NPMRegistry
	if registry == "" {
		registry = "https://registry.npmjs.org"
	}
	registry = strings.TrimRight(registry, "/")

	c.ChangelogURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", c.Name, stripVersionPrefix(c.NewVersion))

	url := fmt.Sprintf("%s/%s", registry, c.Name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.DoWithRetry(client, req, httpclient.DefaultRetry)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	var result struct {
		Description string `json:"description"`
		Repository  struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"repository"`
		Readme string `json:"readme"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Description != "" {
		c.Changelog = result.Description
	}

	if opts.GitHubReleases {
		owner, repo := parseGitHubRepo(result.Repository.URL)
		if owner != "" {
			if body := fetchGitHubReleasesBetween(ctx, client, owner, repo, c.OldVersion, c.NewVersion); body != "" {
				c.Changelog = body
			}
		}
	}
}

func fetchCrates(ctx context.Context, client *http.Client, c *Change, opts FetchOptions) {
	registry := opts.CratesRegistry
	if registry == "" {
		registry = "https://crates.io"
	}
	registry = strings.TrimRight(registry, "/")

	c.ChangelogURL = fmt.Sprintf("%s/crates/%s/%s", registry, c.Name, stripVersionPrefix(c.NewVersion))

	url := fmt.Sprintf("%s/api/v1/crates/%s", registry, c.Name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "patchlog")

	resp, err := httpclient.DoWithRetry(client, req, httpclient.DefaultRetry)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	var result struct {
		Crate struct {
			Description   string `json:"description"`
			Repository    string `json:"repository"`
			Documentation string `json:"documentation"`
			Homepage      string `json:"homepage"`
		} `json:"crate"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Crate.Description != "" {
		c.Changelog = result.Crate.Description
	}

	if opts.GitHubReleases && result.Crate.Repository != "" {
		owner, repo := parseGitHubRepo(result.Crate.Repository)
		if owner != "" {
			if body := fetchGitHubReleasesBetween(ctx, client, owner, repo, c.OldVersion, c.NewVersion); body != "" {
				c.Changelog = body
			}
		}
	}
}

func fetchPyPI(ctx context.Context, client *http.Client, c *Change, opts FetchOptions) {
	registry := opts.PyPIRegistry
	if registry == "" {
		registry = "https://pypi.org"
	}
	registry = strings.TrimRight(registry, "/")

	c.ChangelogURL = fmt.Sprintf("%s/project/%s/%s/", registry, c.Name, stripVersionPrefix(c.NewVersion))

	url := fmt.Sprintf("%s/pypi/%s/json", registry, c.Name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}

	resp, err := httpclient.DoWithRetry(client, req, httpclient.DefaultRetry)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	var result struct {
		Info struct {
			Summary     string            `json:"summary"`
			ProjectURLs map[string]string `json:"project_urls"`
		} `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if result.Info.Summary != "" {
		c.Changelog = result.Info.Summary
	}

	if opts.GitHubReleases && result.Info.ProjectURLs != nil {
		for _, u := range result.Info.ProjectURLs {
			owner, repo := parseGitHubRepo(u)
			if owner != "" {
				if body := fetchGitHubReleasesBetween(ctx, client, owner, repo, c.OldVersion, c.NewVersion); body != "" {
					c.Changelog = body
					break
				}
			}
		}
	}
}

func fetchGo(ctx context.Context, client *http.Client, c *Change, opts FetchOptions) {
	c.ChangelogURL = fmt.Sprintf("https://pkg.go.dev/%s", c.Name)

	if opts.GitHubReleases && strings.HasPrefix(c.Name, "github.com/") {
		parts := strings.Split(strings.TrimPrefix(c.Name, "github.com/"), "/")
		if len(parts) >= 2 {
			owner := parts[0]
			repo := parts[1]
			if body := fetchGitHubReleasesBetween(ctx, client, owner, repo, c.OldVersion, c.NewVersion); body != "" {
				c.Changelog = body
			}
		}
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

func fetchGitHubReleasesBetween(ctx context.Context, client *http.Client, owner, repo, oldVer, newVer string) string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpclient.DoWithRetry(client, req, httpclient.DefaultRetry)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}

	body, err := httpclient.ReadResponse(resp)
	if err != nil {
		return ""
	}

	var releases []ghRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return ""
	}

	var notes []string
	for _, r := range releases {
		if r.Body == "" {
			continue
		}
		if isVersionBetween(r.TagName, oldVer, newVer) {
			title := r.TagName
			if r.Name != "" {
				title = r.Name
			}
			notes = append(notes, fmt.Sprintf("**%s**\n%s", title, strings.TrimSpace(r.Body)))
		}
	}

	return strings.Join(notes, "\n\n---\n\n")
}

func parseGitHubRepo(rawURL string) (owner, repo string) {
	rawURL = strings.TrimSpace(rawURL)
	rawURL = strings.TrimPrefix(rawURL, "git+")
	rawURL = strings.TrimPrefix(rawURL, "git://")
	rawURL = strings.TrimPrefix(rawURL, "git@github.com:")
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")

	if !strings.HasPrefix(rawURL, "github.com/") {
		return "", ""
	}

	rest := strings.TrimPrefix(rawURL, "github.com/")
	rest = strings.TrimSuffix(rest, ".git")
	rest = strings.TrimSuffix(rest, "/")

	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}
