package mica

import (
	"context"
	"errors"
	"testing"
)

func TestCallTimeoutUnwrap(t *testing.T) {
	err := NewCallTimeout("rpc timed out", context.DeadlineExceeded)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CallTimeout should unwrap DeadlineExceeded: %v", err)
	}
	var timeout *CallTimeout
	if !errors.As(err, &timeout) {
		t.Fatalf("errors.As CallTimeout failed: %v", err)
	}
}

func TestCallCancelledUnwrap(t *testing.T) {
	err := NewCallCancelled("rpc cancelled", context.Canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CallCancelled should unwrap Canceled: %v", err)
	}
	var cancelled *CallCancelled
	if !errors.As(err, &cancelled) {
		t.Fatalf("errors.As CallCancelled failed: %v", err)
	}
}
