package repair_test

// Verify accepts a repair that reads strictly, keeps every row of the last-good
// copy unchanged and does not contradict the dispatch log. Non-advancing log
// entries never make a correct repair fail.

import (
	"path/filepath"
	"testing"

	"mosaic-run/internal/domain"
)

// incidentEvidence is the incident's repaired artifact with a last-good copy
// holding rows 1..76 and a dispatch log with one SUCCESS response per row.
func incidentEvidence(t *testing.T) (repaired, lastGood, log string) {
	t.Helper()
	repaired = incidentRepaired(t)
	lastGood = writeFile(t, "Orchestration.last-good.md", withUpTo(t, repaired, 76))
	log = writeLog(t, append([]string{versionLine(t)}, successResponses(t, rowAgents(repaired))...)...)
	return repaired, lastGood, log
}

func TestVerify_IncidentRepairedTo83Rows_IsAcceptedWithTheRepairedState(t *testing.T) {
	repaired, lastGood, log := incidentEvidence(t)

	v := verifyDoc(t, repaired, lastGood, log)

	requireAccepted(t, v)
	if len(v.State.ExecutionLog) != 83 {
		t.Errorf("want the strictly read state with 83 rows, got %d", len(v.State.ExecutionLog))
	}
	if v.State.CurrentState.LastAgent != "test-writer-tdd#83" {
		t.Errorf("want last_agent test-writer-tdd#83 in the state, got %q", v.State.CurrentState.LastAgent)
	}
}

func TestVerify_RepairKeepingEveryLastGoodRowIsAcceptedWhenTheCopyHoldsAllRows(t *testing.T) {
	repaired := incidentRepaired(t)
	lastGood := writeFile(t, "Orchestration.last-good.md", repaired)

	v := verifyDoc(t, repaired, lastGood, "")

	requireAccepted(t, v)
}

