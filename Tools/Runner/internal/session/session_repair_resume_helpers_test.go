package session_test

// Fixtures for the repair-at-run-start tests: a resume of a run whose
// Orchestration.md is damaged when the session starts. The damaged artifact is
// the incident shape (83 Execution Log rows, row 77 split over three lines,
// rows 78 to 83 after it) in the linear workflow, read from the tool's own
// testdata. The last-good copy and the dispatch log are placed where the
// Runner keeps them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

const (
	incidentArtifactFixture = "artifacts/start_repair_incident.md"

	splitRowFirstLine  = "| 77 | agent-a#77 | PLANNING | - | 1 | SUCCESS | 2026-11-06T14:17:00Z | Tests written |"
	splitRowSecondLine = "| 83 | agent-a#83 SUCCESS ... |"
	strayContinuation  = " continuation of the split row: the note text the orchestrator inserted"
	wholeRow77         = splitRowFirstLine + " - | - |"
	noteAnchorLine     = "| 1 | Retry crash incident reproduction |"

	// The response the dispatch log holds for the run's last recorded step.
	lastStepResponse = `{"type":"response","agent_instance_id":"agent-a#83","response":{"agent_instance_id":"agent-a#83","status_code":"SUCCESS","status_message":"m"}}`
)

// incidentDamagedText returns the incident artifact as the orchestrator left it.
func incidentDamagedText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(orchFilePath(filepath.FromSlash(incidentArtifactFixture)))
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// repairIncident is the repair the orchestrator is asked for: row 77 whole
// again, the stray text moved to Workflow Notes, all 83 rows kept.
func repairIncident(t *testing.T, doc string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		switch lines[i] {
		case splitRowFirstLine:
			out = append(out, wholeRow77)
			i += 2 // the second row fragment and the stray continuation line
		case noteAnchorLine:
			out = append(out, lines[i], "| 2 | "+strings.TrimSpace(strayContinuation)+" |")
		default:
			out = append(out, lines[i])
		}
	}
	return strings.Join(out, "\n")
}

// dropRow77 repairs the incident by deleting row 77 instead of restoring it,
// which a repair must never do.
func dropRow77(t *testing.T, doc string) string {
	t.Helper()
	return strings.Replace(repairIncident(t, doc), wholeRow77+"\n", "", 1)
}

// startOpts says what evidence exists beside a damaged run.
type startOpts struct {
	noLastGood    bool
	noDispatchLog bool
	// edit changes the damaged text before it is written; nil keeps the incident.
	edit func(string) string
	// auto starts the run in auto mode with pre-consultation recorded as enabled.
	auto bool
}

// withDamagedRun turns the rig into a resume of the incident run. The artifact
// is written damaged, the last-good copy holds the repaired text and the
// dispatch log holds a response consistent with the last recorded row.
func withDamagedRun(t *testing.T, o startOpts) func(*repairRig) {
	return func(r *repairRig) {
		t.Helper()
		doc := incidentDamagedText(t)
		if o.auto {
			doc = replaceOnce(t, doc, "runner_mode: orchestrated", "runner_mode: auto")
			doc = replaceOnce(t, doc, "runner_pre_consultation: disabled", "runner_pre_consultation: enabled")
			r.cfg.RunSettings.Mode = domain.ExecutionModeAuto
			r.cfg.PreConsultation = true
		}
		if o.edit != nil {
			doc = o.edit(doc)
		}
		writeFile(t, r.run.artifactPath, doc)
		if !o.noLastGood {
			writeFile(t, filepath.Join(r.evidDir, "Orchestration.last-good.md"), repairIncident(t, incidentDamagedText(t)))
		}
		if !o.noDispatchLog {
			writeLogFile(t, dispatchlog.RunLogPath(filepath.Dir(r.run.runFolder), testRunID), logVersionLine, lastStepResponse)
		}
		r.cfg.IsNewRun = false
	}
}

// dispatchLogPathOf is where the Runner keeps the run's dispatch log.
func dispatchLogPathOf(r *repairRig) string {
	return dispatchlog.RunLogPath(filepath.Dir(r.run.runFolder), testRunID)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("test setup: %v", err)
	}
}

// requireStartTriggersNull asserts the request carries the resume values of the
// trigger fields: all null, no deviation.
func requireStartTriggersNull(t *testing.T, req domain.ConsultationRequest) {
	t.Helper()
	if req.LastStatus != nil || req.LastStatusMessage != nil || req.LastErrorReason != nil || req.UserMessage != nil {
		t.Errorf("want last_status, last_status_message, last_error_reason and user_message null, got %+v", triggerOf(req))
	}
	if req.Deviation != nil {
		t.Errorf("want no deviation on a start repair, got %+v", *req.Deviation)
	}
	if req.Context != domain.ConsultContextRouting {
		t.Errorf("want a routing consultation, got context %q", req.Context)
	}
}

// requireNoWriteOnDamaged asserts the artifact still holds the damaged incident
// text, byte for byte: nothing wrote to it.
func requireNoWriteOnDamaged(t *testing.T, rig *repairRig) {
	t.Helper()
	if got := readText(t, rig.run.artifactPath); got != incidentDamagedText(t) {
		t.Error("want the damaged artifact untouched by the Runner")
	}
}
