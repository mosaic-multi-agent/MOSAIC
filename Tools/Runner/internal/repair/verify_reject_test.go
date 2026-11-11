package repair_test

// Verify rejects a repair that is still damaged, changed or lost a row of the
// last-good copy, or contradicts the dispatch log. Every rejection carries
// problems that read as a fresh damage report.

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/repair"
)

func TestVerify_StillDamagedArtifact_IsRejectedWithTheStrictReadProblems(t *testing.T) {
	damaged := incidentDamaged(t)
	_, lastGood, log := incidentEvidence(t)
	_, strictErr := artifact.Parse([]byte(damaged))
	var damage *domain.ArtifactDamageError
	if !errors.As(strictErr, &damage) {
		t.Fatalf("want the strict read to report artifact damage, got %v", strictErr)
	}
	wantProblems := damage.Problems

	v := verifyDoc(t, damaged, lastGood, log)

	requireRejectedAt(t, v, "ExecutionLog", wantProblems[0].Line)
	if len(v.Problems) != len(wantProblems) {
		t.Errorf("want the %d problems of the strict read, got %v", len(wantProblems), v.Problems)
	}
	if len(v.State.ExecutionLog) != 0 {
		t.Errorf("a rejected verdict must not carry a state, got %d rows", len(v.State.ExecutionLog))
	}
}

func TestVerify_StillDamagedArtifact_IsRejectedWithoutAnyEvidence(t *testing.T) {
	v := verifyDoc(t, incidentDamaged(t), "", "")

	requireRejectedAt(t, v, "ExecutionLog", -1)
}

func TestVerify_ArtifactThatCannotBeRead_IsRejectedWithOneProblemWithoutSection(t *testing.T) {
	store := artifact.NewFileStore(filepath.Join(t.TempDir(), "Orchestration.md"))

	v := repair.Verify(repair.VerifyInput{Store: store})

	requireRejectedAt(t, v, "", 0)
	if len(v.Problems) != 1 {
		t.Errorf("want exactly one problem for a read failure that is not damage, got %v", v.Problems)
	}
}

func TestVerify_ChangedLastGoodRow_IsRejectedAtTheRepairedRowsLine(t *testing.T) {
	columns := []struct {
		name   string
		column int
		value  string
	}{
		{"agent", 1, "other-agent#40"},
		{"phase", 2, "PLANNING"},
		{"iteration", 3, "9.Other"},
		{"workflow row", 4, "9"},
		{"status", 5, "PARTIALLY_DONE"},
		{"timestamp", 6, "2030-01-01T00:00:00Z"},
		{"summary", 7, "rewritten by the orchestrator"},
		{"inputs", 8, "Plan.md"},
		{"checkpoint", 9, "abc123"},
	}
	repaired, lastGood, log := incidentEvidence(t)
	for _, c := range columns {
		t.Run(c.name, func(t *testing.T) {
			changed := withRowCell(t, repaired, 40, c.column, c.value)

			v := verifyDoc(t, changed, lastGood, log)

			requireRejectedAt(t, v, "ExecutionLog", rowLineNumber(t, changed, 40))
		})
	}
}

func TestVerify_MissingLastGoodRow_IsRejectedWithAProblemWithoutLine(t *testing.T) {
	repaired, lastGood, log := incidentEvidence(t)

	v := verifyDoc(t, withoutRow(t, repaired, 40), lastGood, log)

	requireRejectedAt(t, v, "ExecutionLog", 0)
}

func TestVerify_EveryChangedOrMissingRow_GetsItsOwnProblem(t *testing.T) {
	repaired, lastGood, log := incidentEvidence(t)
	doc := withRowCell(t, repaired, 10, 7, "changed ten")
	doc = withRowCell(t, doc, 20, 7, "changed twenty")
	doc = withoutRow(t, doc, 30)

	v := verifyDoc(t, doc, lastGood, log)

	requireRejectedAt(t, v, "ExecutionLog", -1)
	if len(v.Problems) != 3 {
		t.Errorf("want one problem per changed or missing row (3), got %v", v.Problems)
	}
}

func TestVerify_RowAddedAfterTheLastGoodCopy_IsNotAChange(t *testing.T) {
	repaired, _, log := incidentEvidence(t)
	lastGood := writeFile(t, "Orchestration.last-good.md", withUpTo(t, repaired, 50))

	v := verifyDoc(t, repaired, lastGood, log)

	requireAccepted(t, v)
}

func TestVerify_LastSeqBelowTheHighestLoggedStepResponse_IsRejected(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	lines := successResponses(t, rowAgents(repaired))
	lines = append(lines, responseLine(t, "implementation-tdd#84", domain.StatusSUCCESS, domain.ErrorNone))

	v := verifyDoc(t, repaired, lastGood, writeLog(t, lines...))

	requireRejectedAt(t, v, "ExecutionLog", -1)
}

func TestVerify_LastAgentWithoutItsResponseInALogThatReachesIt_IsRejected(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	cases := map[string][]string{
		"only a request for it": append(successResponses(t, agents[:82]), requestLine(t, "test-writer-tdd#83")),
		"a response of another agent under that seq": append(successResponses(t, agents[:82]),
			responseLine(t, "other-agent#83", domain.StatusSUCCESS, domain.ErrorNone)),
		"a harness error for it": append(successResponses(t, agents[:82]), harnessErrorLine(t, "test-writer-tdd#83")),
	}
	for name, lines := range cases {
		t.Run(name, func(t *testing.T) {
			v := verifyDoc(t, repaired, lastGood, writeLog(t, lines...))

			requireRejectedAt(t, v, "frontmatter", -1)
		})
	}
}

func TestVerify_LastStatusOrErrorCodeContradictingTheLog_IsRejected(t *testing.T) {
	repaired, lastGood, log := incidentEvidence(t)
	cases := map[string]string{
		"status":     withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED"),
		"error code": withLine(t, repaired, errorCodeLine, "  error_code: E401"),
		"both": withLine(t, withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED"),
			errorCodeLine, "  error_code: E501"),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			v := verifyDoc(t, doc, lastGood, log)

			requireRejectedAt(t, v, "frontmatter", -1)
		})
	}
}

func TestVerify_RecordedBlockedPairBelongingToAnotherAgent_IsRejected(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	lines := successResponses(t, agents[:81])
	lines = append(lines,
		// A different agent name carries the BLOCKED pair; the last agent never did.
		responseLine(t, "plan-review#81", domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING),
		responseLine(t, "test-writer-tdd#82", domain.StatusSUCCESS, domain.ErrorNone),
		responseLine(t, "test-writer-tdd#83", domain.StatusSUCCESS, domain.ErrorNone),
	)
	doc := withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED")
	doc = withLine(t, doc, errorCodeLine, "  error_code: E401")

	v := verifyDoc(t, doc, lastGood, writeLog(t, lines...))

	requireRejectedAt(t, v, "frontmatter", -1)
}

func TestVerify_ReadsTheRepairedArtifactThroughTheStore(t *testing.T) {
	repaired := incidentRepaired(t)
	store := &countingStore{ArtifactStore: artifact.NewFileStore(writeFile(t, "Orchestration.md", repaired))}

	repair.Verify(repair.VerifyInput{Store: store})

	if store.reads != 1 {
		t.Errorf("want the artifact read once through the store, got %d reads", store.reads)
	}
}

// countingStore counts Read calls on the wrapped store.
type countingStore struct {
	domain.ArtifactStore
	reads int
}

func (s *countingStore) Read(ctx context.Context) (domain.ArtifactState, error) {
	s.reads++
	return s.ArtifactStore.Read(ctx)
}
