package session_test

// Row numbering of the anti-loop guard: the log event, the BLOCKED status
// message the consultant receives, and the stop message all name the same
// 1-based row as the dispatch-start event of that row.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/session"
)

// requestRecordingConsultant forwards to inner and keeps every request.
type requestRecordingConsultant struct {
	inner    domain.RoutingConsultant
	requests []domain.ConsultationRequest
}

func (c *requestRecordingConsultant) ConsultRouting(ctx context.Context, req domain.ConsultationRequest) (domain.RoutingInstruction, error) {
	c.requests = append(c.requests, req)
	return c.inner.ConsultRouting(ctx, req)
}

// guardMessages returns the status messages of the consultations that the
// anti-loop guard raised, whichever request field carried them.
func (c *requestRecordingConsultant) guardMessages() []string {
	var out []string
	for _, req := range c.requests {
		if req.Deviation != nil && strings.Contains(req.Deviation.Response.StatusMessage, "anti-loop guard") {
			out = append(out, req.Deviation.Response.StatusMessage)
		} else if req.LastStatusMessage != nil && strings.Contains(*req.LastStatusMessage, "anti-loop guard") {
			out = append(out, *req.LastStatusMessage)
		}
	}
	return out
}

// guardScenario is a run in which the consultant keeps requesting agent-b
// (row 2, index 1) until the guard blocks it and the run stops.
type guardScenario struct {
	logger     *sessionRecordingLogger
	consultant *requestRecordingConsultant
	outcome    domain.RunOutcome
}

func runGuardScenario(t *testing.T) *guardScenario {
	t.Helper()
	inner := &sameDispatchConsultant{agent: "agent-b", rowIndex: 1, safetyCap: consultantSafetyCap}
	consultant := &requestRecordingConsultant{inner: inner}
	logger := &sessionRecordingLogger{}
	ses, f, _, orchPath := newOrchestratedSessionWithDebug(t, consultant, logger)
	queueRepeated(f, "agent-b", 12, failingAgentEntry(domain.ErrorDEPENDENCY_MISSING))

	got, err := ses.Start(context.Background(), baseOrchestratedConfig(orchPath))

	requireGuardEscalationEnd(t, got, err, inner, 2)
	return &guardScenario{logger: logger, consultant: consultant, outcome: got}
}

func (g *guardScenario) triggeredEntries() []sessionLogEntry {
	var out []sessionLogEntry
	for _, e := range g.logger.entriesFor(domain.EventSessionDeviation) {
		if strings.Contains(e.Message, "anti-loop guard triggered") {
			out = append(out, e)
		}
	}
	return out
}

func TestRowNumbering_GuardTriggeredEvent_LogsOneBasedRow(t *testing.T) {
	g := runGuardScenario(t)

	triggered := g.triggeredEntries()
	if len(triggered) == 0 {
		t.Fatalf("want an anti-loop guard triggered entry, got events %v", g.logger.allEvents())
	}
	for _, e := range triggered {
		if row := fieldOf(e, "row"); row != "2" {
			t.Errorf("want row=2 (1-based), got row=%q", row)
		}
	}
}

func TestRowNumbering_GuardBlockedMessageToConsultant_NamesOneBasedRow(t *testing.T) {
	g := runGuardScenario(t)

	msgs := g.consultant.guardMessages()
	if len(msgs) == 0 {
		t.Fatal("scenario did not run as intended: the consultant received no guard status message")
	}
	for _, m := range msgs {
		if !strings.Contains(m, "for row 2;") {
			t.Errorf("want the guard status message to name row 2 (1-based), got %q", m)
		}
	}
}

// The same row shows the same number in the dispatch-start event, the guard
// event, the status message to the consultant and the stop message.
func TestRowNumbering_GuardEndToEnd_SameRowSameNumber(t *testing.T) {
	g := runGuardScenario(t)

	starts := g.logger.entriesFor(domain.EventSessionDispatchStart)
	requireRowForAgent(t, starts, "agent-b#", "2")
	requireRowForAgent(t, g.triggeredEntries(), "agent-b", "2")
	for _, m := range g.consultant.guardMessages() {
		if !strings.Contains(m, "row 2") || strings.Contains(m, "row 1;") {
			t.Errorf("want the guard status message to name row 2, got %q", m)
		}
	}
	if !strings.Contains(g.outcome.Message, "at row 2 ") {
		t.Errorf("want the guard stop message to name row 2, got %q", g.outcome.Message)
	}
}

// newLoggedBlockRig is newBlockRig with a Runner-log recorder wired in.
func newLoggedBlockRig(t *testing.T, logger domain.DebugLogger, agents ...string) *blockRig {
	t.Helper()
	parent := t.TempDir()
	orchPath := copyOrchestratorFile(t, parent, "audit-plan-block-orch.md")
	for _, a := range agents {
		writeAgentFile(t, parent, a)
	}
	runFolder := filepath.Join(parent, domain.RunScopedFolder(testRunID))
	if err := os.MkdirAll(runFolder, 0o700); err != nil {
		t.Fatalf("newLoggedBlockRig: create run folder: %v", err)
	}
	f := harness.NewMockAdapter()
	f.SetWriteRoot(parent)
	store := &memStore{}
	consult := &scriptedRoutingConsultant{}
	ses := session.New(session.Deps{
		Harness: f, Store: store, Routing: consult,
		Clock: fixedClock{t: epoch}, Interact: &noopInteraction{}, Debug: logger,
	})
	return &blockRig{ses: ses, adapter: f, store: store, consult: consult, orchPath: orchPath, runFolder: runFolder}
}
