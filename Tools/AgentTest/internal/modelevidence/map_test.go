package modelevidence_test

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/modelevidence"
)

const testRunID = "20261024T100000Z-cc01"

func keys(pairs ...[2]string) *[]modelevidence.ModelKeyDoc {
	out := make([]modelevidence.ModelKeyDoc, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, modelevidence.ModelKeyDoc{Provider: p[0], Model: p[1]})
	}
	return &out
}

func reportOf(actor modelevidence.ActorDoc) modelevidence.ReportDoc {
	return modelevidence.ReportDoc{Runs: []modelevidence.RunDoc{{RunID: testRunID, Orchestrator: actor}}}
}

func newMapInput(form domain.ObservedModelForm, actor modelevidence.ActorDoc) modelevidence.MapInput {
	return modelevidence.MapInput{
		ExecutablePath: "mosaic-log-analyzer",
		RunID:          testRunID,
		Form:           form,
		Report:         reportOf(actor),
	}
}

// assertNoEvidence checks the invariant of every undeterminable case: the
// given reason, no model value, and a detail naming the run.
func assertNoEvidence(t *testing.T, got domain.ObservedModelEvidence, want domain.NoEvidenceReason) {
	t.Helper()
	if got.NoEvidence != want {
		t.Fatalf("NoEvidence = %q, want %q (detail %q)", got.NoEvidence, want, got.Detail)
	}
	if len(got.Models) != 0 {
		t.Errorf("Models = %v, want none when there is no evidence", got.Models)
	}
	if !strings.Contains(got.Detail, testRunID) {
		t.Errorf("Detail %q must name the run id %q", got.Detail, testRunID)
	}
}

func TestMap_ReportedModels(t *testing.T) {
	tests := []struct {
		name  string
		form  domain.ObservedModelForm
		keys  *[]modelevidence.ModelKeyDoc
		wants []string
	}{
		{"bare claude code model", domain.ObservedModelBare,
			keys([2]string{"anthropic", "claude-sonnet-5-5"}), []string{"claude-sonnet-5-5"}},
		{"bare ignores provider and de-duplicates", domain.ObservedModelBare,
			keys([2]string{"amazon-bedrock", "claude-opus-5-5"}, [2]string{"anthropic", "claude-opus-5-5"}),
			[]string{"claude-opus-5-5"}},
		{"bare accepts an empty provider", domain.ObservedModelBare,
			keys([2]string{"", "claude-sonnet-5-5"}), []string{"claude-sonnet-5-5"}},
		{"provider qualified joins with slash", domain.ObservedModelProviderQualified,
			keys([2]string{"anthropic", "claude-opus-5-5"}), []string{"anthropic/claude-opus-5-5"}},
		{"provider qualified keeps slashes inside the model", domain.ObservedModelProviderQualified,
			keys([2]string{"openrouter", "anthropic/claude-opus-5-5"}), []string{"openrouter/anthropic/claude-opus-5-5"}},
		{"several models are all reported sorted", domain.ObservedModelProviderQualified,
			keys([2]string{"openai", "gpt-5"}, [2]string{"anthropic", "claude-sonnet-5-5"}, [2]string{"anthropic", "claude-opus-5-5"}),
			[]string{"anthropic/claude-opus-5-5", "anthropic/claude-sonnet-5-5", "openai/gpt-5"}},
		{"provider qualified keeps the same model from different providers apart", domain.ObservedModelProviderQualified,
			keys([2]string{"openrouter", "claude-opus-5-5"}, [2]string{"anthropic", "claude-opus-5-5"}),
			[]string{"anthropic/claude-opus-5-5", "openrouter/claude-opus-5-5"}},
		{"provider qualified de-duplicates identical pairs", domain.ObservedModelProviderQualified,
			keys([2]string{"anthropic", "claude-opus-5-5"}, [2]string{"anthropic", "claude-opus-5-5"}),
			[]string{"anthropic/claude-opus-5-5"}},
		{"several bare models are all reported sorted", domain.ObservedModelBare,
			keys([2]string{"anthropic", "claude-sonnet-5-5"}, [2]string{"anthropic", "claude-opus-5-5"}),
			[]string{"claude-opus-5-5", "claude-sonnet-5-5"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := modelevidence.Map(newMapInput(tc.form, modelevidence.ActorDoc{ModelKeys: tc.keys}))

			if got.NoEvidence != "" {
				t.Fatalf("NoEvidence = %q (%s), want evidence", got.NoEvidence, got.Detail)
			}
			if !reflect.DeepEqual(got.Models, tc.wants) {
				t.Errorf("Models = %v, want %v", got.Models, tc.wants)
			}
		})
	}
}

