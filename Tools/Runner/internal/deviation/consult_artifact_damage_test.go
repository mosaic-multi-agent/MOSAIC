package deviation_test

// Tests for the artifact_damage member of the consultation wire request: always
// present on a routing request (null without damage), carrying the problems and
// the evidence paths with damage, never a damaged-copy path, and absent from
// the pre-consultation request.

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"mosaic-run/internal/domain"
)

const (
	damageLastGood = `C:\ws\RunnerLogs\20261106T135514Z-7c2e\Orchestration.last-good.md`
	damageLog      = `C:\ws\RunnerLogs\20261106T135514Z-7c2e\20261106T135514Z-7c2e-dispatch.log`
)

func damageProblems() []domain.ArtifactDamageProblem {
	return []domain.ArtifactDamageProblem{
		{Section: "ExecutionLog", Line: 101, Problem: "row has 8 cells, the header has 10"},
		{Section: "ExecutionLog", Line: 103, Problem: "non-table line before the closing tag"},
	}
}

func routingRequestWithDamage(report *domain.ArtifactDamageReport) domain.ConsultationRequest {
	return domain.ConsultationRequest{
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextRouting,
		ArtifactDamage:        report,
	}
}

// damageWire is the artifact_damage object as the orchestrator receives it.
// The path members are raw so null and absent can be told apart.
type damageWire struct {
	Problems []string `json:"problems"`
}

// sentDamageObject sends one routing consultation and returns the keys of the
// artifact_damage object together with the decoded problems.
func sentDamageObject(t *testing.T, report *domain.ArtifactDamageReport) (map[string]json.RawMessage, damageWire) {
	t.Helper()
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.ConsultRouting(context.Background(), routingRequestWithDamage(report)) //nolint:errcheck

	raw := wireKeys(t, fake.sent)
	member, present := raw["artifact_damage"]
	if !present {
		t.Fatal("want artifact_damage present on a routing request, got absent")
	}
	if string(member) == "null" {
		t.Fatal("want an artifact_damage object for a request with a damage report, got null")
	}
	var keys map[string]json.RawMessage
	mustUnmarshal(t, member, &keys)
	var w damageWire
	mustUnmarshal(t, member, &w)
	return keys, w
}

func TestOrchestratorConsultant_RoutingRequest_ArtifactDamageIsNullWithoutDamage(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.ConsultRouting(context.Background(), routingRequestWithDamage(nil)) //nolint:errcheck

	raw := wireKeys(t, fake.sent)
	got, present := raw["artifact_damage"]
	if !present {
		t.Fatal("want artifact_damage present (as null), got absent")
	}
	if string(got) != "null" {
		t.Errorf("want artifact_damage=null, got %s", got)
	}
}

func TestOrchestratorConsultant_RoutingRequest_CarriesExactlySevenMembersWithDamage(t *testing.T) {
	fake := &fakeRawInvoker{reply: routingStopReply("done")}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.ConsultRouting(context.Background(), routingRequestWithDamage(&domain.ArtifactDamageReport{ //nolint:errcheck
		Problems: damageProblems(), LastGoodCopy: damageLastGood, DispatchLog: damageLog,
	}))

	assertMemberSet(t, wireKeys(t, fake.sent),
		"orchestration_artifact", "context", "last_status", "last_status_message",
		"last_error_reason", "user_message", "artifact_damage")
}

func TestOrchestratorConsultant_RoutingRequest_DamageCarriesEveryProblemAsItsDescription(t *testing.T) {
	problems := damageProblems()

	_, w := sentDamageObject(t, &domain.ArtifactDamageReport{Problems: problems, LastGoodCopy: damageLastGood, DispatchLog: damageLog})

	if len(w.Problems) != len(problems) {
		t.Fatalf("want %d problems, got %v", len(problems), w.Problems)
	}
	for i, p := range problems {
		if w.Problems[i] != p.String() {
			t.Errorf("problem %d: want %q, got %q", i, p.String(), w.Problems[i])
		}
	}
}

func TestOrchestratorConsultant_RoutingRequest_DamageCarriesBothPathsVerbatim(t *testing.T) {
	keys, _ := sentDamageObject(t, &domain.ArtifactDamageReport{
		Problems: damageProblems(), LastGoodCopy: damageLastGood, DispatchLog: damageLog,
	})

	assertJSONString(t, keys, "last_good_copy", damageLastGood)
	assertJSONString(t, keys, "dispatch_log", damageLog)
}

func TestOrchestratorConsultant_RoutingRequest_DamageObjectHasOnlyProblemsAndTheTwoPaths(t *testing.T) {
	keys, _ := sentDamageObject(t, &domain.ArtifactDamageReport{
		Problems: damageProblems(), LastGoodCopy: damageLastGood, DispatchLog: damageLog,
	})

	assertMemberSet(t, keys, "problems", "last_good_copy", "dispatch_log")
	if _, present := keys["damaged_copy"]; present {
		t.Error("want no damaged_copy member: the damaged copy is for human diagnosis only")
	}
}

