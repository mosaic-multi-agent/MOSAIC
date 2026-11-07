package harness

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// killGroup ends an agent process together with every descendant. Run creates
// one per invocation before Start (newKillGroup, build-tagged per platform),
// calls attach right after Start, and finishes with exactly one of kill (timeout
// or cancellation) or release (the process exited on its own; descendants are
// left alone).
type killGroup interface {
	attach(p *os.Process)
	kill(p *os.Process)
	release()
}

// killProcessTree terminates the process and, on Windows, its entire
// descendant tree. On non-Windows platforms it falls back to p.Kill(),
// preserving existing behavior exactly.
//
// Errors are intentionally swallowed: the process may have already exited by
// the time the kill is attempted, matching the existing _ = p.Kill() pattern.
func killProcessTree(p *os.Process) {
	if runtime.GOOS == "windows" {
		// /F forces termination; /T kills the entire process tree rooted at
		// the given PID, including grandchildren (e.g. node.js spawned by a
		// .cmd shim wrapper).
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(p.Pid)).Run()
		return
	}
	_ = p.Kill()
}
