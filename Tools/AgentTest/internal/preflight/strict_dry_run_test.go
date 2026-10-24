package preflight_test

// Tests for the preflight dry-run deploy of a catalogue-path subject: it sends
// the tier-to-model map derived from the user's selection (stub falling back
// to subject), and a strict unresolved-model failure from the deployment tool
// is reported naming the agent and tier.

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"mosaic-agent-test/internal/agentdeploy"
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/preflight"
)

// TestValidate_DryRunDeploy_SendsTierMapOfTheSelection asserts the dry-run
// deploy receives exactly the two tiers mapped to the selected models.
func TestValidate_DryRunDeploy_SendsTierMapOfTheSelection(t *testing.T) {
	cases := []struct {
		name      string
		overrides preflight.Overrides
		want      map[string]string
	}{
		{
			"distinct subject and stub",
			preflight.Overrides{SubjectModel: "subj", StubModel: "stub"},
			map[string]string{domain.TierTestSubject: "subj", domain.TierTestStub: "stub"},
		},
		{
			"stub defaults to subject",
			preflight.Overrides{SubjectModel: "subj"},
			map[string]string{domain.TierTestSubject: "subj", domain.TierTestStub: "subj"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, deployer := validateWithOverrides(t, tc.overrides)

			if len(deployer.deployCalls) != 1 {
				t.Fatalf("Deploy called %d time(s), want 1 dry-run deploy", len(deployer.deployCalls))
			}
			call := deployer.deployCalls[0]
			if !call.DryRun {
				t.Error("preflight deploy was not a dry run")
			}
			if !reflect.DeepEqual(call.TierModels, tc.want) {
				t.Errorf("dry-run TierModels = %v, want %v", call.TierModels, tc.want)
			}
		})
	}
}

// TestValidate_DryRunDeploy_SendsTheSameMapAsTheSharedDerivation asserts the
// dry run and the setup deploy cannot drift: the map sent is the one the
// selection's single derivation yields.
func TestValidate_DryRunDeploy_SendsTheSameMapAsTheSharedDerivation(t *testing.T) {
	for _, sel := range []domain.ModelSelection{
		{Subject: "subj", Stub: "stub"},
		{Subject: "subj"},
	} {
		_, _, deployer := validateWithOverrides(t, preflight.Overrides{SubjectModel: sel.Subject, StubModel: sel.Stub})

		if len(deployer.deployCalls) != 1 {
			t.Fatalf("selection %+v: Deploy called %d time(s), want 1", sel, len(deployer.deployCalls))
		}
		if got, want := deployer.deployCalls[0].TierModels, sel.TierModels(); !reflect.DeepEqual(got, want) {
			t.Errorf("selection %+v: dry-run TierModels = %v, want ModelSelection.TierModels() = %v", sel, got, want)
		}
	}
}

// strictFailureDeployer returns a deployer whose Deploy fails the way the
// deployment tool does under strict model resolution.
func strictFailureDeployer(toolMessage string) *fakeDeployer {
	return &fakeDeployer{
		deployFn: func(context.Context, domain.DeployRequest) (domain.DeployResult, error) {
			return domain.DeployResult{}, &agentdeploy.DeployError{
				Reason:      agentdeploy.ReasonUnresolvedModels,
				Detail:      "orchestrator (tier TEST_SUBJECT)",
				ToolMessage: toolMessage,
				ExitCode:    1,
			}
		},
	}
}

// TestValidate_StrictDeployFailure_ReportsUnresolvedTierModelNamingAgentAndTier
// asserts a strict failure at the dry run is its own diagnostic carrying the
// tool's message, which names the agent and its tier.
func TestValidate_StrictDeployFailure_ReportsUnresolvedTierModelNamingAgentAndTier(t *testing.T) {
	const toolMessage = "strict models: 1 agent(s) have no resolved model: orchestrator (tier TEST_SUBJECT)"
	root := writeTree(t, map[string]string{
		"suite.suite.yaml":   validSuiteWithOneTest,
		"subject.test.yaml":  definitionWithCatalogueKeyNoStubAgents,
		"subject.stubs.json": validRegistryForDeployTests,
	})
	suitePath, fixtureRoot := root+"/suite.suite.yaml", root+"/fixtures"

	_, report := preflight.Validate(preflight.Input{
		SuitePath:     suitePath,
		FixtureRoot:   fixtureRoot,
		HarnessID:     "claude-code",
		CatalogFolder: catalogWith(t, nil),
		Deploy:        strictFailureDeployer(toolMessage),
		Overrides:     selectedModels(),
	})

	got := diagnosticsWithCode(report, preflight.DiagUnresolvedTierModel)
	if len(got) != 1 {
		t.Fatalf("got %d unresolved-tier-model diagnostics, want 1: %v", len(got), report.Diagnostics)
	}
	for _, want := range []string{"orchestrator", "TEST_SUBJECT"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("Message %q does not name %q", got[0].Message, want)
		}
	}
	if !report.HasErrors() {
		t.Error("report has no errors, want the run blocked")
	}
	if other := diagnosticsWithCode(report, "undeployable-subject-declaration"); len(other) != 0 {
		t.Errorf("strict failure also reported as undeployable-subject-declaration: %v", other)
	}
}

// TestValidate_OtherDeployFailure_KeepsUndeployableSubjectDeclaration asserts
// only the strict failure gets the new code: any other deploy failure keeps
// today's diagnostic.
func TestValidate_OtherDeployFailure_KeepsUndeployableSubjectDeclaration(t *testing.T) {
	root := writeTree(t, map[string]string{
		"suite.suite.yaml":   validSuiteWithOneTest,
		"subject.test.yaml":  definitionWithCatalogueKeyNoStubAgents,
		"subject.stubs.json": validRegistryForDeployTests,
	})
	deployer := &fakeDeployer{
		deployFn: func(context.Context, domain.DeployRequest) (domain.DeployResult, error) {
			return domain.DeployResult{}, &agentdeploy.DeployError{ToolMessage: "disk on fire", ExitCode: 1}
		},
	}

	_, report := preflight.Validate(preflight.Input{
		SuitePath:     root + "/suite.suite.yaml",
		FixtureRoot:   root + "/fixtures",
		HarnessID:     "claude-code",
		CatalogFolder: catalogWith(t, nil),
		Deploy:        deployer,
		Overrides:     selectedModels(),
	})

	if len(diagnosticsWithCode(report, "undeployable-subject-declaration")) != 1 {
		t.Errorf("want one undeployable-subject-declaration, got: %v", report.Diagnostics)
	}
	if len(diagnosticsWithCode(report, preflight.DiagUnresolvedTierModel)) != 0 {
		t.Errorf("unrelated deploy failure reported as unresolved-tier-model: %v", report.Diagnostics)
	}
}
