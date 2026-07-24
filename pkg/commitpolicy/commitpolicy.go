// Package commitpolicy proves that the exact protected-branch commit selected
// for a release satisfies the hosting provider's required status-check policy.
package commitpolicy

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const EvidenceSchemaVersion = 1

// Request identifies the immutable commit and protected branch whose policy
// status must be proven.
type Request struct {
	Repository string
	Branch     string
	Commit     string
}

// RequiredCheck identifies a provider-required check and, when present, the
// integration that must have produced it.
type RequiredCheck struct {
	Context       string `json:"context"`
	IntegrationID int64  `json:"integration_id,omitempty"`
}

// Evidence is stable input to a release-plan fingerprint. Volatile provider
// timestamps and URLs are deliberately excluded.
type Evidence struct {
	SchemaVersion  int             `json:"schema_version"`
	Provider       string          `json:"provider"`
	Repository     string          `json:"repository"`
	Branch         string          `json:"branch"`
	Commit         string          `json:"commit"`
	RequiredChecks []RequiredCheck `json:"required_checks"`
}

// Verifier proves provider policy for one exact commit.
type Verifier interface {
	Verify(context.Context, Request) (Evidence, error)
}

// Normalize returns stable, sorted evidence suitable for equality checks and
// release-plan fingerprinting.
func (e Evidence) Normalize() Evidence {
	e.Provider = strings.TrimSpace(e.Provider)
	e.Repository = strings.TrimSpace(e.Repository)
	e.Branch = strings.TrimSpace(e.Branch)
	e.Commit = strings.TrimSpace(e.Commit)
	if e.SchemaVersion == 0 {
		e.SchemaVersion = EvidenceSchemaVersion
	}
	e.RequiredChecks = append([]RequiredCheck(nil), e.RequiredChecks...)
	sort.Slice(e.RequiredChecks, func(i, j int) bool {
		if e.RequiredChecks[i].Context == e.RequiredChecks[j].Context {
			return e.RequiredChecks[i].IntegrationID < e.RequiredChecks[j].IntegrationID
		}
		return e.RequiredChecks[i].Context < e.RequiredChecks[j].Context
	})
	return e
}

// Validate rejects incomplete evidence so an implementation cannot
// accidentally turn "no policy data" into a successful verification.
func (e Evidence) Validate(req Request) error {
	e = e.Normalize()
	if e.SchemaVersion != EvidenceSchemaVersion {
		return fmt.Errorf("unsupported commit-policy evidence schema %d", e.SchemaVersion)
	}
	if e.Provider == "" || e.Repository == "" || e.Branch == "" || e.Commit == "" {
		return fmt.Errorf("commit-policy evidence is incomplete")
	}
	if e.Repository != req.Repository || e.Branch != req.Branch || e.Commit != req.Commit {
		return fmt.Errorf(
			"commit-policy evidence identity mismatch: got %s %s@%s, expected %s %s@%s",
			e.Repository, e.Branch, e.Commit, req.Repository, req.Branch, req.Commit,
		)
	}
	if len(e.RequiredChecks) == 0 {
		return fmt.Errorf("protected branch %s has no required status checks", req.Branch)
	}
	for _, check := range e.RequiredChecks {
		if strings.TrimSpace(check.Context) == "" {
			return fmt.Errorf("commit-policy evidence contains an empty required check")
		}
	}
	return nil
}
