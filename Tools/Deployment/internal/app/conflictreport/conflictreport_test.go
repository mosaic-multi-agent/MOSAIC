package conflictreport_test

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app/conflictreport"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

func TestBuild_ShadowedAgentYieldsOneLineNamingWinnerAndShadowed(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "reviewer", "2.0.0")
	addTestSubagent(t, low, "reviewer", "1.0.0")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	if len(report.Lines) != 1 {
		t.Fatalf("expected 1 line, got %+v", report.Lines)
	}
	line := lineFor(t, report, agentItem("reviewer"))
	if line.Winner != identityOf("high") {
		t.Errorf("Winner = %+v, want high", line.Winner)
	}
	if !reflect.DeepEqual(line.Shadowed, []domain.CatalogIdentity{identityOf("low")}) {
		t.Errorf("Shadowed = %+v, want [low]", line.Shadowed)
	}
}

func TestBuild_ThreeLayersNameBothShadowedCatalogsHighestFirst(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, mid, low := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "mid"), newTestCatalogRoot(t, "low")
	for _, r := range []string{top, mid, low} {
		addTestSubagent(t, r, "reviewer", "1.0.0")
	}
	c := newTestStack(t, mosaic, top, mid, low)

	report := buildFor(c, nil, nil)

	line := lineFor(t, report, agentItem("reviewer"))
	if line.Winner != identityOf("top") {
		t.Errorf("Winner = %+v, want top", line.Winner)
	}
	want := []domain.CatalogIdentity{identityOf("mid"), identityOf("low")}
	if !reflect.DeepEqual(line.Shadowed, want) {
		t.Errorf("Shadowed = %+v, want %+v", line.Shadowed, want)
	}
	if len(report.Lines) != 1 {
		t.Errorf("expected one line for one shadowed item, got %d", len(report.Lines))
	}
}

func TestBuild_SeverityFollowsWinnerDeclaration(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	withTestOverrides(t, high, "high", "  agents: [declared]\n")
	for _, k := range []string{"declared", "undeclared"} {
		addTestSubagent(t, high, k, "2.0.0")
		addTestSubagent(t, low, k, "1.0.0")
	}
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	if got := lineFor(t, report, agentItem("declared")).Severity; got != domain.ConflictInfo {
		t.Errorf("declared override severity = %q, want info", got)
	}
	if got := lineFor(t, report, agentItem("undeclared")).Severity; got != domain.ConflictWarning {
		t.Errorf("undeclared override severity = %q, want warning", got)
	}
}

func TestBuild_SingleLayerStackYieldsEmptyReport(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	only := newTestCatalogRoot(t, "only")
	addTestSubagent(t, only, "reviewer", "1.0.0")
	addTestOrchestratorPair(t, only, "1.0.0")
	c := newTestStack(t, mosaic, only)

	report := buildFor(c, nil, nil)

	if len(report.Lines) != 0 {
		t.Errorf("expected no lines, got %+v", report.Lines)
	}
}

func TestBuild_NilShadowsYieldsEmptyReport(t *testing.T) {
	report := conflictreport.Build(conflictreport.Input{})

	if len(report.Lines) != 0 {
		t.Errorf("expected no lines, got %+v", report.Lines)
	}
}

func TestBuild_OrchestratorPairIsExactlyOneLineAndNoAgentLines(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestratorPair(t, high, "2.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	if len(report.Lines) != 1 {
		t.Fatalf("expected exactly one line for the pair, got %+v", report.Lines)
	}
	line := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair})
	if line.Winner != identityOf("high") || !reflect.DeepEqual(line.Shadowed, []domain.CatalogIdentity{identityOf("low")}) {
		t.Errorf("pair line = %+v, want high over low", line)
	}
	for _, l := range report.Lines {
		if l.Item.Kind == domain.CatalogItemAgent {
			t.Errorf("unexpected agent line %+v", l)
		}
	}
}

