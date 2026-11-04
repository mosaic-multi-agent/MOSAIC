package preflight_test

// Tests that a run without an explicit subject model selection is rejected by
// preflight, before anything is deployed or spawned, and that a missing stub
// selection keeps defaulting to the subject selection.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/preflight"
)

// validateWithOverrides runs Validate on a passing suite, with a conforming
// catalog and a recording deployer, under the given overrides.
func validateWithOverrides(t *testing.T, overrides preflight.Overrides) (preflight.Plan, authoring.Report, *fakeDeployer) {
	t.Helper()
	root := writeTree(t, map[string]string{
		"suite.suite.yaml":   validSuiteWithOneTest,
		"subject.test.yaml":  definitionWithCatalogueKeyNoStubAgents,
		"subject.stubs.json": validRegistryForDeployTests,
	})
	deployer := &fakeDeployer{}
	plan, report := preflight.Validate(preflight.Input{
		SuitePath:     filepath.Join(root, "suite.suite.yaml"),
		FixtureRoot:   filepath.Join(root, "fixtures"),
		HarnessID:     "claude-code",
		CatalogFolder: catalogWith(t, nil),
		Deploy:        deployer,
		Overrides:     overrides,
	})
	return plan, report, deployer
}

// TestValidate_NoSubjectModel_RejectedWithExactlyOneDiagnostic asserts the
// rejection: one error, naming the CLI flag and the TUI screen, and saying
// the run never falls back to the harness default.
func TestValidate_NoSubjectModel_RejectedWithExactlyOneDiagnostic(t *testing.T) {
	_, report, _ := validateWithOverrides(t, preflight.Overrides{})

	got := diagnosticsWithCode(report, preflight.DiagMissingSubjectModel)
	if len(got) != 1 {
		t.Fatalf("got %d missing-subject-model diagnostics, want exactly 1: %v", len(got), report.Diagnostics)
	}
	if got[0].Severity != authoring.SeverityError {
		t.Errorf("severity = %v, want error", got[0].Severity)
	}
	msg := strings.ToLower(got[0].Message)
	for _, want := range []string{"--subject-model", "tui", "model screen", "default"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Message %q does not mention %q", got[0].Message, want)
		}
	}
	if !report.HasErrors() {
		t.Error("report has no errors, want the run blocked")
	}
}

// TestValidate_NoSubjectModel_StubSelectionDoesNotSubstitute asserts a stub
// selection alone is not a subject selection.
func TestValidate_NoSubjectModel_StubSelectionDoesNotSubstitute(t *testing.T) {
	_, report, _ := validateWithOverrides(t, preflight.Overrides{StubModel: "only-a-stub-model"})

	if got := diagnosticsWithCode(report, preflight.DiagMissingSubjectModel); len(got) != 1 {
		t.Errorf("got %d missing-subject-model diagnostics, want 1: %v", len(got), report.Diagnostics)
	}
}

// TestValidate_NoSubjectModel_NothingIsDeployed asserts the rejection comes
// before any deploy, so nothing is spent or written, and that the same cause
// does not also surface as a deploy diagnostic.
func TestValidate_NoSubjectModel_NothingIsDeployed(t *testing.T) {
	_, report, deployer := validateWithOverrides(t, preflight.Overrides{})

	if len(deployer.deployCalls) != 0 {
		t.Errorf("Deploy called %d time(s) without a subject model selection, want 0: %v",
			len(deployer.deployCalls), deployer.deployCalls)
	}
	for _, d := range report.Diagnostics {
		if d.Code == preflight.DiagUnresolvedTierModel || d.Code == "undeployable-subject-declaration" {
			t.Errorf("deploy diagnostic %q reported for a run that never selected a subject model: %v", d.Code, d)
		}
	}
}

// TestValidate_SubjectModelOnly_PassesAndStubDefaultsToSubject asserts an
// unselected stub model is legal and resolves to the subject selection.
func TestValidate_SubjectModelOnly_PassesAndStubDefaultsToSubject(t *testing.T) {
	plan, report, _ := validateWithOverrides(t, preflight.Overrides{SubjectModel: "chosen-subject"})

	if report.HasErrors() {
		t.Fatalf("a subject-only selection was rejected: %v", report.Diagnostics)
	}
	if len(plan.Tests) == 0 {
		t.Fatal("plan has no tests")
	}
	for i, rt := range plan.Tests {
		if rt.Models.Subject != "chosen-subject" {
			t.Errorf("plan.Tests[%d].Models.Subject = %q, want %q", i, rt.Models.Subject, "chosen-subject")
		}
		if got := rt.Models.EffectiveStub(); got != "chosen-subject" {
			t.Errorf("plan.Tests[%d].Models.EffectiveStub() = %q, want the subject selection %q", i, got, "chosen-subject")
		}
	}
}

// TestValidate_SubjectAndStubModel_BothAccepted asserts an explicit stub model
// is carried as selected.
func TestValidate_SubjectAndStubModel_BothAccepted(t *testing.T) {
	plan, report, _ := validateWithOverrides(t, preflight.Overrides{SubjectModel: "chosen-subject", StubModel: "chosen-stub"})

	if report.HasErrors() {
		t.Fatalf("a full selection was rejected: %v", report.Diagnostics)
	}
	for i, rt := range plan.Tests {
		if got := rt.Models.EffectiveStub(); got != "chosen-stub" {
			t.Errorf("plan.Tests[%d].Models.EffectiveStub() = %q, want %q", i, got, "chosen-stub")
		}
	}
}
