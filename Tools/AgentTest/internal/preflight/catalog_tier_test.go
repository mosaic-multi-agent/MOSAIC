package preflight_test

// Tests for the catalog tier vocabulary check: on every Validate, each agent
// file of the catalog being deployed from must declare a recommended_tier
// that is one of the AgentTest tiers. The incident this guards against is a
// catalog file spelled TEST_SUBJECT, which no deploy selection matches.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
)

// validateWithCatalog runs Validate on a passing suite with the given catalog
// folder as the process-wide default and a subject model selected.
func validateWithCatalog(t *testing.T, catalogFolder string) authoring.Report {
	t.Helper()
	suitePath, fixtureRoot := suiteTree(t)
	_, report := preflight.Validate(preflight.Input{
		SuitePath:     suitePath,
		FixtureRoot:   fixtureRoot,
		HarnessID:     "claude-code",
		CatalogFolder: catalogFolder,
		Overrides:     selectedModels(),
	})
	return report
}

// TestValidate_CatalogUsingOnlyTestTiers_Passes asserts a catalog whose every
// agent declares one of the AgentTest tiers yields no tier diagnostic and no
// error.
func TestValidate_CatalogUsingOnlyTestTiers_Passes(t *testing.T) {
	report := validateWithCatalog(t, catalogWith(t, nil))

	if report.HasErrors() {
		t.Fatalf("catalog with only AgentTest tiers was rejected: %v", report.Diagnostics)
	}
	if got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier); len(got) != 0 {
		t.Errorf("got invalid-catalog-tier diagnostics for a conforming catalog: %v", got)
	}
}

// TestValidate_OrchestratorScriptWithUnderscoreTier_RejectedNamingFileValueAndPermitted
// is the incident case: TEST_SUBJECT in orchestrator-script.md.
func TestValidate_OrchestratorScriptWithUnderscoreTier_RejectedNamingFileValueAndPermitted(t *testing.T) {
	catalog := catalogWith(t, map[string]string{
		"Orchestrator/orchestrator-script.md": agentFileWithTier("orchestrator-script", "TEST_SUBJECT"),
	})

	report := validateWithCatalog(t, catalog)

	got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier)
	if len(got) != 1 {
		t.Fatalf("got %d invalid-catalog-tier diagnostics, want exactly 1: %v", len(got), report.Diagnostics)
	}
	d := got[0]
	if d.Severity != authoring.SeverityError {
		t.Errorf("severity = %v, want error", d.Severity)
	}
	if want := filepath.Join(catalog, "Orchestrator", "orchestrator-script.md"); d.Path != want {
		t.Errorf("Path = %q, want %q", d.Path, want)
	}
	if d.Pointer != "recommended_tier" {
		t.Errorf("Pointer = %q, want recommended_tier", d.Pointer)
	}
	for _, want := range []string{"orchestrator-script.md", "TEST_SUBJECT", domain.TierTestSubject, domain.TierTestStub} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("Message %q does not mention %q", d.Message, want)
		}
	}
	if !report.HasErrors() {
		t.Error("report has no errors, want the run blocked")
	}
}

