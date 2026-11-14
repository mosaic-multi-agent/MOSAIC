package analysis_test

// Tests for the run-model integrity checks: an instance holding another agent
// type's id, and an instance with no proper {type}#{N} id.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// imInvocation builds a run-model invocation with n usage records.
func imInvocation(id domain.AgentInstanceID, agentType string, n int) domain.Invocation {
	return domain.Invocation{
		Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: id, Type: agentType},
		State: domain.InvocationClosed,
		Usage: make([]domain.InvocationUsage, n),
	}
}

func imRun(invs ...domain.Invocation) domain.RunAggregate {
	return domain.RunAggregate{
		Run:      testRunRef,
		Activity: domain.RunActivity{Invocations: invs},
	}
}

// imOfKind returns the findings of one kind.
func imOfKind(fs []domain.Finding, kind domain.FindingKind) []domain.Finding {
	var out []domain.Finding
	for _, f := range fs {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestRunModelIntegrity_TypeDiffersFromIdType_OneMismatchFinding(t *testing.T) {
	inv := imInvocation("implementation-review#82", "test-writer-tdd", 2)

	got := analysis.RunModelIntegrityFindings(imRun(inv))

	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Kind != domain.FindingInstanceTypeMismatch {
		t.Errorf("kind = %q, want %q", f.Kind, domain.FindingInstanceTypeMismatch)
	}
	if f.Severity != domain.SeverityWarning {
		t.Errorf("severity = %v, want warning", f.Severity)
	}
	if f.Run != testRunRef {
		t.Errorf("run = %+v, want %+v", f.Run, testRunRef)
	}
	if f.Actor != inv.Actor {
		t.Errorf("actor = %+v, want %+v", f.Actor, inv.Actor)
	}
	if !strings.Contains(f.Detail, "test-writer-tdd") {
		t.Errorf("detail = %q, want it to name the agent type held", f.Detail)
	}
}

func TestRunModelIntegrity_TypeEqualsIdType_NoFinding(t *testing.T) {
	// A mismatching instance alongside proves the check ran and discriminated.
	run := imRun(
		imInvocation("implementation-review#82", "implementation-review", 1),
		imInvocation("a.b_c-d#7", "a.b_c-d", 1),
		imInvocation("planner#1", "other", 1),
	)

	got := analysis.RunModelIntegrityFindings(run)

	if len(got) != 1 || got[0].Actor.Instance != "planner#1" {
		t.Errorf("findings = %+v, want exactly one, for planner#1", got)
	}
}

func TestRunModelIntegrity_UnknownAgentType_NoMismatchFinding(t *testing.T) {
	run := imRun(
		imInvocation("implementation-review#82", "", 1),
		imInvocation("planner#1", "other", 1),
	)

	got := analysis.RunModelIntegrityFindings(run)

	if len(got) != 1 || got[0].Actor.Instance != "planner#1" {
		t.Errorf("findings = %+v, want exactly one, for planner#1", got)
	}
}

func TestRunModelIntegrity_IdWithoutTypeNumberForm_OneUnresolvedFindingStatingUsageCount(t *testing.T) {
	cases := []struct {
		name string
		id   domain.AgentInstanceID
		n    int
	}{
		{"unknown-agent", "unknown-agent", 13},
		{"logger fallback id", "claude_20261113T181048Z-ab12", 37},
		{"no usage records", "unknown-agent", 0},
		{"empty hash suffix", "agent#", 29},
		{"empty type", "#1", 29},
		{"trailing text after number", "agent#1x", 29},
		{"type starts with digit", "1agent#2", 29},
		{"type starts with hyphen", "-a#1", 29},
		{"type starts with dot", ".a#1", 29},
		{"type starts with underscore", "_a#1", 29},
		{"space in type", "my agent#1", 29},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := imInvocation(tc.id, "", tc.n)

			got := analysis.RunModelIntegrityFindings(imRun(inv))

			if len(got) != 1 {
				t.Fatalf("findings = %+v, want exactly one", got)
			}
			f := got[0]
			if f.Kind != domain.FindingUnresolvedInvocation {
				t.Errorf("kind = %q, want %q", f.Kind, domain.FindingUnresolvedInvocation)
			}
			if f.Severity != domain.SeverityWarning || f.Run != testRunRef || f.Actor != inv.Actor {
				t.Errorf("finding = %+v, want warning scoped to the run and the instance", f)
			}
			if !hasNumberToken(f.Detail, tc.n) {
				t.Errorf("detail = %q, want it to state the usage record count %d", f.Detail, tc.n)
			}
		})
	}
}

func TestRunModelIntegrity_UnresolvedDetail_StatesTheCountOfThatInstance(t *testing.T) {
	run := imRun(
		imInvocation("unknown-agent", "", 14),
		imInvocation("other-fallback_20261113T000000Z-ffff", "", 29),
	)

	got := analysis.RunModelIntegrityFindings(run)

	if len(got) != 2 {
		t.Fatalf("findings = %+v, want two", got)
	}
	for _, f := range got {
		want, other := 14, 29
		if f.Actor.Instance != "unknown-agent" {
			want, other = other, want
		}
		if !hasNumberToken(f.Detail, want) || hasNumberToken(f.Detail, other) {
			t.Errorf("detail for %s = %q, want count %d only", f.Actor.Instance, f.Detail, want)
		}
	}
}

// hasNumberToken reports whether text contains n as a whole number, not as part of a longer digit run.
func hasNumberToken(text string, n int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(n) + `([^0-9]|$)`).MatchString(text)
}

// imAgentsOnlyRun builds a run as a digest decodes it without an invocations
// section: agents carry usage records, Activity.Invocations is empty.
func imAgentsOnlyRun(actors ...domain.ActorAggregate) domain.RunAggregate {
	return domain.RunAggregate{Run: testRunRef, Agents: actors}
}

