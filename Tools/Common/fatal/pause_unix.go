//go:build !windows

package fatal

import "io"

// discardPendingInput is a no-op: the pause only happens on Windows.
func discardPendingInput(io.Reader) {}
