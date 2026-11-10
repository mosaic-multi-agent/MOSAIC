package session_test

// Fixtures for the in-run HITL request tests: an approval reader that records
// what the HITL gate asked about, a rig over the three-step workflows, and
// readers of the HITL value every invocation carried.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

var threeAgents = []string{"agent-a", "agent-b", "agent-c"}

// approvalLog is an ApprovalReader that records every artifact the HITL gate
// reads, by file name, and answers from a per-read function. A nil answer
// function approves everything.
type approvalLog struct {
	mu     sync.Mutex
	reads  map[string]int
	answer func(file string, nth int) domain.HumanApproval
}

func (a *approvalLog) ReadApproval(_ context.Context, path string) domain.HumanApproval {
	file := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	a.mu.Lock()
	if a.reads == nil {
		a.reads = map[string]int{}
	}
	a.reads[file]++
	nth := a.reads[file]
	a.mu.Unlock()
	if a.answer == nil {
		return domain.ApprovalTrue
	}
	return a.answer(file, nth)
}

// readCount reports how often the gate read the named output file.
func (a *approvalLog) readCount(file string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reads[file]
}

// refuseFile answers false for every read of file and true for all others.
func refuseFile(file string) func(string, int) domain.HumanApproval {
	return func(f string, _ int) domain.HumanApproval {
		if f == file {
			return domain.ApprovalFalse
		}
		return domain.ApprovalTrue
	}
}

// newHITLRig builds an in-run rig over a three-step fixture (or any fixture
// whose agents are listed) with the approval reader wired as the HITL gate.
func newHITLRig(t *testing.T, fixture string, agents []string, approvals domain.ApprovalReader,
	consultant *scriptedRoutingConsultant, user *manualUser) *inRunRig {
	t.Helper()
	if user == nil {
		user = newManualUser(t)
	}
	return newInRunRig(t, user, inRunOptions{
		fixture: fixture, agents: agents, approvals: approvals, consultant: consultant,
	})
}

// requestAfter sets the given requests the first time agent finishes a harness
// call: "while the agent runs".
func (r *inRunRig) requestAfter(agent string, flags ...*requestFlag) {
	done := false
	r.afterInvoke = func(id string) {
		if id == agent && !done {
			done = true
			for _, f := range flags {
				f.request()
			}
		}
	}
}

// hitlSequence lists every harness invocation in order as "agent=hitl", with
// the HITL value the dispatched request carried.
func hitlSequence(f *harness.MockAdapter) []string {
	var out []string
	for _, inv := range f.Invocations() {
		out = append(out, fmt.Sprintf("%s=%t", inv.Agent.Identifier, inv.Request.HumanInTheLoop))
	}
	return out
}

// appliedFor returns the fields of the first session.request.applied entry
// written for the given request kind, or nil.
func (r *inRunRig) appliedFor(kind domain.InRunRequest) map[string]string {
	r.logger.mu.Lock()
	defer r.logger.mu.Unlock()
	for _, e := range r.logger.entries {
		if e.Event != domain.EventSessionRequestApplied {
			continue
		}
		out := map[string]string{}
		for _, f := range e.Fields {
			out[f.Key] = f.Value
		}
		if out["request"] == string(kind) {
			return out
		}
	}
	return nil
}

// dialogueWithHITL scripts a complete manual dialogue that picks the given row
// of the linear workflow and answers the HITL step with hitlOption.
func dialogueWithHITL(t *testing.T, row, task, hitlOption string) *manualUser {
	t.Helper()
	u := newManualUser(t).pickOne(domain.QuestionManualRow, row)
	u.say(domain.QuestionManualTask, task)
	u.acceptDefaultsPlus(domain.QuestionManualInputs)
	u.acceptDefaultsPlus(domain.QuestionManualOutputs)
	u.say(domain.QuestionManualConstraints, "")
	return u.pickOne(domain.QuestionManualHITL, hitlOption)
}

// startAuto runs the rig's session in the given mode over the "linear" workflow.
func (r *inRunRig) startAuto(mode domain.ExecutionMode) (domain.RunOutcome, error) {
	return r.ses.Start(context.Background(), configForMode(r.orchPath, mode))
}