// withInvokeErr and friends tweak a valid input into one failure case.
func withInvokeErr(in modelevidence.MapInput, exit int) modelevidence.MapInput {
	in.InvokeErr = errors.New("boom")
	in.ExitCode = exit
	return in
}

func withExit(in modelevidence.MapInput, exit int) modelevidence.MapInput {
	in.ExitCode = exit
	return in
}

func withParseErr(in modelevidence.MapInput) modelevidence.MapInput {
	in.ParseErr = errors.New("bad json")
	return in
}

func withUnattributable(in modelevidence.MapInput, actor modelevidence.ActorDoc) modelevidence.MapInput {
	in.Report.Unattributable = &modelevidence.RunDoc{Orchestrator: actor}
	return in
}

func TestMap_NoEvidenceReasons(t *testing.T) {
	good := keys([2]string{"anthropic", "claude-sonnet-5-5"})
	goodActor := modelevidence.ActorDoc{ModelKeys: good}
	valid := newMapInput(domain.ObservedModelBare, goodActor)
	otherRun := modelevidence.ReportDoc{Runs: []modelevidence.RunDoc{{RunID: "20261024T100000Z-zz99", Orchestrator: goodActor}}}
	twice := modelevidence.ReportDoc{Runs: []modelevidence.RunDoc{
		{RunID: testRunID, Orchestrator: goodActor}, {RunID: testRunID, Orchestrator: goodActor}}}

	tests := []struct {
		name string
		in   modelevidence.MapInput
		want domain.NoEvidenceReason
	}{
		{"unknown form", newMapInput("", goodActor), domain.NoEvidenceUnknownForm},
		{"unrecognised form", newMapInput("other", goodActor), domain.NoEvidenceUnknownForm},
		{"unknown form wins over invoke error", withInvokeErr(newMapInput("", goodActor), 0), domain.NoEvidenceUnknownForm},
		{"invoke error wins over exit code", withInvokeErr(valid, 2), domain.NoEvidenceToolUnavailable},
		{"exit 2 is no data", withExit(valid, 2), domain.NoEvidenceNoData},
		{"unparseable output", withParseErr(valid), domain.NoEvidenceUnparseable},
		{"run absent from report", modelevidence.MapInput{RunID: testRunID, Form: domain.ObservedModelBare, Report: otherRun},
			domain.NoEvidenceRunNotFound},
		{"report without runs", modelevidence.MapInput{RunID: testRunID, Form: domain.ObservedModelBare},
			domain.NoEvidenceRunNotFound},
		{"run listed twice", modelevidence.MapInput{RunID: testRunID, Form: domain.ObservedModelBare, Report: twice},
			domain.NoEvidenceAmbiguousRun},
		{"model keys absent from an older analyser",
			newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{Models: []string{"claude-sonnet-5-5"}}),
			domain.NoEvidenceUnsupportedOutput},
		{"usage in the unattributable bucket", withUnattributable(valid, goodActor), domain.NoEvidenceUnattributedUsage},
		{"model-less requests in the unattributable bucket",
			withUnattributable(valid, modelevidence.ActorDoc{ModelKeys: keys(), RequestsWithoutModel: 1}),
			domain.NoEvidenceUnattributedUsage},
		{"unattributed usage wins over unidentified requests",
			withUnattributable(newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: good, RequestsWithoutModel: 2}), goodActor),
			domain.NoEvidenceUnattributedUsage},
		{"some requests have no model even though others do",
			newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: good, RequestsWithoutModel: 1}),
			domain.NoEvidenceUnidentifiedRequests},
		{"no subject usage", newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys()}),
			domain.NoEvidenceNoSubjectUsage},
		{"models list is not a substitute for empty keys",
			newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{Models: []string{"claude-sonnet-5-5"}, ModelKeys: keys()}),
			domain.NoEvidenceNoSubjectUsage},
		{"empty provider when provider is required",
			newMapInput(domain.ObservedModelProviderQualified,
				modelevidence.ActorDoc{ModelKeys: keys([2]string{"anthropic", "claude-opus-5-5"}, [2]string{"", "gpt-5"})}),
			domain.NoEvidenceEmptyProvider},
		// Rule ordering: the first matching rule decides.
		{"tool failure wins over unparseable output", withParseErr(withExit(valid, 1)), domain.NoEvidenceToolFailed},
		{"no data wins over unparseable output", withParseErr(withExit(valid, 2)), domain.NoEvidenceNoData},
		{"run not found wins over unsupported output",
			modelevidence.MapInput{RunID: testRunID, Form: domain.ObservedModelBare, Report: modelevidence.ReportDoc{
				Runs: []modelevidence.RunDoc{{RunID: "20261024T100000Z-zz99", Orchestrator: modelevidence.ActorDoc{}}}}},
			domain.NoEvidenceRunNotFound},
		{"ambiguous run wins over unsupported output",
			modelevidence.MapInput{RunID: testRunID, Form: domain.ObservedModelBare, Report: modelevidence.ReportDoc{
				Runs: []modelevidence.RunDoc{{RunID: testRunID}, {RunID: testRunID}}}},
			domain.NoEvidenceAmbiguousRun},
		{"unsupported output wins over unattributed usage",
			withUnattributable(newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{}), goodActor),
			domain.NoEvidenceUnsupportedOutput},
		{"unidentified requests win over no subject usage",
			newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys(), RequestsWithoutModel: 3}),
			domain.NoEvidenceUnidentifiedRequests},
		{"unidentified requests win over empty provider",
			newMapInput(domain.ObservedModelProviderQualified,
				modelevidence.ActorDoc{ModelKeys: keys([2]string{"", "gpt-5"}), RequestsWithoutModel: 1}),
			domain.NoEvidenceUnidentifiedRequests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertNoEvidence(t, modelevidence.Map(tc.in), tc.want)
		})
	}
}