func TestBuild_OrchestratorPairSeverityFollowsPairDeclaration(t *testing.T) {
	cases := []struct {
		name     string
		declared bool
		want     domain.ConflictSeverity
	}{
		{"declared pair is info", true, domain.ConflictInfo},
		{"undeclared pair is warning", false, domain.ConflictWarning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mosaic := newTestMosaicRoot(t)
			high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
			if tc.declared {
				withTestOverrides(t, high, "high", "  orchestrator_pair: true\n")
			}
			addTestOrchestratorPair(t, high, "2.0.0")
			addTestOrchestratorPair(t, low, "1.0.0")
			c := newTestStack(t, mosaic, high, low)

			report := buildFor(c, nil, nil)

			line := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair})
			if line.Severity != tc.want {
				t.Errorf("severity = %q, want %q", line.Severity, tc.want)
			}
		})
	}
}

func TestBuild_SystemFileShadowingIsReported(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	mustWrite(t, high, catalogpaths.RelProtocolFile, "# protocol high\n")
	mustWrite(t, low, catalogpaths.RelProtocolFile, "# protocol low\n")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	line := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemSystemFile, Key: catalogpaths.RelProtocolFile})
	if line.Winner != identityOf("high") || !reflect.DeepEqual(line.Shadowed, []domain.CatalogIdentity{identityOf("low")}) {
		t.Errorf("system-file line = %+v, want high over low", line)
	}
	if line.Severity != domain.ConflictWarning {
		t.Errorf("severity = %q, want warning", line.Severity)
	}
}

func TestBuild_HarnessInjectionShadowingIsReportedWithDeclaredSeverity(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	claude := catalogpaths.HarnessDirNameClaudeCode
	withTestOverrides(t, high, "high", "  harness_injections: [\""+claude+"\"]\n")
	mustWrite(t, high, "HarnessInjections/"+claude+"/HarnessInjections.md", "# h\n")
	mustWrite(t, low, "HarnessInjections/"+claude+"/HarnessInjections.md", "# l\n")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	line := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: claude})
	if line.Winner != identityOf("high") || line.Severity != domain.ConflictInfo {
		t.Errorf("injections line = %+v, want high winner and info", line)
	}
}

func TestBuild_AffectedWorkflowsComeFromSelectionAndDeployedSetIncludingLowerLayers(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "reviewer", "2.0.0")
	addTestSubagent(t, low, "reviewer", "1.0.0")
	addTestWorkflow(t, high, "selected-flow", "reviewer")
	addTestWorkflow(t, low, "deployed-lower-flow", "reviewer")
	addTestWorkflow(t, high, "unrelated-flow", "other-agent")
	addTestWorkflow(t, high, "not-deployed-flow", "reviewer")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, []string{"selected-flow", "unrelated-flow"}, []string{"deployed-lower-flow"})

	got := lineFor(t, report, agentItem("reviewer")).AffectedWorkflows
	want := []string{"deployed-lower-flow", "selected-flow"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AffectedWorkflows = %s, want %s", joined(got), joined(want))
	}
}

func TestBuild_EmptyDeployedSetUsesSelectionOnly(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "reviewer", "2.0.0")
	addTestSubagent(t, low, "reviewer", "1.0.0")
	addTestWorkflow(t, high, "selected-flow", "reviewer")
	addTestWorkflow(t, high, "other-flow", "reviewer")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, []string{"selected-flow"}, nil)

	got := lineFor(t, report, agentItem("reviewer")).AffectedWorkflows
	if !reflect.DeepEqual(got, []string{"selected-flow"}) {
		t.Errorf("AffectedWorkflows = %s, want selected-flow", joined(got))
	}
}

