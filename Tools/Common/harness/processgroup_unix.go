//go:build !windows

package harness

import (
	"os"
	"os/exec"
	"syscall"
)

// pgroup runs the agent as the leader of its own process group, so a kill can
// signal the whole group. Limitation: a descendant that deliberately leaves
// the group (setsid or setpgid) is not reached.
type pgroup struct{}

func newKillGroup(c *exec.Cmd) killGroup {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
	return pgroup{}
}

func (pgroup) attach(*os.Process) {}

func (pgroup) kill(p *os.Process) {
	// A negative pid signals the whole process group led by p.
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil {
		_ = p.Kill()
	}
}

func (pgroup) release() {}