// TestValidate_CatalogAgentFileTiers_RejectedOutsideVocabulary covers every
// way an agent file can fail to declare a permitted tier, at each place the
// check looks.
func TestValidate_CatalogAgentFileTiers_RejectedOutsideVocabulary(t *testing.T) {
	cases := []struct {
		name      string
		rel       string
		content   string
		wantValue string
	}{
		{"orchestrator with production tier", "Orchestrator/orchestrator.md", agentFileWithTier("orchestrator", "HIGH"), "HIGH"},
		{"stub with lower-case tier", "Subagents/TestStubs/stub-researcher.md", agentFileWithTier("stub-researcher", "test-stub"), "test-stub"},
		{"stub nested deeply", "Subagents/TestStubs/Deep/stub-deep.md", agentFileWithTier("stub-deep", "TEST_STUB"), "TEST_STUB"},
		{"tier absent", "Subagents/TestStubs/stub-researcher.md", agentFileWithTier("stub-researcher", ""), "<missing>"},
		{"tier empty", "Subagents/TestStubs/stub-researcher.md", "---\nname: x\nrecommended_tier: \"\"\n---\n# x\n", ""},
		{"no frontmatter", "Subagents/TestStubs/stub-researcher.md", "# stub without frontmatter\n", "<missing>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catalog := catalogWith(t, map[string]string{tc.rel: tc.content})

			report := validateWithCatalog(t, catalog)

			got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier)
			if len(got) != 1 {
				t.Fatalf("got %d invalid-catalog-tier diagnostics, want 1: %v", len(got), report.Diagnostics)
			}
			if want := filepath.Join(catalog, filepath.FromSlash(tc.rel)); got[0].Path != want {
				t.Errorf("Path = %q, want %q", got[0].Path, want)
			}
			if tc.wantValue != "" && !strings.Contains(got[0].Message, tc.wantValue) {
				t.Errorf("Message %q does not name the value found %q", got[0].Message, tc.wantValue)
			}
			if !strings.Contains(got[0].Message, domain.TierTestSubject+", "+domain.TierTestStub) {
				t.Errorf("Message %q does not list the permitted values", got[0].Message)
			}
		})
	}
}

// TestValidate_CatalogWithSeveralBadFiles_OneDiagnosticEachOrderedByPath
// asserts every offending file is reported, not just the first.
func TestValidate_CatalogWithSeveralBadFiles_OneDiagnosticEachOrderedByPath(t *testing.T) {
	catalog := catalogWith(t, map[string]string{
		"Subagents/TestStubs/stub-researcher.md": agentFileWithTier("stub-researcher", "LOW"),
		"Orchestrator/orchestrator-script.md":    agentFileWithTier("orchestrator-script", "TEST_SUBJECT"),
		"Subagents/TestStubs/Deep/stub-deep.md":  agentFileWithTier("stub-deep", "MEDIUM"),
	})

	report := validateWithCatalog(t, catalog)

	got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier)
	if len(got) != 3 {
		t.Fatalf("got %d invalid-catalog-tier diagnostics, want 3: %v", len(got), got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Path > got[i].Path {
			t.Errorf("diagnostics not ordered by path: %q before %q", got[i-1].Path, got[i].Path)
		}
	}
}

// TestValidate_CatalogFilesOutsideCheckedLocations_AreIgnored asserts only
// agent markdown is checked: non-markdown files, markdown nested below
// Orchestrator/ and files outside Orchestrator/ and Subagents/ are not agents
// of the catalog.
func TestValidate_CatalogFilesOutsideCheckedLocations_AreIgnored(t *testing.T) {
	catalog := catalogWith(t, map[string]string{
		"Orchestrator/notes.txt":            "recommended_tier: HIGH\n",
		"Orchestrator/Archive/old-agent.md": agentFileWithTier("old", "HIGH"),
		"Subagents/TestStubs/readme.txt":    "recommended_tier: HIGH\n",
		"Workflows/Index.md":                "# no frontmatter\n",
		"CommunicationProtocol.md":          "# no frontmatter\n",
	})

	report := validateWithCatalog(t, catalog)

	if got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier); len(got) != 0 {
		t.Errorf("files outside Orchestrator/*.md and Subagents/**/*.md were checked: %v", got)
	}
}

// TestValidate_CatalogOverride_IsTheCatalogChecked asserts the check follows
// the catalog the run deploys from: the per-run override wins over the
// process-wide default.
func TestValidate_CatalogOverride_IsTheCatalogChecked(t *testing.T) {
	good := catalogWith(t, nil)
	bad := catalogWith(t, map[string]string{
		"Orchestrator/orchestrator-script.md": agentFileWithTier("orchestrator-script", "TEST_SUBJECT"),
	})
	suitePath, fixtureRoot := suiteTree(t)
	overrides := selectedModels()
	overrides.CatalogFolder = &bad

	_, report := preflight.Validate(preflight.Input{
		SuitePath:     suitePath,
		FixtureRoot:   fixtureRoot,
		HarnessID:     "claude-code",
		CatalogFolder: good,
		Overrides:     overrides,
	})

	if got := diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier); len(got) != 1 {
		t.Errorf("got %d invalid-catalog-tier diagnostics, want 1 from the overriding catalog: %v", len(got), report.Diagnostics)
	}
}

