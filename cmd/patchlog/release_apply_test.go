package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/fxdv/patchlog/pkg/gittag"
)

type fakeCoreReleasePlan struct {
	events    *[]string
	hasBump   bool
	hasTag    bool
	verifyErr error
}

func (f fakeCoreReleasePlan) HasBump() bool { return f.hasBump }
func (f fakeCoreReleasePlan) HasTag() bool  { return f.hasTag }
func (f fakeCoreReleasePlan) Revalidate(context.Context) error {
	*f.events = append(*f.events, "revalidate")
	return nil
}
func (f fakeCoreReleasePlan) ApplyBump() error {
	*f.events = append(*f.events, "bump")
	return nil
}
func (f fakeCoreReleasePlan) ApplyGit(context.Context) (*gittag.Result, error) {
	*f.events = append(*f.events, "git")
	return &gittag.Result{Tag: "v1.0.0", Pushed: true}, nil
}
func (f fakeCoreReleasePlan) VerifyRemoteRef(context.Context) error {
	*f.events = append(*f.events, "verify-remote")
	return f.verifyErr
}
func (f fakeCoreReleasePlan) RemoteRef() (RemoteReleaseRef, bool) {
	return RemoteReleaseRef{tag: "v1.0.0"}, true
}

func TestApplyCoreReleaseUsesPlannedOrder(t *testing.T) {
	var events []string
	plan := fakeCoreReleasePlan{events: &events, hasBump: true, hasTag: true}
	result, err := ApplyCoreRelease(context.Background(), CoreReleaseApplyRequest{
		Plan: plan,
		PublishProvider: func(_ context.Context, ref RemoteReleaseRef) (string, error) {
			events = append(events, "publish:"+ref.Tag())
			return "https://example.test/releases/v1.0.0", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"revalidate", "bump", "git", "verify-remote", "publish:v1.0.0"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if got := result.State.Completed(); !reflect.DeepEqual(got, []string{"version bump", "git commit/tag/push", "provider publish"}) {
		t.Fatalf("completed = %v", got)
	}
}

func TestApplyCoreReleaseReportsVerificationAfterLocalCompletion(t *testing.T) {
	var events []string
	plan := fakeCoreReleasePlan{events: &events, hasBump: true, hasTag: true, verifyErr: errors.New("tag mismatch")}
	_, err := ApplyCoreRelease(context.Background(), CoreReleaseApplyRequest{
		Plan: plan,
		PublishProvider: func(context.Context, RemoteReleaseRef) (string, error) {
			t.Fatal("publish should not run")
			return "", nil
		},
	})
	var partial *PartialApplyError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %T %v", err, err)
	}
	want := []string{"version bump", "git commit/tag/push"}
	if !reflect.DeepEqual(partial.Completed, want) {
		t.Fatalf("completed = %v, want %v", partial.Completed, want)
	}
}
