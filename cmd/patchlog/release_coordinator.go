package main

import (
	"context"
	"fmt"

	"github.com/fxdv/patchlog/pkg/gittag"
)

type CoordinatedReleasePlan interface {
	CoreReleasePlan
	Phase() ReleasePhase
	RequireApproval(string) error
	ApplyProtectedPrepare(context.Context) (*gittag.Result, error)
	ApplyProtectedFinalize(context.Context) (*gittag.Result, error)
}

// ApplyApprovedRelease coordinates the three explicit release contracts while
// keeping command parsing and presentation out of mutation ordering.
func ApplyApprovedRelease(
	ctx context.Context,
	plan CoordinatedReleasePlan,
	approved string,
	publish ProviderPublishOperation,
) (*CoreReleaseApplyResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("approved release apply requires a plan")
	}
	if err := plan.RequireApproval(approved); err != nil {
		return nil, err
	}

	result := &CoreReleaseApplyResult{State: &ApplyState{}}
	var err error
	switch plan.Phase() {
	case ReleasePhasePrepare:
		result.Tag, err = plan.ApplyProtectedPrepare(ctx)
		if err == nil {
			result.State.completed = append(result.State.completed, "protected release prepare")
		}
	case ReleasePhaseFinalize:
		result.Tag, err = plan.ApplyProtectedFinalize(ctx)
		if err == nil {
			result.State.completed = append(result.State.completed, "protected release finalize")
		}
		if err == nil && publish != nil {
			if verifyErr := plan.VerifyRemoteRef(ctx); verifyErr != nil {
				err = result.State.Failure("immutable remote release ref verification", verifyErr)
			} else if ref, ok := plan.RemoteRef(); !ok {
				err = result.State.Failure("provider publish", fmt.Errorf("release plan has no remote ref"))
			} else {
				result.PublishURL, err = publish(ctx, ref)
				if err != nil {
					err = result.State.Failure("provider publish", err)
				}
			}
		}
	case ReleasePhaseDirect:
		return ApplyCoreRelease(ctx, CoreReleaseApplyRequest{
			Plan:            plan,
			PublishProvider: publish,
		})
	default:
		return result, fmt.Errorf("unsupported release phase %q", plan.Phase())
	}
	return result, err
}
