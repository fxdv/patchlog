package main

import (
	"context"
	"fmt"
	"strings"
)

type mutation func(context.Context) error

// ApplyState records the irreversible boundary and makes partial completion
// explicit when a later local or remote operation fails.
type ApplyState struct {
	completed []string
}

type PartialApplyError struct {
	Completed []string
	Failed    string
	Err       error
}

func (e *PartialApplyError) Error() string {
	if len(e.Completed) == 0 {
		return fmt.Sprintf("apply %s failed before any mutation completed: %v", e.Failed, e.Err)
	}
	return fmt.Sprintf("apply %s failed after [%s] completed: %v", e.Failed, strings.Join(e.Completed, ", "), e.Err)
}

func (e *PartialApplyError) Unwrap() error { return e.Err }

func (s *ApplyState) Run(ctx context.Context, name string, operation mutation) error {
	if operation == nil {
		return nil
	}
	if err := operation(ctx); err != nil {
		return &PartialApplyError{
			Completed: append([]string(nil), s.completed...),
			Failed:    name,
			Err:       err,
		}
	}
	s.completed = append(s.completed, name)
	return nil
}

func (s *ApplyState) Failure(name string, err error) error {
	return &PartialApplyError{
		Completed: append([]string(nil), s.completed...),
		Failed:    name,
		Err:       err,
	}
}

func (s *ApplyState) Completed() []string {
	return append([]string(nil), s.completed...)
}
