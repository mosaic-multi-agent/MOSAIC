package domain

import (
	"fmt"
	"time"
)

// InfraFailure is one infrastructure-agent failure recorded in a run's
// Execution Log, built from persisted data only.
type InfraFailure struct {
	AgentInstance string     // "{AgentName}#{Seq}" as recorded
	Status        StatusCode // never StatusSUCCESS
	ErrorCode     ErrorCode  // ErrorNone unless recorded
	Summary       string     // the recorded Summary (status message)
	Timestamp     time.Time
}

// String returns "{AgentInstance}  {Status}[ {ErrorCode}]  {Summary}"; the
// error code part is omitted when ErrorCode is ErrorNone.
func (f InfraFailure) String() string {
	if f.ErrorCode == ErrorNone {
		return fmt.Sprintf("%s  %s  %s", f.AgentInstance, f.Status, f.Summary)
	}
	return fmt.Sprintf("%s  %s %s  %s", f.AgentInstance, f.Status, f.ErrorCode, f.Summary)
}

// ListInfraFailures returns, in log order, one InfraFailure per Execution Log
// entry whose agent belongs to infra and whose Status is not SUCCESS.
// Entries of agents that infra does not contain (workflow agents, including
// HITL-rejected workflow attempts) are never listed. A nil infra lists nothing.
// Returns nil when there is nothing to list.
func ListInfraFailures(log []ExecutionLogEntry, infra InfraAgentSet) []InfraFailure {
	if infra == nil {
		return nil
	}
	var out []InfraFailure
	for _, e := range log {
		if e.Status == StatusSUCCESS || !infra.IsInfrastructureAgent(e.Agent) {
			continue
		}
		out = append(out, InfraFailure{
			AgentInstance: e.Agent,
			Status:        e.Status,
			ErrorCode:     e.ErrorCode,
			Summary:       e.Summary,
			Timestamp:     e.Timestamp,
		})
	}
	return out
}

// InfraFailureReportLines renders the end-of-run list of infrastructure
// failures: a header with the count, then one indented line per failure. It
// returns nil when there are no failures.
func InfraFailureReportLines(failures []InfraFailure) []string {
	if len(failures) == 0 {
		return nil
	}
	lines := make([]string, 0, len(failures)+1)
	lines = append(lines, fmt.Sprintf("Infrastructure failures in this run (%d):", len(failures)))
	for _, f := range failures {
		lines = append(lines, "  "+f.String())
	}
	return lines
}
