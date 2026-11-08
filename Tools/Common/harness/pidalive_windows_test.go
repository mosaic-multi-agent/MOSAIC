//go:build windows

package harness_test

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

// pidAlive reports whether a process with pid is still running.
func pidAlive(pid int) bool {
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false // no such process
	}
	defer syscall.CloseHandle(h) //nolint:errcheck
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
