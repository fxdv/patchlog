package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fxdv/patchlog/pkg/commitpolicy"
)

const (
	releasePlanFingerprintSchema = 1
	ReleasePlanSchema            = "https://patchlog.dev/schemas/release-plan/v1"
)

type ReleasePlanFile struct {
	Path         string `json:"path"`
	Mode         uint32 `json:"mode"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

type releasePlanPayload struct {
	SchemaVersion        int                    `json:"schema_version"`
	Phase                ReleasePhase           `json:"phase"`
	Head                 string                 `json:"head"`
	ProtectedBranch      string                 `json:"protected_branch,omitempty"`
	ReleaseBranch        string                 `json:"release_branch,omitempty"`
	CurrentVersion       string                 `json:"current_version,omitempty"`
	TargetVersion        string                 `json:"target_version"`
	Tag                  string                 `json:"tag,omitempty"`
	TagRequested         bool                   `json:"tag_requested"`
	PushRequested        bool                   `json:"push_requested"`
	PublishRequested     bool                   `json:"publish_requested"`
	PublishTarget        string                 `json:"publish_target,omitempty"`
	ConfluenceTarget     string                 `json:"confluence_target,omitempty"`
	RenderedOutputSHA256 string                 `json:"rendered_output_sha256,omitempty"`
	MutationTargets      []string               `json:"mutation_targets,omitempty"`
	Actions              []string               `json:"actions"`
	Files                []ReleasePlanFile      `json:"files"`
	CommitPolicy         *commitpolicy.Evidence `json:"commit_policy,omitempty"`
}

// ReleasePlanDocument is the stable, versioned JSON representation exported
// for review, audit storage, and external policy tooling.
type ReleasePlanDocument struct {
	Schema      string `json:"schema"`
	Fingerprint string `json:"fingerprint"`
	releasePlanPayload
}

func fingerprintReleasePlan(plan *ReleasePlan) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("release plan is nil")
	}
	payload := releasePlanPayloadFor(plan)
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func releasePlanPayloadFor(plan *ReleasePlan) releasePlanPayload {
	payload := releasePlanPayload{
		SchemaVersion:        releasePlanFingerprintSchema,
		Phase:                plan.phase,
		Head:                 plan.head,
		ProtectedBranch:      plan.protectedBranch,
		ReleaseBranch:        plan.releaseBranch,
		TargetVersion:        plan.targetVersion,
		Tag:                  plan.tagName,
		TagRequested:         plan.tagOptions.Tag,
		PushRequested:        plan.tagOptions.Push,
		PublishTarget:        plan.publishTarget,
		ConfluenceTarget:     plan.confluenceTarget,
		RenderedOutputSHA256: plan.renderedOutputHash,
		MutationTargets:      append([]string(nil), plan.mutationTargets...),
		Actions:              append([]string{}, plan.actions...),
		Files:                []ReleasePlanFile{},
	}
	if plan.policyEvidence != nil {
		evidence := plan.policyEvidence.Normalize()
		payload.CommitPolicy = &evidence
	}
	if plan.remoteRef != nil {
		payload.PublishRequested = true
	}
	if plan.bump != nil {
		payload.CurrentVersion = plan.bump.CurrentVersion
		payload.Files = make([]ReleasePlanFile, 0, len(plan.bump.Changes))
		for _, change := range plan.bump.Changes {
			before := sha256.Sum256(change.Before)
			after := sha256.Sum256(change.After)
			payload.Files = append(payload.Files, ReleasePlanFile{
				Path:         change.Path,
				Mode:         uint32(change.Mode.Perm()),
				BeforeSHA256: hex.EncodeToString(before[:]),
				AfterSHA256:  hex.EncodeToString(after[:]),
			})
		}
		sort.Slice(payload.Files, func(i, j int) bool {
			return payload.Files[i].Path < payload.Files[j].Path
		})
	}
	sort.Strings(payload.MutationTargets)
	return payload
}

func (p *ReleasePlan) Document() (ReleasePlanDocument, error) {
	if p == nil {
		return ReleasePlanDocument{}, fmt.Errorf("release plan is nil")
	}
	if p.fingerprint == "" {
		return ReleasePlanDocument{}, fmt.Errorf("release plan has no fingerprint")
	}
	return ReleasePlanDocument{
		Schema:             ReleasePlanSchema,
		Fingerprint:        p.fingerprint,
		releasePlanPayload: releasePlanPayloadFor(p),
	}, nil
}

func (p *ReleasePlan) JSON() ([]byte, error) {
	document, err := p.Document()
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode release plan JSON: %w", err)
	}
	return append(raw, '\n'), nil
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