func TestOrchestratorConsultant_RoutingRequest_DamageWithoutLastGoodCopyCarriesNoLastGoodPath(t *testing.T) {
	keys, _ := sentDamageObject(t, &domain.ArtifactDamageReport{Problems: damageProblems(), DispatchLog: damageLog})

	assertJSONNull(t, keys, "last_good_copy")
	assertJSONString(t, keys, "dispatch_log", damageLog)
}

func TestOrchestratorConsultant_RoutingRequest_DamageWithoutDispatchLogCarriesNoLogPath(t *testing.T) {
	keys, _ := sentDamageObject(t, &domain.ArtifactDamageReport{Problems: damageProblems(), LastGoodCopy: damageLastGood})

	assertJSONNull(t, keys, "dispatch_log")
	assertJSONString(t, keys, "last_good_copy", damageLastGood)
}

func TestOrchestratorConsultant_RoutingRequest_DamageWithNoEvidenceCarriesNoPaths(t *testing.T) {
	keys, w := sentDamageObject(t, &domain.ArtifactDamageReport{Problems: damageProblems()})

	assertJSONNull(t, keys, "last_good_copy")
	assertJSONNull(t, keys, "dispatch_log")
	if len(w.Problems) != 2 {
		t.Errorf("want the 2 problems still sent, got %v", w.Problems)
	}
}

func TestOrchestratorConsultant_RoutingRequest_RetriesResendTheSameDamageReport(t *testing.T) {
	invoker := &sequencedRawInvoker{replies: []sequencedReply{
		malformedSchemaReply(),
		malformedSchemaReply(),
		{reply: routingStopReply("done")},
	}}
	c := newTestOrchestratorConsultant(invoker, mustParseTable(t))

	c.ConsultRouting(context.Background(), routingRequestWithDamage(&domain.ArtifactDamageReport{ //nolint:errcheck
		Problems: damageProblems(), LastGoodCopy: damageLastGood, DispatchLog: damageLog,
	}))

	if len(invoker.sent) < 2 {
		t.Fatalf("want the invalid reply retried, got %d call(s)", len(invoker.sent))
	}
	for i, payload := range invoker.sent {
		var w struct {
			ArtifactDamage *damageWire `json:"artifact_damage"`
		}
		mustUnmarshal(t, payload, &w)
		if w.ArtifactDamage == nil || len(w.ArtifactDamage.Problems) != 2 {
			t.Errorf("attempt %d: want the damage report resent with its 2 problems, got %+v", i+1, w.ArtifactDamage)
		}
	}
}

func TestOrchestratorConsultant_PreConsult_NeverCarriesArtifactDamageEvenWhenRequestHasOne(t *testing.T) {
	fake := &fakeRawInvoker{reply: []byte(`{}`)}
	c := newTestOrchestratorConsultant(fake, mustParseTable(t))

	c.PreConsult(context.Background(), domain.ConsultationRequest{ //nolint:errcheck
		OrchestrationArtifact: "Orchestration-abc/Orchestration.md",
		Context:               domain.ConsultContextPreConsultation,
		ArtifactDamage: &domain.ArtifactDamageReport{
			Problems: damageProblems(), LastGoodCopy: damageLastGood, DispatchLog: damageLog,
		},
	})

	assertMemberSet(t, wireKeys(t, fake.sent),
		"orchestration_artifact", "context", "last_status", "last_status_message",
		"last_error_reason", "user_message")
}

func TestOrchestratorConsultant_DispatchLogRecordsTheRoutingRequestAsSentWithTheDamageReport(t *testing.T) {
	invoker, logger := newOrderingPair(routingStopReply("done"), nil)
	c := newTestConsultantWithLogger(invoker, mustParseTable(t), logger)

	c.ConsultRouting(context.Background(), routingRequestWithDamage(&domain.ArtifactDamageReport{ //nolint:errcheck
		Problems: damageProblems(), LastGoodCopy: damageLastGood,
	}))

	if len(logger.requests) != 1 {
		t.Fatalf("want one logged request, got %d", len(logger.requests))
	}
	var w struct {
		ArtifactDamage *damageWire `json:"artifact_damage"`
	}
	mustUnmarshal(t, []byte(logger.requests[0].TaskDescription), &w)
	if w.ArtifactDamage == nil || len(w.ArtifactDamage.Problems) != 2 {
		t.Errorf("want the logged request to carry the damage report, got %+v", w.ArtifactDamage)
	}
}

// assertMemberSet fails unless keys holds exactly the wanted member names.
func assertMemberSet(t *testing.T, keys map[string]json.RawMessage, want ...string) {
	t.Helper()
	var got []string
	for k := range keys {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("want members %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want members %v, got %v", want, got)
		}
	}
}

func assertJSONString(t *testing.T, keys map[string]json.RawMessage, key, want string) {
	t.Helper()
	raw, present := keys[key]
	if !present {
		t.Errorf("want %q present, got absent", key)
		return
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || got != want {
		t.Errorf("want %q=%q, got %s", key, want, raw)
	}
}

func assertJSONNull(t *testing.T, keys map[string]json.RawMessage, key string) {
	t.Helper()
	raw, present := keys[key]
	if !present {
		t.Errorf("want %q present (as null), got absent", key)
		return
	}
	if string(raw) != "null" {
		t.Errorf("want %q=null, got %s", key, raw)
	}
}
