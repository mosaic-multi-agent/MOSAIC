package modelevidence_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/modelevidence"
)

// invocation records what the provider asked the log-analysis tool to do.
type invocation struct {
	path       string
	args       []string
	workingDir string
}

// newRecordingInvoke returns a CommandRunner answering with the given outcome.
func newRecordingInvoke(rec *invocation, stdout string, exit int, err error) modelevidence.CommandRunner {
	return func(_ context.Context, path string, args []string, workingDir string) ([]byte, int, error) {
		*rec = invocation{path: path, args: args, workingDir: workingDir}
		return []byte(stdout), exit, err
	}
}

const ccReportJSON = `{"schema_version":"1","runs":[{"run_id":"20261024T100000Z-cc01",` +
	`"orchestrator":{"actor":"orchestrator","models":["claude-sonnet-5-5"],` +
	`"model_keys":[{"provider":"anthropic","model":"claude-sonnet-5-5"}],"requests_without_model":0},` +
	`"agents":[{"actor":"stub#1","models":["claude-haiku-5-5"],` +
	`"model_keys":[{"provider":"anthropic","model":"claude-haiku-5-5"}],"requests_without_model":0}]}],` +
	`"unattributable":null}`

const ocRunID = "20261024T110000Z-0c01"

const ocReportJSON = `{"schema_version":"1","runs":[{"run_id":"` + ocRunID + `",` +
	`"orchestrator":{"actor":"orchestrator","models":["claude-opus-5-5"],` +
	`"model_keys":[{"provider":"anthropic","model":"claude-opus-5-5"}],"requests_without_model":0},` +
	`"agents":[]}],"unattributable":null}`

func query(form domain.ObservedModelForm) domain.ObservedModelQuery {
	return domain.ObservedModelQuery{
		LogRoot:  "/logs/20261024T100000Z-cc01",
		LogsRoot: "/logs",
		RunID:    testRunID,
		Form:     form,
	}
}

func TestProvider_InvokesReportForTheRunOverTheLogsRoot(t *testing.T) {
	var rec invocation
	p := modelevidence.New(modelevidence.Options{
		ExecutablePath: "/bin/mosaic-log-analyzer",
		WorkingDir:     "/mosaic-root",
		Invoke:         newRecordingInvoke(&rec, ccReportJSON, 0, nil),
	})

	_, err := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantArgs := []string{"report", "--run", testRunID, "--path", "/logs", "--format", "json"}
	if !reflect.DeepEqual(rec.args, wantArgs) {
		t.Errorf("args = %v, want %v", rec.args, wantArgs)
	}
	if rec.path != "/bin/mosaic-log-analyzer" || rec.workingDir != "/mosaic-root" {
		t.Errorf("path/workingDir = %q/%q, want the configured values", rec.path, rec.workingDir)
	}
}

func TestProvider_FallsBackToLogRootWhenLogsRootEmpty(t *testing.T) {
	var rec invocation
	p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, ccReportJSON, 0, nil)})
	q := query(domain.ObservedModelBare)
	q.LogsRoot = ""

	_, _ = p.ObservedSubjectModels(context.Background(), q)

	wantArgs := []string{"report", "--run", testRunID, "--path", q.LogRoot, "--format", "json"}
	if !reflect.DeepEqual(rec.args, wantArgs) {
		t.Errorf("args = %v, want %v", rec.args, wantArgs)
	}
}

func TestProvider_ClaudeCodeReportYieldsSubjectBareModelNotStubModel(t *testing.T) {
	var rec invocation
	p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, ccReportJSON, 0, nil)})

	got, err := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

	if err != nil || got.NoEvidence != "" {
		t.Fatalf("got %+v err %v, want evidence", got, err)
	}
	if !reflect.DeepEqual(got.Models, []string{"claude-sonnet-5-5"}) {
		t.Errorf("Models = %v, want only the subject model", got.Models)
	}
}

