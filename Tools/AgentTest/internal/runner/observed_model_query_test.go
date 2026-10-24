package runner_test

// Tests that Run asks the observed-model port which model the subject session
// used, after the run and before teardown, and that the answer reaches the
// evidence and the result next to the selection. A missing port or a port
// error is never a model: it is evidence of nothing.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

// recordingObservedModels is a domain.ObservedModelProvider double that
// records each query in the shared call recorder and returns a scripted answer.
type recordingObservedModels struct {
	rec      *callRecorder
	evidence domain.ObservedModelEvidence
	err      error

	queries []domain.ObservedModelQuery
}

var _ domain.ObservedModelProvider = (*recordingObservedModels)(nil)

func (p *recordingObservedModels) ObservedSubjectModels(_ context.Context, q domain.ObservedModelQuery) (domain.ObservedModelEvidence, error) {
	p.rec.record("observed.Query")
	p.queries = append(p.queries, q)
	return p.evidence, p.err
}

// formReportingAdapter is the stub adapter plus the optional capability that
// declares how the harness's logs identify a model.
type formReportingAdapter struct {
	*stubAdapter
	form domain.ObservedModelForm
}

var _ domain.ObservedModelFormReporter = formReportingAdapter{}

func (a formReportingAdapter) ObservedModelForm() domain.ObservedModelForm { return a.form }

func observedRequest(id string) runner.Request {
	req := catalogueRequest(id)
	req.Test.Models = domain.ModelSelection{Subject: "model-a", Stub: "model-stub"}
	return req
}

func indexOfCall(calls []string, name string) int {
	for i, c := range calls {
		if c == name {
			return i
		}
	}
	return -1
}

func TestRun_QueriesObservedModelsOncePerRunWithTheRunsLogLocations(t *testing.T) {
	h := newHarness(t)
	port := &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{Models: []string{"model-a"}}}
	h.Deps.ObservedModels = port
	req := observedRequest("observed-query")

	_, err := runner.Run(context.Background(), h.Deps, req, nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(port.queries) != 1 {
		t.Fatalf("port queried %d times, want exactly once", len(port.queries))
	}
	q := port.queries[0]
	if q.RunID != req.Key.RunID {
		t.Errorf("query RunID = %q, want %q", q.RunID, req.Key.RunID)
	}
	costQuery := h.Cost.lastQuery
	if q.LogRoot == "" || q.LogRoot != costQuery.LogRoot {
		t.Errorf("query LogRoot = %q, want the run's log root %q", q.LogRoot, costQuery.LogRoot)
	}
	if q.LogsRoot != costQuery.LogsRoot {
		t.Errorf("query LogsRoot = %q, want %q", q.LogsRoot, costQuery.LogsRoot)
	}
}

func TestRun_ObservedModelQuery_HappensAfterTheRunAndBeforeTeardown(t *testing.T) {
	h := newHarness(t)
	h.Deps.ObservedModels = &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{Models: []string{"model-a"}}}

	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-order"), nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	calls := h.Rec.all()
	query, launch, teardown := indexOfCall(calls, "observed.Query"), indexOfCall(calls, "launcher.Launch"), indexOfCall(calls, "workspace.Teardown")
	if query < 0 {
		t.Fatalf("port was never queried (calls=%v)", calls)
	}
	if launch < 0 || query < launch {
		t.Errorf("port queried before the subject ran (calls=%v); the logs do not exist yet", calls)
	}
	if teardown < 0 || query > teardown {
		t.Errorf("port queried after teardown (calls=%v); the logs may be gone", calls)
	}
}

func TestRun_ObservedModelQuery_PassesTheHarnessModelForm(t *testing.T) {
	h := newHarness(t)
	port := &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{Models: []string{"p/model-a"}}}
	h.Deps.ObservedModels = port
	h.Deps.Adapter = formReportingAdapter{stubAdapter: h.Adapter, form: domain.ObservedModelProviderQualified}

	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-form"), nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(port.queries) != 1 || port.queries[0].Form != domain.ObservedModelProviderQualified {
		t.Errorf("queries = %+v, want one carrying form %q", port.queries, domain.ObservedModelProviderQualified)
	}
}