func TestBuild_DeployedIdUnknownToCatalogIsIgnored(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "reviewer", "2.0.0")
	addTestSubagent(t, low, "reviewer", "1.0.0")
	addTestWorkflow(t, high, "known-flow", "reviewer")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, []string{"known-flow", "ghost-flow"})

	got := lineFor(t, report, agentItem("reviewer")).AffectedWorkflows
	if !reflect.DeepEqual(got, []string{"known-flow"}) {
		t.Errorf("AffectedWorkflows = %s, want known-flow only", joined(got))
	}
}

func TestBuild_AffectedWorkflowsAreUniqueAndSorted(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "reviewer", "2.0.0")
	addTestSubagent(t, low, "reviewer", "1.0.0")
	for _, id := range []string{"b-flow", "a-flow", "c-flow"} {
		addTestWorkflow(t, high, id, "reviewer")
	}
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, []string{"c-flow", "a-flow"}, []string{"b-flow", "a-flow"})

	got := lineFor(t, report, agentItem("reviewer")).AffectedWorkflows
	want := []string{"a-flow", "b-flow", "c-flow"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AffectedWorkflows = %s, want %s", joined(got), joined(want))
	}
}

func TestBuild_OrchestratorPairLineNamesAllDeployedWorkflows(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestOrchestratorPair(t, high, "2.0.0")
	addTestOrchestratorPair(t, low, "1.0.0")
	addTestWorkflow(t, high, "selected-flow", "reviewer")
	addTestWorkflow(t, low, "deployed-flow") // references no agents at all
	addTestWorkflow(t, high, "idle-flow")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, []string{"selected-flow"}, []string{"deployed-flow", "ghost-flow"})

	got := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemOrchestratorPair}).AffectedWorkflows
	want := []string{"deployed-flow", "selected-flow"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AffectedWorkflows = %s, want %s", joined(got), joined(want))
	}
}

func TestBuild_NonAgentNonPairItemsHaveNoAffectedWorkflows(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSkill(t, high, "my-skill")
	addTestSkill(t, low, "my-skill")
	addTestWorkflow(t, high, "some-flow", "reviewer")
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, []string{"some-flow"}, nil)

	line := lineFor(t, report, domain.CatalogItem{Kind: domain.CatalogItemSkill, Key: "my-skill"})
	if len(line.AffectedWorkflows) != 0 {
		t.Errorf("skill line AffectedWorkflows = %v, want none", line.AffectedWorkflows)
	}
}

func TestBuild_LinesAreOrderedByKindThenKey(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	claude := catalogpaths.HarnessDirNameClaudeCode
	for _, r := range []string{high, low} {
		addTestHook(t, r, "guard")
		addTestSkill(t, r, "zeta")
		addTestSkill(t, r, "alpha")
		addTestWorkflow(t, r, "flow")
		addTestSubagent(t, r, "reviewer", "1.0.0")
		mustWrite(t, r, "HarnessInjections/"+claude+"/HarnessInjections.md", "# x\n")
		mustWrite(t, r, catalogpaths.RelProtocolFile, "# p\n")
		addTestOrchestratorPair(t, r, "1.0.0")
	}
	c := newTestStack(t, mosaic, high, low)

	report := buildFor(c, nil, nil)

	var got []domain.CatalogItem
	for _, l := range report.Lines {
		got = append(got, l.Item)
	}
	want := []domain.CatalogItem{
		{Kind: domain.CatalogItemOrchestratorPair},
		{Kind: domain.CatalogItemSystemFile, Key: catalogpaths.RelProtocolFile},
		{Kind: domain.CatalogItemHarnessInjections, Key: claude},
		{Kind: domain.CatalogItemAgent, Key: "reviewer"},
		{Kind: domain.CatalogItemWorkflow, Key: "flow"},
		{Kind: domain.CatalogItemSkill, Key: "alpha"},
		{Kind: domain.CatalogItemSkill, Key: "zeta"},
		{Kind: domain.CatalogItemHook, Key: "guard"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("line order = %+v, want %+v", got, want)
	}
}
