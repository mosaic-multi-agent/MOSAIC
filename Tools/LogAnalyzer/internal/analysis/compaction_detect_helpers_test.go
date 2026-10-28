package analysis_test

// Shared builders for the compaction detection and mismatch tests. They build
// on the context helpers: calls are per-call records with every category
// present, so a call's context equals the number passed in.

import (
	"strings"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// detPt is one call: its time in seconds from the run base and its context.
type detPt struct {
	sec int
	ctx int64
}

// detCalls builds per-call records with the given contexts.
func detCalls(prefix string, pts ...detPt) []domain.InvocationUsage {
	out := make([]domain.InvocationUsage, 0, len(pts))
	for i, p := range pts {
		out = append(out, ctxCall(prefix+string(rune('a'+i)), p.sec, p.ctx, 0, 0, 10))
	}
	return out
}

// detOutputOnlyCall is a per-call record whose context is absent.
func detOutputOnlyCall(recordID string, sec int, out int64) domain.InvocationUsage {
	return domain.InvocationUsage{
		Model:    ctxModel,
		Harness:  domain.HarnessClaudeCode,
		RecordID: recordID,
		At:       ctxAt(sec),
		Usage:    domain.TokenUsage{Output: domain.Tokens(out)},
	}
}

// detPriceAll prices an aggregate and returns the whole report and findings.
func detPriceAll(t *testing.T, agg domain.Aggregate, s domain.AnalysisSettings) (domain.Report, []domain.Finding) {
	t.Helper()
	table := domain.NewPricingTable([]domain.ModelPricing{flatModelPricing(t, ctxModel)})
	return analysis.Price(agg, table, s, testSource())
}

// detPrice prices a one-run spec with default settings.
func detPrice(t *testing.T, spec ctxRunSpec) (domain.RunReport, []domain.Finding) {
	t.Helper()
	return detPriceSettings(t, spec, domain.DefaultAnalysisSettings())
}

func detPriceSettings(t *testing.T, spec ctxRunSpec, s domain.AnalysisSettings) (domain.RunReport, []domain.Finding) {
	t.Helper()
	report, findings := detPriceAll(t, ctxAggregate(spec), s)
	if len(report.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(report.Runs))
	}
	return report.Runs[0], findings
}

// detDetected returns the detected marks of an actor.
func detDetected(a domain.ActorContext) []domain.CompactionMark {
	var out []domain.CompactionMark
	for _, m := range a.Compactions {
		if m.Kind == domain.CompactionDetected {
			out = append(out, m)
		}
	}
	return out
}

// detLogged is a single (unpaired) logged compaction of an actor.
func detLogged(actor domain.ActorRef, sec int) domain.Compaction {
	return domain.Compaction{Actor: actor, At: ctxAt(sec), Shape: domain.CompactionSingle}
}

// detLoggedPaired is a pre/post compaction spanning startSec..endSec.
func detLoggedPaired(actor domain.ActorRef, startSec, endSec int) domain.Compaction {
	return domain.Compaction{
		Actor: actor, At: ctxAt(startSec), EndedAt: ctxAt(endSec), Shape: domain.CompactionPaired,
	}
}

func detAgentActor(id domain.AgentInstanceID) domain.ActorRef {
	return domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: id, Type: "worker"}
}

func detAgent(id domain.AgentInstanceID, pts ...detPt) ctxAgent {
	return ctxAgent{id: id, typ: "worker", usage: detCalls(string(id)+"-", pts...)}
}

// detMismatches returns the compaction mismatch findings.
func detMismatches(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingCompactionMismatch {
			out = append(out, f)
		}
	}
	return out
}

// detRequireMismatches fails unless exactly want mismatch findings exist.
func detRequireMismatches(t *testing.T, findings []domain.Finding, want int) []domain.Finding {
	t.Helper()
	got := detMismatches(findings)
	if len(got) != want {
		t.Fatalf("compaction mismatch findings = %d, want %d: %+v", len(got), want, got)
	}
	return got
}

func detAssertRunActor(t *testing.T, f domain.Finding, actor domain.ActorRef) {
	t.Helper()
	if f.Run != domain.NamedRun(ctxRunID) {
		t.Errorf("finding run = %+v, want %s", f.Run, ctxRunID)
	}
	if f.Actor.Kind != actor.Kind || f.Actor.Instance != actor.Instance {
		t.Errorf("finding actor = %+v, want %+v", f.Actor, actor)
	}
}

func detAssertDetailContains(t *testing.T, f domain.Finding, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(f.Detail, p) {
			t.Errorf("finding detail %q does not contain %q", f.Detail, p)
		}
	}
}

func detAssertMark(t *testing.T, m domain.CompactionMark, before, after int, beforeTokens, afterTokens int64, at time.Time) {
	t.Helper()
	if m.Before != before || m.After != after {
		t.Errorf("mark positions = (%d, %d), want (%d, %d)", m.Before, m.After, before, after)
	}
	if m.BeforeTokens != beforeTokens || m.AfterTokens != afterTokens {
		t.Errorf("mark tokens = %d -> %d, want %d -> %d", m.BeforeTokens, m.AfterTokens, beforeTokens, afterTokens)
	}
	if !m.At.Equal(at) {
		t.Errorf("mark At = %v, want %v", m.At, at)
	}
}

// detRequireCounts fails unless the actor has exactly the wanted logged and
// detected compaction counts. Agreement tests use it to prove the drop was
// detected and the logged compaction attributed, so a zero-finding result is
// not reachable by an implementation that detects nothing.
func detRequireCounts(t *testing.T, a domain.ActorContext, logged, detected int) {
	t.Helper()
	if a.LoggedCount() != logged || a.DetectedCount() != detected {
		t.Fatalf("logged/detected = %d/%d, want %d/%d", a.LoggedCount(), a.DetectedCount(), logged, detected)
	}
}

// detAssertNoGenericWording fails when a finding uses either of the two
// generic mismatch wordings instead of the orchestrator-stream wording.
func detAssertNoGenericWording(t *testing.T, f domain.Finding) {
	t.Helper()
	for _, generic := range []string{"no logged compaction near it", "no context drop near it"} {
		if strings.Contains(f.Detail, generic) {
			t.Errorf("finding detail %q must use the orchestrator-stream wording, not %q", f.Detail, generic)
		}
	}
}
