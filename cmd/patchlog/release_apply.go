package main

import (
	"context"
	"fmt"

	"github.com/fxdv/patchlog/pkg/gittag"
)

type ProviderPublishOperation func(context.Context, RemoteReleaseRef) (string, error)

type CoreReleasePlan interface {
	HasBump() bool
	HasTag() bool
	Revalidate(context.Context) error
	ApplyBump() error
	ApplyGit(context.Context) (*gittag.Result, error)
	VerifyRemoteRef(context.Context) error
	RemoteRef() (RemoteReleaseRef, bool)
}

type CoreReleaseApplyRequest struct {
	Plan            CoreReleasePlan
	PublishProvider ProviderPublishOperation
}

type CoreReleaseApplyResult struct {
	State      *ApplyState
	Tag        *gittag.Result
	PublishURL string
}

// ApplyCoreRelease owns the ordered local/remote transaction boundary. Output
// rendering and presentation stay outside this executor and are independently
// testable.
func ApplyCoreRelease(ctx context.Context, req CoreReleaseApplyRequest) (*CoreReleaseApplyResult, error) {
	if req.Plan == nil {
		return nil, fmt.Errorf("core release apply requires a plan")
	}
	result := &CoreReleaseApplyResult{State: &ApplyState{}}
	if req.Plan.HasBump() {
		if err := req.Plan.Revalidate(ctx); err != nil {
			return result, fmt.Errorf("release plan changed before apply: %w", err)
		}
		if err := result.State.Run(ctx, "version bump", func(context.Context) error {
			return req.Plan.ApplyBump()
		}); err != nil {
			return result, err
		}
	}
	if req.Plan.HasTag() {
		if err := result.State.Run(ctx, "git commit/tag/push", func(ctx context.Context) error {
			var err error
			result.Tag, err = req.Plan.ApplyGit(ctx)
			return err
		}); err != nil {
			return result, err
		}
	}
	if req.PublishProvider != nil {
		if err := req.Plan.VerifyRemoteRef(ctx); err != nil {
			return result, result.State.Failure("immutable remote release ref verification", err)
		}
		ref, ok := req.Plan.RemoteRef()
		if !ok {
			return result, result.State.Failure("provider publish", fmt.Errorf("release plan has no remote ref"))
		}
		if err := result.State.Run(ctx, "provider publish", func(ctx context.Context) error {
			var err error
			result.PublishURL, err = req.PublishProvider(ctx, ref)
			return err
		}); err != nil {
			return result, err
		}
	}
	return result, nil
}
