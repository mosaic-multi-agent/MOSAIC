//go:build !windows

package harness_test

import (
	"os"
	"syscall"
)

// pidAlive reports whether a process with pid is still running.
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