// TestValidate_CatalogFolderMissing_ReportsUnverifiable asserts a configured
// catalog that cannot be read is an error rather than a silent pass.
func TestValidate_CatalogFolderMissing_ReportsUnverifiable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-catalog")

	report := validateWithCatalog(t, missing)

	got := diagnosticsWithCode(report, preflight.DiagCatalogTierUnverifiable)
	if len(got) != 1 {
		t.Fatalf("got %d catalog-tier-unverifiable diagnostics, want 1: %v", len(got), report.Diagnostics)
	}
	if got[0].Severity != authoring.SeverityError {
		t.Errorf("severity = %v, want error", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, missing) {
		t.Errorf("Message %q does not name the folder %q", got[0].Message, missing)
	}
}

// TestValidate_EmptyCatalogFolderWithDeploy_ReportsUnverifiable asserts that a
// run which deploys from the product catalog (no folder given) cannot have its
// tiers checked and is refused.
func TestValidate_EmptyCatalogFolderWithDeploy_ReportsUnverifiable(t *testing.T) {
	suitePath, fixtureRoot := suiteTree(t)

	_, report := preflight.Validate(preflight.Input{
		SuitePath:   suitePath,
		FixtureRoot: fixtureRoot,
		HarnessID:   "claude-code",
		Deploy:      &fakeDeployer{},
		Overrides:   selectedModels(),
	})

	if got := diagnosticsWithCode(report, preflight.DiagCatalogTierUnverifiable); len(got) != 1 {
		t.Errorf("got %d catalog-tier-unverifiable diagnostics, want 1: %v", len(got), report.Diagnostics)
	}
}

// TestValidate_EmptyCatalogFolderWithoutDeploy_ChecksNothing asserts that when
// nothing is deployed there is nothing to verify.
func TestValidate_EmptyCatalogFolderWithoutDeploy_ChecksNothing(t *testing.T) {
	suitePath, fixtureRoot := suiteTree(t)

	_, report := preflight.Validate(preflight.Input{
		SuitePath:   suitePath,
		FixtureRoot: fixtureRoot,
		HarnessID:   "claude-code",
		Overrides:   selectedModels(),
	})

	for _, code := range []string{preflight.DiagCatalogTierUnverifiable, preflight.DiagInvalidCatalogTier} {
		if got := diagnosticsWithCode(report, code); len(got) != 0 {
			t.Errorf("unexpected %s diagnostics with no catalog and no deployer: %v", code, got)
		}
	}
}

// TestValidate_BadCatalogTier_StillReportedWhenSubjectModelMissing asserts the
// tier check does not depend on the selection check passing.
func TestValidate_BadCatalogTier_StillReportedWhenSubjectModelMissing(t *testing.T) {
	bad := catalogWith(t, map[string]string{
		"Orchestrator/orchestrator-script.md": agentFileWithTier("orchestrator-script", "TEST_SUBJECT"),
	})
	suitePath, fixtureRoot := suiteTree(t)

	_, report := preflight.Validate(preflight.Input{
		SuitePath:     suitePath,
		FixtureRoot:   fixtureRoot,
		HarnessID:     "claude-code",
		CatalogFolder: bad,
	})

	if len(diagnosticsWithCode(report, preflight.DiagInvalidCatalogTier)) != 1 {
		t.Errorf("invalid-catalog-tier missing when the subject model is also missing: %v", report.Diagnostics)
	}
	if len(diagnosticsWithCode(report, preflight.DiagMissingSubjectModel)) != 1 {
		t.Errorf("missing-subject-model missing alongside the tier error: %v", report.Diagnostics)
	}
}