func TestRun_ObservedModelQuery_FormIsUnknownWhenTheAdapterDoesNotSayHowItLogsModels(t *testing.T) {
	h := newHarness(t)
	port := &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{Models: []string{"model-a"}}}
	h.Deps.ObservedModels = port

	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-form-unknown"), nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(port.queries) != 1 || port.queries[0].Form != "" {
		t.Errorf("queries = %+v, want one with an empty form (unknown)", port.queries)
	}
}

func TestRun_ObservedModelsReachEvidenceAlongsideSelection(t *testing.T) {
	h := newHarness(t)
	want := domain.ObservedModelEvidence{Models: []string{"model-b"}}
	h.Deps.ObservedModels = &recordingObservedModels{rec: h.Rec, evidence: want}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-evidence"), captureEvaluator(&ev))

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if len(ev.ObservedSubjectModels.Models) != 1 || ev.ObservedSubjectModels.Models[0] != "model-b" {
		t.Errorf("evidence ObservedSubjectModels = %+v, want %+v", ev.ObservedSubjectModels, want)
	}
	if ev.SubjectModel != "model-a" {
		t.Errorf("evidence SubjectModel = %q, want the selection %q unchanged", ev.SubjectModel, "model-a")
	}
}

func TestRun_NoEvidenceFromThePortReachesEvidenceUnchanged(t *testing.T) {
	h := newHarness(t)
	h.Deps.ObservedModels = &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{
		NoEvidence: domain.NoEvidenceRunNotFound, Detail: "run not in report",
	}}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-no-evidence"), captureEvaluator(&ev))

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	got := ev.ObservedSubjectModels
	if got.NoEvidence != domain.NoEvidenceRunNotFound || got.Detail != "run not in report" || len(got.Models) != 0 {
		t.Errorf("evidence ObservedSubjectModels = %+v, want the port's no-evidence answer unchanged", got)
	}
}

func TestRun_ObservedModelsReachTheResultWhenNoEvaluatorIsGiven(t *testing.T) {
	h := newHarness(t)
	h.Deps.ObservedModels = &recordingObservedModels{rec: h.Rec, evidence: domain.ObservedModelEvidence{Models: []string{"model-a", "model-b"}}}

	result, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-result"), nil)

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if got := strings.Join(result.ObservedSubjectModels, ","); got != "model-a,model-b" {
		t.Errorf("result ObservedSubjectModels = %q, want %q", got, "model-a,model-b")
	}
	if result.SubjectModel != "model-a" {
		t.Errorf("result SubjectModel = %q, want the selection %q", result.SubjectModel, "model-a")
	}
}

func TestRun_PortError_IsTreatedAsNoEvidence(t *testing.T) {
	h := newHarness(t)
	h.Deps.ObservedModels = &recordingObservedModels{
		rec: h.Rec,
		// A provider that returns a model together with an error must not
		// have that model believed.
		evidence: domain.ObservedModelEvidence{Models: []string{"model-a"}},
		err:      errors.New("analyzer exploded"),
	}

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-port-error"), captureEvaluator(&ev))

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v; a port error does not abort the run", err)
	}
	got := ev.ObservedSubjectModels
	if got.NoEvidence != domain.NoEvidencePortError {
		t.Errorf("NoEvidence = %q, want %q", got.NoEvidence, domain.NoEvidencePortError)
	}
	if len(got.Models) != 0 {
		t.Errorf("Models = %v, want none after a port error", got.Models)
	}
	if !strings.Contains(got.Detail, "analyzer exploded") {
		t.Errorf("Detail = %q, want it to carry the port's error text", got.Detail)
	}
}

func TestRun_MissingPort_IsNoEvidenceNeverAModel(t *testing.T) {
	h := newHarness(t)
	h.Deps.ObservedModels = nil

	var ev domain.RunEvidence
	_, err := runner.Run(context.Background(), h.Deps, observedRequest("observed-no-port"), captureEvaluator(&ev))

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	got := ev.ObservedSubjectModels
	if got.NoEvidence != domain.NoEvidencePortUnavailable {
		t.Errorf("NoEvidence = %q, want %q", got.NoEvidence, domain.NoEvidencePortUnavailable)
	}
	if len(got.Models) != 0 {
		t.Errorf("Models = %v, want none: the selection must never stand in for an observation", got.Models)
	}
	if got.Detail == "" {
		t.Errorf("Detail is empty, want an explanation")
	}
}