func TestMap_EmptyUnattributableBucketDoesNotBlockEvidence(t *testing.T) {
	in := withUnattributable(
		newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys([2]string{"anthropic", "claude-sonnet-5-5"})}),
		modelevidence.ActorDoc{ModelKeys: keys()})

	got := modelevidence.Map(in)

	if got.NoEvidence != "" || !reflect.DeepEqual(got.Models, []string{"claude-sonnet-5-5"}) {
		t.Errorf("got %+v, want the run's own model reported", got)
	}
}

func TestMap_UnattributableBucketWithoutModelKeysAndNoUsageDoesNotBlockEvidence(t *testing.T) {
	in := withUnattributable(
		newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys([2]string{"anthropic", "claude-sonnet-5-5"})}),
		modelevidence.ActorDoc{})

	got := modelevidence.Map(in)

	if got.NoEvidence != "" || !reflect.DeepEqual(got.Models, []string{"claude-sonnet-5-5"}) {
		t.Errorf("got %+v, want the run's own model reported", got)
	}
}

func TestMap_ToolFailedNamesExitCode(t *testing.T) {
	valid := newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys([2]string{"a", "m"})})
	for _, code := range []int{1, 3, 4, 7} {
		got := modelevidence.Map(withExit(valid, code))

		assertNoEvidence(t, got, domain.NoEvidenceToolFailed)
		if !strings.Contains(got.Detail, strconv.Itoa(code)) {
			t.Errorf("exit %d: Detail %q must name the exit code", code, got.Detail)
		}
	}
}

func TestMap_EmptyModelInKeysIsNeverReported(t *testing.T) {
	in := newMapInput(domain.ObservedModelBare, modelevidence.ActorDoc{ModelKeys: keys([2]string{"anthropic", ""})})

	got := modelevidence.Map(in)

	if got.NoEvidence == "" || len(got.Models) != 0 {
		t.Errorf("got %+v, want no evidence and no models for an empty model", got)
	}
}
