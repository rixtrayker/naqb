package runtime

import "fmt"

// InterruptedError is returned when a graph node pauses for external input.
type InterruptedError struct {
	NodeID string
	Reason string
}

func (e *InterruptedError) Error() string {
	return fmt.Sprintf("interrupted at %s: %s", e.NodeID, e.Reason)
}

// IsInterrupted reports whether err is an interruption.
func IsInterrupted(err error) (*InterruptedError, bool) {
	if e, ok := err.(*InterruptedError); ok {
		return e, true
	}
	return nil, false
}