func TestProvider_OpenCodeReportYieldsProviderQualifiedModel(t *testing.T) {
	var rec invocation
	p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, ocReportJSON, 0, nil)})

	q := query(domain.ObservedModelProviderQualified)
	q.RunID = ocRunID

	got, _ := p.ObservedSubjectModels(context.Background(), q)

	if !reflect.DeepEqual(got.Models, []string{"anthropic/claude-opus-5-5"}) {
		t.Errorf("Models = %v (%s), want provider/model", got.Models, got.Detail)
	}
}

func TestProvider_UnattributableUsageIsDecodedAndBlocksEvidence(t *testing.T) {
	withUsage := `{"runs":[{"run_id":"` + testRunID + `","orchestrator":{"model_keys":[{"provider":"anthropic","model":"claude-sonnet-5-5"}],"requests_without_model":0}}],` +
		`"unattributable":{"run_id":"unattributable","orchestrator":{"model_keys":[{"provider":"anthropic","model":"claude-sonnet-5-5"}],"requests_without_model":0}}}`
	var rec invocation
	p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, withUsage, 0, nil)})

	got, _ := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

	assertNoEvidence(t, got, domain.NoEvidenceUnattributedUsage)
}

func TestProvider_NonStartableExecutableIsToolUnavailable(t *testing.T) {
	p := modelevidence.New(modelevidence.Options{
		ExecutablePath: filepath.Join(t.TempDir(), "no-such-log-analyzer"),
		Timeout:        5 * time.Second,
	})

	got, err := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

	if err != nil {
		t.Fatalf("error = %v, want evidence", err)
	}
	assertNoEvidence(t, got, domain.NoEvidenceToolUnavailable)
}

func TestProvider_FailuresAreEvidenceNotErrors(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		exit   int
		err    error
		want   domain.NoEvidenceReason
	}{
		{"invocation error", "", 0, errors.New("cannot start"), domain.NoEvidenceToolUnavailable},
		{"no data exit", "", 2, nil, domain.NoEvidenceNoData},
		{"failure exit", "", 1, nil, domain.NoEvidenceToolFailed},
		{"garbage output", "not json", 0, nil, domain.NoEvidenceUnparseable},
		{"truncated json", `{"runs":[{"run_id":"` + testRunID + `","orchest`, 0, nil, domain.NoEvidenceUnparseable},
		{"wrong top-level type", `[{"run_id":"` + testRunID + `"}]`, 0, nil, domain.NoEvidenceUnparseable},
		{"empty output", "", 0, nil, domain.NoEvidenceUnparseable},
		{"older analyser without model keys",
			`{"runs":[{"run_id":"` + testRunID + `","orchestrator":{"models":["claude-sonnet-5-5"]}}]}`, 0, nil,
			domain.NoEvidenceUnsupportedOutput},
		{"other run only", `{"runs":[{"run_id":"20261024T100000Z-zz99","orchestrator":{"model_keys":[]}}]}`, 0, nil,
			domain.NoEvidenceRunNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rec invocation
			p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, tc.stdout, tc.exit, tc.err)})

			got, err := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

			if err != nil {
				t.Fatalf("error = %v, want failures reported as evidence", err)
			}
			assertNoEvidence(t, got, tc.want)
		})
	}
}

func TestProvider_UnknownFormIsNoEvidenceEvenWithValidReport(t *testing.T) {
	var rec invocation
	p := modelevidence.New(modelevidence.Options{Invoke: newRecordingInvoke(&rec, ccReportJSON, 0, nil)})

	got, _ := p.ObservedSubjectModels(context.Background(), query(""))

	assertNoEvidence(t, got, domain.NoEvidenceUnknownForm)
}

func TestProvider_TimeoutIsToolUnavailable(t *testing.T) {
	blocking := func(ctx context.Context, _ string, _ []string, _ string) ([]byte, int, error) {
		<-ctx.Done()
		return nil, 0, ctx.Err()
	}
	p := modelevidence.New(modelevidence.Options{Timeout: 20 * time.Millisecond, Invoke: blocking})

	got, err := p.ObservedSubjectModels(context.Background(), query(domain.ObservedModelBare))

	if err != nil {
		t.Fatalf("error = %v, want evidence", err)
	}
	assertNoEvidence(t, got, domain.NoEvidenceToolUnavailable)
}
