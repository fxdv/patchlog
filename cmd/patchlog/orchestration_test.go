package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestApplyStateReportsPartialRemoteFailure(t *testing.T) {
	state := &ApplyState{}
	ctx := context.Background()
	if err := state.Run(ctx, "version bump", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := state.Run(ctx, "atomic git push", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	err := state.Run(ctx, "provider publish", func(context.Context) error { return errors.New("remote returned 500") })
	var partial *PartialApplyError
	if !errors.As(err, &partial) {
		t.Fatalf("expected PartialApplyError, got %T: %v", err, err)
	}
	if got, want := partial.Completed, []string{"version bump", "atomic git push"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completed = %v, want %v", got, want)
	}
	if got := state.Completed(); !reflect.DeepEqual(got, partial.Completed) {
		t.Fatalf("state completed = %v, error completed = %v", got, partial.Completed)
	}
}
