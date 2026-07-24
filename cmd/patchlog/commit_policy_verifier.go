package main

import (
	"fmt"
	"strings"

	"github.com/fxdv/patchlog/pkg/commitpolicy"
	"github.com/fxdv/patchlog/pkg/config"
)

func newCommitPolicyVerifier(cfg config.Config) (commitpolicy.Verifier, error) {
	providerType := strings.TrimSpace(cfg.Provider.Type)
	if providerType == "" || strings.TrimSpace(cfg.Provider.Repo) == "" {
		return nil, withHint(
			fmt.Errorf("protected finalize requires provider.type and provider.repo for commit-policy verification"),
			"configure the GitHub repository and a token with read access to administration and checks, then rerun `patchlog release --dry-run`",
		)
	}
	switch providerType {
	case "github":
		verifier, err := commitpolicy.NewGitHub(commitpolicy.GitHubConfig{
			Token:      cfg.Provider.Token,
			Repository: cfg.Provider.Repo,
			BaseURL:    cfg.Provider.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		return verifier, nil
	default:
		return nil, withHint(
			fmt.Errorf("protected finalize commit-policy verification does not yet support provider %q", providerType),
			"use the GitHub protected workflow for the stable 0.2 contract; GitLab and Gitea remain direct-mode compatibility providers",
		)
	}
}
