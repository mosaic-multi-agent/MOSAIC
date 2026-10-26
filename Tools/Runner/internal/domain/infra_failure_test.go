package domain_test

// Tests of listing the infrastructure-agent failures recorded in an Execution
// Log: which rows are listed, in which order, and the line a failure renders as.

import (
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
)

var infraFailureEpoch = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

// infraFailureDeclared declares the two infrastructure agents of the fixtures.
func infraFailureDeclared() domain.InfraAgentSet {
	return domain.NewInfraAgentSet([]domain.DeclaredInfraAgent{
		{Name: "commit-manager-git", Class: "commit", OnFailure: "continue"},
		{Name: "checkpoint-manager-git", Class: "checkpoint", OnFailure: "halt"},
	})
}

func logEntry(seq int, agent string, status domain.StatusCode, code domain.ErrorCode, summary string) domain.ExecutionLogEntry {
	return domain.ExecutionLogEntry{
		Seq:       seq,
		Agent:     agent,
		Status:    status,
		ErrorCode: code,
		Summary:   summary,
		Timestamp: infraFailureEpoch.Add(time.Duration(seq) * time.Minute),
	}
}

func agentsOf(failures []domain.InfraFailure) []string {
	var out []string
	for _, f := range failures {
		out = append(out, f.AgentInstance)
	}
	return out
}

func requireAgents(t *testing.T, got []domain.InfraFailure, want ...string) {
	t.Helper()
	if g := agentsOf(got); strings.Join(g, ",") != strings.Join(want, ",") {
		t.Errorf("listed failures = %v, want %v", g, want)
	}
}

func TestListInfraFailures_ListsNonSuccessInfrastructureRowsInLogOrder(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		logEntry(1, "planner#1", domain.StatusSUCCESS, domain.ErrorNone, "planned"),
		logEntry(2, "commit-manager-git#2", domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "nothing to commit against"),
		logEntry(3, "builder#3", domain.StatusSUCCESS, domain.ErrorNone, "built"),
		logEntry(4, "checkpoint-manager-git#4", domain.StatusPARTIALLY_DONE, domain.ErrorNone, "half a checkpoint"),
		logEntry(5, "commit-manager-git#5", domain.StatusSUCCESS, domain.ErrorNone, "committed"),
	}

	got := domain.ListInfraFailures(log, infraFailureDeclared())

	requireAgents(t, got, "commit-manager-git#2", "checkpoint-manager-git#4")
	if len(got) == 2 {
		if got[0].Status != domain.StatusBLOCKED || got[0].ErrorCode != domain.ErrorINPUT_NOT_FOUND ||
			got[0].Summary != "nothing to commit against" || !got[0].Timestamp.Equal(log[1].Timestamp) {
			t.Errorf("first failure = %+v, want the recorded status, error code, summary and timestamp of row 2", got[0])
		}
		if got[1].Status != domain.StatusPARTIALLY_DONE || got[1].ErrorCode != domain.ErrorNone {
			t.Errorf("second failure = %+v, want PARTIALLY_DONE without an error code", got[1])
		}
	}
}

func TestListInfraFailures_WorkflowAgentFailuresAreNeverListed(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		logEntry(1, "builder#1", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "builder blocked"),
		logEntry(2, "checker#2", domain.StatusNEEDS_CLARIFICATION, domain.ErrorNone, "unclear"),
		logEntry(3, "commit-manager-git#3", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "git failed"),
	}

	got := domain.ListInfraFailures(log, infraFailureDeclared())

	requireAgents(t, got, "commit-manager-git#3")
}