func TestVerify_WithoutLastGoodCopy_RowLossIsNotJudgedButTheOtherChecksStillApply(t *testing.T) {
	repaired, _, log := incidentEvidence(t)
	missingRow := withoutRow(t, repaired, 40)

	accepted := verifyDoc(t, missingRow, "", log)
	rejected := verifyDoc(t, withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED"), "", log)

	requireAccepted(t, accepted)
	requireRejectedAt(t, rejected, "frontmatter", -1)
}

func TestVerify_UnusableLastGoodCopy_CountsAsAbsent(t *testing.T) {
	repaired := incidentRepaired(t)
	missingRow := withoutRow(t, repaired, 40)
	damagedCopy := writeFile(t, "Orchestration.last-good.md", incidentDamaged(t))
	absentCopy := filepath.Join(t.TempDir(), "never-written.md")

	for name, copyPath := range map[string]string{"damaged copy": damagedCopy, "absent copy": absentCopy} {
		t.Run(name, func(t *testing.T) {
			v := verifyDoc(t, missingRow, copyPath, "")

			requireAccepted(t, v)
		})
	}
}

func TestVerify_NonAdvancingLogEntries_DoNotMakeACorrectRepairFail(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	lines := []string{versionLine(t)}
	lines = append(lines, successResponses(t, agents[:82])...)
	lines = append(lines,
		// A consultation between steps: its id has three parts and its response
		// carries no step status.
		responseLine(t, "orchestrator#routing#3", domain.StatusSUCCESS, domain.ErrorNone),
		// A harness error and a request without response for a later dispatch.
		harnessErrorLine(t, "test-writer-tdd#83"),
		requestLine(t, "test-writer-tdd#83"),
		// The step that became row 83, answered after an interrupted attempt.
		responseLine(t, "test-writer-tdd#83", domain.StatusSUCCESS, domain.ErrorNone),
		requestLine(t, "implementation-tdd#84"),
		harnessErrorLine(t, "implementation-tdd#84"),
		responseLine(t, "orchestrator#routing#4", domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING),
		// Consultation counters are not step Seqs: these ones lie far above the
		// repaired log's last Seq and must neither count as a later step nor as
		// an attempt of the step named "orchestrator#routing" or "orchestrator#7".
		responseLine(t, "orchestrator#routing#200", domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING),
		responseLine(t, "orchestrator#7#300", domain.StatusSUCCESS, domain.ErrorNone),
	)

	v := verifyDoc(t, repaired, lastGood, writeLog(t, lines...))

	requireAccepted(t, v)
}

// Infrastructure and force-stopped steps get an Execution Log row and a logged
// response, yet they do not move current_state: last_agent stays the last
// workflow step. Verify must judge current_state against the workflow step, not
// against the highest-Seq response, and needs no list of infrastructure names.
func TestVerify_ResponsesOfStepsThatDoNotMoveCurrentState_DoNotMakeACorrectRepairFail(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	doc := withRowsAfter(t, repaired, 83,
		"| 84 | infra-agent#84 | EXECUTION | 1.Test | 2 | BLOCKED | 2026-11-06T14:18:00Z | Infrastructure step | - | - |",
		"| 85 | stopped-agent#85 | EXECUTION | 1.Test | 2 | PARTIALLY_DONE | 2026-11-06T14:19:00Z | Force-stopped step | - | - |",
	)
	lines := append([]string{versionLine(t)}, successResponses(t, agents)...)
	lines = append(lines,
		// Different status and error code than the recorded current_state pair.
		responseLine(t, "infra-agent#84", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
		responseLine(t, "stopped-agent#85", domain.StatusPARTIALLY_DONE, domain.ErrorNone),
	)
	log := writeLog(t, lines...)

	accepted := verifyDoc(t, doc, lastGood, log)
	// The recorded pair still has to come from the last_agent's own responses.
	rejected := verifyDoc(t, withLine(t, doc, lastStatusLine, "  last_status: PARTIALLY_DONE"), lastGood, log)

	requireAccepted(t, accepted)
	requireRejectedAt(t, rejected, "frontmatter", -1)
}

// Any attempt of the last_agent's name at or below its Seq is admitted, not only
// the latest: interrupted and HITL-rejected attempts cannot be told apart from
// the log, so the rule is deliberately lenient.
func TestVerify_RepeatedAttemptsForOneSeq_AreAllAdmitted(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	lines := append([]string{}, successResponses(t, agents[:82])...)
	lines = append(lines,
		responseLine(t, "test-writer-tdd#83", domain.StatusBLOCKED, domain.ErrorTOOL_UNAVAILABLE),
		responseLine(t, "test-writer-tdd#83", domain.StatusPARTIALLY_DONE, domain.ErrorNone),
		responseLine(t, "test-writer-tdd#83", domain.StatusSUCCESS, domain.ErrorNone),
	)
	// The repaired state records the pair of the FIRST attempt, not the latest.
	doc := withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED")
	doc = withLine(t, doc, errorCodeLine, "  error_code: E501")
	log := writeLog(t, lines...)

	accepted := verifyDoc(t, doc, lastGood, log)
	rejected := verifyDoc(t, withLine(t, repaired, lastStatusLine, "  last_status: NEEDS_CLARIFICATION"), lastGood, log)

	requireAccepted(t, accepted)
	requireRejectedAt(t, rejected, "frontmatter", -1)
}

func TestVerify_EmptyLastAgent_SkipsTheLogChecks(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	noAgent := withLine(t, repaired, lastAgentLine, "  last_agent: null")
	// The log reaches Seq 83 without a step response for it.
	lines := append(successResponses(t, rowAgents(repaired)[:82]), harnessErrorLine(t, "test-writer-tdd#83"))

	v := verifyDoc(t, noAgent, lastGood, writeLog(t, lines...))

	requireAccepted(t, v)
}

func TestVerify_LastAgentBeyondWhatTheLogCarries_IsNotJudgedAgainstTheLog(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	truncated := writeLog(t, successResponses(t, rowAgents(repaired)[:82])...)

	v := verifyDoc(t, repaired, lastGood, truncated)

	requireAccepted(t, v)
}

// The pair of any same-name response at or below last_agent's Seq is admitted
// (the HITL re-dispatch rule), not only that of the latest advancing one.
func TestVerify_StatusAndErrorCodeOfAnEarlierSameNameResponse_AreAdmitted(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	// test-writer-tdd#82 was blocked; the dispatch re-run as #83 recorded the
	// blocked pair in current_state (HITL re-dispatch of the same agent).
	lines := successResponses(t, agents[:81])
	lines = append(lines,
		responseLine(t, "test-writer-tdd#82", domain.StatusBLOCKED, domain.ErrorDEPENDENCY_MISSING),
		responseLine(t, "test-writer-tdd#83", domain.StatusSUCCESS, domain.ErrorNone),
	)
	doc := withLine(t, repaired, lastStatusLine, "  last_status: BLOCKED")
	doc = withLine(t, doc, errorCodeLine, "  error_code: E401")

	v := verifyDoc(t, doc, lastGood, writeLog(t, lines...))

	requireAccepted(t, v)
}

func TestVerify_LogSpanningSeveralRunnerSessions_IsReadAsOne(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	agents := rowAgents(repaired)
	lines := []string{versionLine(t)}
	lines = append(lines, successResponses(t, agents[:40])...)
	lines = append(lines, versionLine(t)) // a second Runner session appended its own header
	lines = append(lines, successResponses(t, agents[40:])...)

	v := verifyDoc(t, repaired, lastGood, writeLog(t, lines...))

	requireAccepted(t, v)
}

// Deciding that a startup-* fallback file is no usable log belongs to the
// caller that resolves the path; Verify skips the check for an empty path or a
// log it cannot read.
func TestVerify_LogCheckIsSkipped_ForAnEmptyPathAndAnUnreadableLog(t *testing.T) {
	repaired := incidentRepaired(t)
	lastGood := writeFile(t, "Orchestration.last-good.md", withUpTo(t, repaired, 76))
	logs := map[string]string{
		"empty path":     "",
		"unreadable log": filepath.Join(t.TempDir(), "absent-dispatch.log"),
	}

	for name, logPath := range logs {
		t.Run(name, func(t *testing.T) {
			v := verifyDoc(t, repaired, lastGood, logPath)

			requireAccepted(t, v)
		})
	}
}

func TestVerify_LogWithoutAnyStepResponse_IsNotJudgedAgainstTheArtifact(t *testing.T) {
	repaired, lastGood, _ := incidentEvidence(t)
	log := writeLog(t,
		versionLine(t),
		requestLine(t, "orchestrator#routing#1"),
		responseLine(t, "orchestrator#routing#1", domain.StatusSUCCESS, domain.ErrorNone),
		harnessErrorLine(t, "test-writer-tdd#83"),
	)

	v := verifyDoc(t, repaired, lastGood, log)

	requireAccepted(t, v)
}
