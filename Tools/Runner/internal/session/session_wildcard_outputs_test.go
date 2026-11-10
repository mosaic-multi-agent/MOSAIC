package session_test

// Session tests of written-output detection for wildcard Outputs when no
// output write detector is wired (the fail-closed fallback).

import (
	"context"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
)

// requireNotAsked asserts that the recorder was asked about no path with the
// given suffix.
func requireNotAsked(t *testing.T, approvals *blockApprovalRecorder, suffix string) {
	t.Helper()
	if approvals.asked(suffix) {
		t.Errorf("want no approval asked for %s, got paths %q", suffix, approvals.paths)
	}
}

// askedAnyWildcard reports whether the recorder was asked about a path that
// still contains '*'.
func askedAnyWildcard(approvals *blockApprovalRecorder) bool {
	for _, p := range approvals.paths {
		if strings.Contains(p, "*") {
			return true
		}
	}
	return false
}

// TestSession_Start_WildcardOutputWithoutDetector_ApprovalAskedForEveryExistingMatch
// verifies that, with no output write detector wired, the fallback reports for
// a 2.16 table's wildcard Output every existing file that matches it, those
// the row wrote and those already present, and none that does not match.
func TestSession_Start_WildcardOutputWithoutDetector_ApprovalAskedForEveryExistingMatch(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "wildcard-outputs-hitl-orch.md", approvals, "agent-a")
	rig.writeRunFile(t, "Report-0/notes.md", "# before the run\n")
	rig.writeRunFile(t, "Report-0/other.md", "# does not match\n")
	rig.queueOK("agent-a",
		runFileWrite("Report-2/notes.md", "# two\n"),
		runFileWrite("Report-1/notes.md", "# one\n"),
		runFileWrite("Elsewhere/notes.md", "# does not match\n"))

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-outputs-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAsked(t, approvals, "Report-0/notes.md", "Report-1/notes.md", "Report-2/notes.md")
	requireNotAsked(t, approvals, "Report-0/other.md")
	requireNotAsked(t, approvals, "Elsewhere/notes.md")
	if askedAnyWildcard(approvals) {
		t.Errorf("want no approval asked for a path still holding '*', got %q", approvals.paths)
	}
}

// TestSession_Start_WildcardOutputWithoutDetectorAndNoMatch_ReportsNoWildcardPath
// verifies that a wildcard Output matching no existing file reports nothing
// for that entry: the unexpanded declaration is not an artifact to approve.
// The concrete Output is still reported as declared.
func TestSession_Start_WildcardOutputWithoutDetectorAndNoMatch_ReportsNoWildcardPath(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "wildcard-outputs-hitl-orch.md", approvals, "agent-a")
	rig.queueOK("agent-a")

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-outputs-hitl", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	if askedAnyWildcard(approvals) {
		t.Errorf("want no approval asked for a path still holding '*', got %q", approvals.paths)
	}
	requireAsked(t, approvals, "summary.md")
}

// TestSession_Start_StageStarOutputWithoutDetector_ApprovalAskedForEveryExistingMatch
// verifies that a Stage-* Output is a plain wildcard in the fallback: every
// existing matching file is reported, whatever its folder name, with no
// schedule to take the IDs from.
func TestSession_Start_StageStarOutputWithoutDetector_ApprovalAskedForEveryExistingMatch(t *testing.T) {
	approvals := &blockApprovalRecorder{approval: domain.ApprovalTrue}
	rig := newBlockRig(t, "wildcard-stage-output-orch.md", approvals, "agent-a")
	rig.writeRunFile(t, "Stage-A/notes.md", "# before the run\n")
	rig.writeRunFile(t, "Stage-A/other.md", "# does not match\n")
	rig.writeRunFile(t, "Other/notes.md", "# does not match\n")
	rig.queueOK("agent-a",
		runFileWrite("Stage-b-2/notes.md", "# written\n"),
		runFileWrite("Stage-b-2/other.md", "# does not match\n"))

	got, err := rig.ses.Start(context.Background(), rig.config("wildcard-stage-output", domain.ExecutionModeAuto))

	requireRunStatus(t, got, err, domain.RunCompleted)
	requireAsked(t, approvals, "Stage-A/notes.md", "Stage-b-2/notes.md", "summary.md")
	requireNotAsked(t, approvals, "Stage-A/other.md")
	requireNotAsked(t, approvals, "Stage-b-2/other.md")
	requireNotAsked(t, approvals, "Other/notes.md")
	if askedAnyWildcard(approvals) {
		t.Errorf("want no approval asked for a path still holding '*', got %q", approvals.paths)
	}
}