func TestListInfraFailures_FailuresOfEarlierSessionsOfAResumedRunAreListed(t *testing.T) {
	// One log: the first session ended after the first failure; the resumed
	// session recorded the second one after a workflow step.
	log := []domain.ExecutionLogEntry{
		logEntry(1, "builder#1", domain.StatusSUCCESS, domain.ErrorNone, "built"),
		logEntry(2, "commit-manager-git#2", domain.StatusBLOCKED, domain.ErrorINPUT_NOT_FOUND, "first session failure"),
		logEntry(3, "checker#3", domain.StatusSUCCESS, domain.ErrorNone, "checked"),
		logEntry(4, "commit-manager-git#4", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE, "resumed session failure"),
	}

	got := domain.ListInfraFailures(log, infraFailureDeclared())

	requireAgents(t, got, "commit-manager-git#2", "commit-manager-git#4")
}

func TestListInfraFailures_AgentNameSharingAPrefixWithADeclaredAgentIsNotListed(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		logEntry(1, "commit-manager-git-extra#1", domain.StatusBLOCKED, domain.ErrorNone, "not the declared agent"),
		logEntry(2, "commit-manager#2", domain.StatusBLOCKED, domain.ErrorNone, "shorter name"),
		logEntry(3, "commit-manager-git#3", domain.StatusBLOCKED, domain.ErrorNone, "declared agent"),
	}

	got := domain.ListInfraFailures(log, infraFailureDeclared())

	requireAgents(t, got, "commit-manager-git#3")
}

func TestListInfraFailures_RejectedInfrastructureAttemptIsListedLikeAnyNonSuccessRow(t *testing.T) {
	// A HITL-rejected attempt of an infrastructure agent is recorded as a row of
	// that agent with a non-SUCCESS status, so it is a failure of the run.
	log := []domain.ExecutionLogEntry{
		logEntry(1, "checkpoint-manager-git#1", domain.StatusNEEDS_CLARIFICATION, domain.ErrorNone, "rejected by the user"),
	}

	got := domain.ListInfraFailures(log, infraFailureDeclared())

	requireAgents(t, got, "checkpoint-manager-git#1")
}

func TestListInfraFailures_NothingToList_ReturnsNil(t *testing.T) {
	cases := map[string][]domain.ExecutionLogEntry{
		"nil log": nil,
		"only successes": {
			logEntry(1, "commit-manager-git#1", domain.StatusSUCCESS, domain.ErrorNone, "committed"),
		},
		"only workflow failures": {
			logEntry(1, "builder#1", domain.StatusBLOCKED, domain.ErrorNone, "blocked"),
		},
	}
	for name, log := range cases {
		t.Run(name, func(t *testing.T) {
			if got := domain.ListInfraFailures(log, infraFailureDeclared()); got != nil {
				t.Errorf("ListInfraFailures = %+v, want nil", got)
			}
		})
	}
}

func TestListInfraFailures_NilInfraSet_ListsNothing(t *testing.T) {
	log := []domain.ExecutionLogEntry{
		logEntry(1, "commit-manager-git#1", domain.StatusBLOCKED, domain.ErrorNone, "failed"),
	}

	if got := domain.ListInfraFailures(log, nil); len(got) != 0 {
		t.Errorf("ListInfraFailures with a nil set = %+v, want nothing", got)
	}
}

func TestInfraFailure_String_ShowsInstanceStatusErrorCodeAndSummary(t *testing.T) {
	f := domain.InfraFailure{
		AgentInstance: "commit-manager-git#2",
		Status:        domain.StatusBLOCKED,
		ErrorCode:     domain.ErrorINPUT_NOT_FOUND,
		Summary:       "nothing to commit against",
	}

	if got, want := f.String(), "commit-manager-git#2  BLOCKED E101  nothing to commit against"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestInfraFailure_String_OmitsTheErrorCodePartWhenThereIsNone(t *testing.T) {
	f := domain.InfraFailure{
		AgentInstance: "checkpoint-manager-git#4",
		Status:        domain.StatusPARTIALLY_DONE,
		ErrorCode:     domain.ErrorNone,
		Summary:       "half a checkpoint",
	}

	if got, want := f.String(), "checkpoint-manager-git#4  PARTIALLY_DONE  half a checkpoint"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
