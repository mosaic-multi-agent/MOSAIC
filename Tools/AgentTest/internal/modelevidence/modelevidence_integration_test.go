package modelevidence_test

// Integration coverage: the provider is run against the REAL mosaic-log-analyzer
// binary and tool-local fixture log trees (testdata/modelevidence/). Each tree
// holds subject usage in the orchestrator stream and stub usage in an agent
// folder, with different models, so reporting a stub model would fail.
//
// The analyser is built fresh for every test. If it cannot be built the test
// skips with an explicit reason.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/modelevidence"
)

func TestIntegration_RealAnalyser_ObservedSubjectModels(t *testing.T) {
	binPath := buildAnalyserBinary(t)
	tests := []struct {
		name   string
		tree   string
		runID  string
		form   domain.ObservedModelForm
		models []string
		reason domain.NoEvidenceReason
	}{
		{"claude code subject model, not the stub model", "claudecode", "20261024T100000Z-cc01",
			domain.ObservedModelBare, []string{"claude-sonnet-5-5"}, ""},
		{"claude code run with only stub usage has no subject usage", "claudecode", "20261024T100100Z-cc02",
			domain.ObservedModelBare, nil, domain.NoEvidenceNoSubjectUsage},
		{"opencode provider qualified subject model, not the stub model", "opencode", "20261024T110000Z-0c01",
			domain.ObservedModelProviderQualified, []string{"anthropic/claude-opus-5-5"}, ""},
		{"opencode subject using several models", "opencode", "20261024T110100Z-0c02",
			domain.ObservedModelProviderQualified,
			[]string{"anthropic/claude-opus-5-5", "anthropic/claude-sonnet-5-5"}, ""},
		{"usage in the unknown-run bucket is unattributed", "unattributed", "20261024T120000Z-a001",
			domain.ObservedModelProviderQualified, nil, domain.NoEvidenceUnattributedUsage},
		{"a request without a model is unidentified", "modelless", "20261024T130000Z-b001",
			domain.ObservedModelProviderQualified, nil, domain.NoEvidenceUnidentifiedRequests},
		{"an empty provider cannot be qualified", "emptyprovider", "20261024T140000Z-e001",
			domain.ObservedModelProviderQualified, nil, domain.NoEvidenceEmptyProvider},
		{"an empty provider is fine for the bare form", "emptyprovider", "20261024T140000Z-e001",
			domain.ObservedModelBare, []string{"claude-opus-5-5"}, ""},
		{"run missing from the tree", "opencode", "20261024T119999Z-ff99",
			domain.ObservedModelProviderQualified, nil, domain.NoEvidenceRunNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logsRoot := fixtureTree(t, tc.tree)
			p := modelevidence.New(modelevidence.Options{
				ExecutablePath: binPath,
				WorkingDir:     stageEmptyWorkDir(t),
				Timeout:        30 * time.Second,
			})

			got, err := p.ObservedSubjectModels(context.Background(), domain.ObservedModelQuery{
				LogRoot:  filepath.Join(logsRoot, tc.runID),
				LogsRoot: logsRoot,
				RunID:    tc.runID,
				Form:     tc.form,
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.NoEvidence != tc.reason {
				t.Fatalf("NoEvidence = %q (%s), want %q", got.NoEvidence, got.Detail, tc.reason)
			}
			if !reflect.DeepEqual(got.Models, tc.models) {
				t.Errorf("Models = %v, want %v", got.Models, tc.models)
			}
		})
	}
}

func TestIntegration_RealAnalyser_MissingLogsRootIsNoEvidence(t *testing.T) {
	binPath := buildAnalyserBinary(t)
	missing := filepath.Join(t.TempDir(), "absent")
	p := modelevidence.New(modelevidence.Options{
		ExecutablePath: binPath, WorkingDir: stageEmptyWorkDir(t), Timeout: 30 * time.Second})

	got, err := p.ObservedSubjectModels(context.Background(), domain.ObservedModelQuery{
		LogRoot: missing, LogsRoot: missing, RunID: testRunID, Form: domain.ObservedModelBare})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.NoEvidence == "" || len(got.Models) != 0 || got.Detail == "" {
		t.Errorf("got %+v, want no evidence with a detail and no models", got)
	}
}

func testFileDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve this file's own path")
	}
	return filepath.Dir(thisFile)
}

// buildAnalyserBinary builds mosaic-log-analyzer and returns its path, or
// skips with an explicit reason.
func buildAnalyserBinary(t *testing.T) string {
	t.Helper()
	// <repo>/Tools/AgentTest/internal/modelevidence -> <repo>/Tools
	toolsDir := filepath.Dir(filepath.Dir(filepath.Dir(testFileDir(t))))
	logAnalyzerDir := filepath.Join(toolsDir, "LogAnalyzer")
	if _, err := os.Stat(logAnalyzerDir); err != nil {
		t.Skipf("LogAnalyzer module not found at %s: %v", logAnalyzerDir, err)
	}
	binName := "mosaic-log-analyzer"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/mosaic-log-analyzer")
	cmd.Dir = logAnalyzerDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build mosaic-log-analyzer: %v\n%s", err, out)
	}
	return binPath
}

// fixtureTree returns the tool-local fixture logs root for one harness shape.
func fixtureTree(t *testing.T, tree string) string {
	t.Helper()
	root := filepath.Join(filepath.Dir(filepath.Dir(testFileDir(t))), "testdata", "modelevidence", tree)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixture tree not found at %s: %v", root, err)
	}
	return root
}

// stageEmptyWorkDir returns a test-owned working directory so the analyser
// never reads the repository pricing file.
func stageEmptyWorkDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
