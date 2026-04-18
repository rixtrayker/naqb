package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestInterruptedError_Error(t *testing.T) {
	err := &InterruptedError{NodeID: "review", Reason: "waiting for approval"}
	msg := err.Error()
	if !strings.Contains(msg, "review") {
		t.Errorf("expected NodeID in error message, got: %q", msg)
	}
	if !strings.Contains(msg, "waiting for approval") {
		t.Errorf("expected Reason in error message, got: %q", msg)
	}
}

func TestIsInterrupted_True(t *testing.T) {
	err := &InterruptedError{NodeID: "pause", Reason: "human-in-the-loop"}
	interrupted, ok := IsInterrupted(err)
	if !ok {
		t.Fatal("expected IsInterrupted to return true")
	}
	if interrupted.NodeID != "pause" {
		t.Errorf("NodeID = %q, want pause", interrupted.NodeID)
	}
}

func TestIsInterrupted_False(t *testing.T) {
	err := errors.New("regular error")
	_, ok := IsInterrupted(err)
	if ok {
		t.Error("expected IsInterrupted to return false for regular error")
	}
}

func TestIsInterrupted_Nil(t *testing.T) {
	_, ok := IsInterrupted(nil)
	if ok {
		t.Error("expected IsInterrupted to return false for nil")
	}
}
