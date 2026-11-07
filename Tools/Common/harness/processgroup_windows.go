//go:build windows

package harness

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modKernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW      = modKernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObj  = modKernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObj = modKernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject    = modKernel32.NewProc("TerminateJobObject")
	procOpenProcess           = modKernel32.NewProc("OpenProcess")
)

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

// jobBasicLimitInformation mirrors JOBOBJECT_BASIC_LIMIT_INFORMATION.
type jobBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

// jobExtendedLimitInformation mirrors JOBOBJECT_EXTENDED_LIMIT_INFORMATION.
type jobExtendedLimitInformation struct {
	BasicLimitInformation jobBasicLimitInformation
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// jobGroup is a Job Object with kill-on-close semantics. The started process
// is assigned to it right after Start; every process it spawns afterwards
// joins the job, so terminating the job ends the whole descendant set even
// when an intermediate parent has already exited.
type jobGroup struct {
	job syscall.Handle
}

func newKillGroup(_ *exec.Cmd) killGroup {
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return &jobGroup{}
	}
	g := &jobGroup{job: syscall.Handle(h)}
	if !g.setKillOnClose(true) {
		_ = syscall.CloseHandle(g.job)
		g.job = 0
	}
	return g
}

func (g *jobGroup) setKillOnClose(on bool) bool {
	var info jobExtendedLimitInformation
	if on {
		info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	}
	r, _, _ := procSetInformationJobObj.Call(
		uintptr(g.job),
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	return r != 0
}

// attach assigns the started process to the job. If the job could not be
// created or the assignment fails, the group degrades to the live-parent-chain
// tree kill.
func (g *jobGroup) attach(p *os.Process) {
	if g.job == 0 {
		return
	}
	ph, _, _ := procOpenProcess.Call(processSetQuota|processTerminate, 0, uintptr(p.Pid))
	if ph == 0 {
		g.discard()
		return
	}
	defer syscall.CloseHandle(syscall.Handle(ph))
	r, _, _ := procAssignProcessToJobObj.Call(uintptr(g.job), ph)
	if r == 0 {
		g.discard()
	}
}

func (g *jobGroup) discard() {
	_ = syscall.CloseHandle(g.job)
	g.job = 0
}

// kill terminates every process in the job and closes it. Without a job it
// falls back to the taskkill tree kill.
func (g *jobGroup) kill(p *os.Process) {
	if g.job == 0 {
		killProcessTree(p)
		return
	}
	_, _, _ = procTerminateJobObject.Call(uintptr(g.job), 1)
	g.discard()
}

// release frees the job after a normal exit. Kill-on-close is cleared first so
// a descendant that deliberately outlives the agent is left running, exactly
// as it was before jobs were used.
func (g *jobGroup) release() {
	if g.job == 0 {
		return
	}
	g.setKillOnClose(false)
	g.discard()
}
