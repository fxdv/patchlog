package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

const releasePlanFingerprintSchema = 1

type fingerprintFile struct {
	Path         string `json:"path"`
	Mode         uint32 `json:"mode"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

type fingerprintPayload struct {
	Schema               int               `json:"schema"`
	Phase                ReleasePhase      `json:"phase"`
	Head                 string            `json:"head"`
	ProtectedBranch      string            `json:"protected_branch,omitempty"`
	ReleaseBranch        string            `json:"release_branch,omitempty"`
	CurrentVersion       string            `json:"current_version,omitempty"`
	TargetVersion        string            `json:"target_version"`
	Tag                  string            `json:"tag,omitempty"`
	TagRequested         bool              `json:"tag_requested"`
	PushRequested        bool              `json:"push_requested"`
	PublishRequested     bool              `json:"publish_requested"`
	PublishTarget        string            `json:"publish_target,omitempty"`
	ConfluenceTarget     string            `json:"confluence_target,omitempty"`
	RenderedOutputSHA256 string            `json:"rendered_output_sha256,omitempty"`
	MutationTargets      []string          `json:"mutation_targets,omitempty"`
	Actions              []string          `json:"actions"`
	Files                []fingerprintFile `json:"files"`
}

func fingerprintReleasePlan(plan *ReleasePlan) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("release plan is nil")
	}
	payload := fingerprintPayload{
		Schema:               releasePlanFingerprintSchema,
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
		Actions:              append([]string(nil), plan.actions...),
	}
	if plan.remoteRef != nil {
		payload.PublishRequested = true
	}
	if plan.bump != nil {
		payload.CurrentVersion = plan.bump.CurrentVersion
		payload.Files = make([]fingerprintFile, 0, len(plan.bump.Changes))
		for _, change := range plan.bump.Changes {
			before := sha256.Sum256(change.Before)
			after := sha256.Sum256(change.After)
			payload.Files = append(payload.Files, fingerprintFile{
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
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