func imAgent(id domain.AgentInstanceID, agentType string, n int) domain.ActorAggregate {
	return domain.ActorAggregate{
		Actor:       domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: id, Type: agentType},
		Invocations: make([]domain.InvocationUsage, n),
	}
}

func TestRunModelIntegrity_AgentsOnlyRun_FindsMismatchAndUnresolvedWithUsageCount(t *testing.T) {
	run := imAgentsOnlyRun(
		imAgent("implementation-review#82", "test-writer-tdd", 2),
		imAgent("unknown-agent", "", 17),
		imAgent("planner#1", "planner", 3),
	)

	got := analysis.RunModelIntegrityFindings(run)

	mismatch := imOfKind(got, domain.FindingInstanceTypeMismatch)
	if len(mismatch) != 1 || mismatch[0].Actor.Instance != "implementation-review#82" {
		t.Errorf("instance_type_mismatch findings = %+v, want one for implementation-review#82", mismatch)
	}
	unresolved := imOfKind(got, domain.FindingUnresolvedInvocation)
	if len(unresolved) != 1 || unresolved[0].Actor.Instance != "unknown-agent" || !hasNumberToken(unresolved[0].Detail, 17) {
		t.Errorf("unresolved_invocation findings = %+v, want one for unknown-agent stating 17", unresolved)
	}
}

func TestRunModelIntegrity_InstanceInAgentsAndInvocations_OneFindingPerInstance(t *testing.T) {
	run := imRun(imInvocation("implementation-review#82", "test-writer-tdd", 1), imInvocation("unknown-agent", "", 1))
	run.Agents = []domain.ActorAggregate{
		imAgent("implementation-review#82", "test-writer-tdd", 1),
		imAgent("unknown-agent", "", 1),
	}

	got := analysis.RunModelIntegrityFindings(run)

	if len(got) != 2 {
		t.Errorf("findings = %+v, want exactly one per instance (two)", got)
	}
}

func TestRunModelIntegrity_SameInstanceInTwoInvocations_OneFindingPerInstance(t *testing.T) {
	run := imRun(
		imInvocation("implementation-review#82", "test-writer-tdd", 1),
		imInvocation("implementation-review#82", "test-writer-tdd", 1),
		imInvocation("unknown-agent", "", 1),
		imInvocation("unknown-agent", "", 1),
	)

	got := analysis.RunModelIntegrityFindings(run)

	if len(imOfKind(got, domain.FindingInstanceTypeMismatch)) != 1 || len(imOfKind(got, domain.FindingUnresolvedInvocation)) != 1 {
		t.Errorf("findings = %+v, want one mismatch and one unresolved", got)
	}
}

func TestRunModelIntegrity_UnresolvedIdWithKnownType_IsUnresolvedNotMismatch(t *testing.T) {
	got := analysis.RunModelIntegrityFindings(imRun(imInvocation("unknown-agent", "test-writer-tdd", 1)))

	if len(got) != 1 || got[0].Kind != domain.FindingUnresolvedInvocation {
		t.Errorf("findings = %+v, want exactly one unresolved_invocation", got)
	}
}

func TestRunModelIntegrity_ProperInstances_NoFindings(t *testing.T) {
	run := imRun(imInvocation("planner#1", "planner", 1), imInvocation("planner#2", "planner", 0))

	if got := analysis.RunModelIntegrityFindings(run); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestRunModelIntegrity_SeveralFindings_OrderedByInstanceId(t *testing.T) {
	run := imRun(
		imInvocation("a-first#1", "x", 1),
		imInvocation("m-middle#1", "m-middle", 1),
		imInvocation("unknown-agent", "", 1),
		imInvocation("z-last#1", "y", 1),
	)

	got := analysis.RunModelIntegrityFindings(run)

	var ids []domain.AgentInstanceID
	for _, f := range got {
		ids = append(ids, f.Actor.Instance)
	}
	want := []domain.AgentInstanceID{"a-first#1", "unknown-agent", "z-last#1"}
	if len(ids) != len(want) {
		t.Fatalf("instances = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("instances = %v, want %v", ids, want)
		}
	}
}

func TestRunModelIntegrity_Findings_AreNotDataLoss(t *testing.T) {
	run := imRun(imInvocation("a#1", "b", 1), imInvocation("unknown-agent", "", 1))

	got := analysis.RunModelIntegrityFindings(run)

	if len(got) != 2 {
		t.Fatalf("findings = %+v, want two", got)
	}
	if domain.NewQualitySummary(got).Incomplete() {
		t.Error("integrity findings must not make the report incomplete")
	}
}

func TestPrice_RunWithIntegrityProblems_RaisesTheRunModelFindingsForNamedRunsAndTheBucket(t *testing.T) {
	bucket := imRun(imInvocation("unknown-agent", "", 2))
	bucket.Run = domain.UnattributableRun()
	agg := domain.Aggregate{
		Runs:           []domain.RunAggregate{imRun(imInvocation("implementation-review#82", "test-writer-tdd", 1))},
		Unattributable: &bucket,
	}

	_, findings := analysis.Price(agg, domain.PricingTable{}, domain.DefaultAnalysisSettings(), testSource())

	mismatch := imOfKind(findings, domain.FindingInstanceTypeMismatch)
	if len(mismatch) != 1 || mismatch[0].Run != testRunRef {
		t.Errorf("instance_type_mismatch findings = %+v, want one for the named run", mismatch)
	}
	unresolved := imOfKind(findings, domain.FindingUnresolvedInvocation)
	if len(unresolved) != 1 || unresolved[0].Run != domain.UnattributableRun() {
		t.Errorf("unresolved_invocation findings = %+v, want one for the unattributable bucket", unresolved)
	}
}
