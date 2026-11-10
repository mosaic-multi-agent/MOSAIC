package conflictreport_test

// harness_filter_test.go verifies that a report built for a run with a target harness keeps
// harness-injection shadows only for that harness's injection directory, never filters any other
// kind of shadow, and that a run without a target harness reports every shadow as before.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app/conflictreport"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/domain"
)

// injectionItem is the report item of a harness injection directory.
func injectionItem(dir string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: dir}
}

// newShadowedMixedStack returns a two-layer stack in which both layers supply injections for
// Claude Code and OpenCode and one item of every other shadowable kind.
func newShadowedMixedStack(t *testing.T) stack.Catalog {
	t.Helper()
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	for _, r := range []string{high, low} {
		for _, dir := range []string{catalogpaths.HarnessDirNameClaudeCode, catalogpaths.HarnessDirNameOpenCode} {
			mustWrite(t, r, "HarnessInjections/"+dir+"/HarnessInjections.md", "# x\n")
		}
		addTestSubagent(t, r, "reviewer", "1.0.0")
		addTestWorkflow(t, r, "flow")
		addTestSkill(t, r, "alpha")
		addTestHook(t, r, "guard")
		mustWrite(t, r, catalogpaths.RelProtocolFile, "# p\n")
		addTestOrchestratorPair(t, r, "1.0.0")
	}
	return newTestStack(t, mosaic, high, low)
}

// buildScoped runs Build over the stack's shadows with the given harness scope.
func buildScoped(c stack.Catalog, scoped bool, dir string) domain.ConflictReport {
	return conflictreport.Build(conflictreport.Input{
		Shadows:       c.Shadows(),
		Workflow:      c.Workflow,
		HarnessScoped: scoped,
		InjectionDir:  dir,
	})
}

// itemsOf returns the items of the report lines, in order.
func itemsOf(r domain.ConflictReport) []domain.CatalogItem {
	var out []domain.CatalogItem
	for _, l := range r.Lines {
		out = append(out, l.Item)
	}
	return out
}

// nonInjectionItems returns the items of the report that are not harness injections.
func nonInjectionItems(r domain.ConflictReport) []domain.CatalogItem {
	var out []domain.CatalogItem
	for _, it := range itemsOf(r) {
		if it.Kind != domain.CatalogItemHarnessInjections {
			out = append(out, it)
		}
	}
	return out
}

func containsItem(items []domain.CatalogItem, want domain.CatalogItem) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

func TestBuild_HarnessScoped_DropsInjectionShadowsOfOtherHarnesses(t *testing.T) {
	c := newShadowedMixedStack(t)

	report := buildScoped(c, true, catalogpaths.HarnessDirNameOpenCode)

	items := itemsOf(report)
	if containsItem(items, injectionItem(catalogpaths.HarnessDirNameClaudeCode)) {
		t.Errorf("report lists the Claude Code injection shadow for an OpenCode run: %+v", items)
	}
	if !containsItem(items, injectionItem(catalogpaths.HarnessDirNameOpenCode)) {
		t.Errorf("report lacks the target harness's own injection shadow: %+v", items)
	}
}

func TestBuild_HarnessScoped_KeepsEveryNonInjectionShadow(t *testing.T) {
	c := newShadowedMixedStack(t)
	unscoped := buildScoped(c, false, "")

	scoped := buildScoped(c, true, catalogpaths.HarnessDirNameOpenCode)

	if len(nonInjectionItems(unscoped)) < 6 {
		t.Fatalf("fixture produces too few non-injection shadows: %+v", itemsOf(unscoped))
	}
	if got, want := nonInjectionItems(scoped), nonInjectionItems(unscoped); !reflect.DeepEqual(got, want) {
		t.Errorf("non-injection lines = %+v, want all of %+v", got, want)
	}
	if len(scoped.Lines) >= len(unscoped.Lines) {
		t.Errorf("scoped report has %d lines, want fewer than the unscoped %d", len(scoped.Lines), len(unscoped.Lines))
	}
}

func TestBuild_HarnessScoped_KeptLinesAreUnchanged(t *testing.T) {
	c := newShadowedMixedStack(t)
	unscoped := buildScoped(c, false, "")

	scoped := buildScoped(c, true, catalogpaths.HarnessDirNameClaudeCode)

	var want []domain.ConflictLine
	for _, l := range unscoped.Lines {
		if l.Item.Kind == domain.CatalogItemHarnessInjections && l.Item.Key != catalogpaths.HarnessDirNameClaudeCode {
			continue
		}
		want = append(want, l)
	}
	if len(want) == len(unscoped.Lines) {
		t.Fatal("fixture error: nothing to filter")
	}
	if !reflect.DeepEqual(scoped.Lines, want) {
		t.Errorf("scoped lines = %+v, want the unscoped lines minus the other harness's injections %+v", scoped.Lines, want)
	}
}

func TestBuild_HarnessScopedWithEmptyDir_DropsEveryInjectionShadow(t *testing.T) {
	c := newShadowedMixedStack(t)

	report := buildScoped(c, true, "")

	for _, it := range itemsOf(report) {
		if it.Kind == domain.CatalogItemHarnessInjections {
			t.Errorf("report keeps the injection shadow %+v for a harness that reads no stack injections", it)
		}
	}
	if len(nonInjectionItems(report)) == 0 {
		t.Error("non-injection shadows were dropped too, want them kept")
	}
}

func TestBuild_NotHarnessScoped_KeepsEveryInjectionShadowEvenWithDir(t *testing.T) {
	c := newShadowedMixedStack(t)

	report := buildScoped(c, false, catalogpaths.HarnessDirNameOpenCode)

	items := itemsOf(report)
	for _, dir := range []string{catalogpaths.HarnessDirNameClaudeCode, catalogpaths.HarnessDirNameOpenCode} {
		if !containsItem(items, injectionItem(dir)) {
			t.Errorf("unscoped report lacks the injection shadow of %q: %+v", dir, items)
		}
	}
}

func TestBuild_HarnessScoped_OnlyOtherInjectionShadows_ReturnsEmptyReport(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	for _, r := range []string{high, low} {
		mustWrite(t, r, "HarnessInjections/"+catalogpaths.HarnessDirNameClaudeCode+"/HarnessInjections.md", "# x\n")
	}
	c := newTestStack(t, mosaic, high, low)

	report := buildScoped(c, true, catalogpaths.HarnessDirNameOpenCode)

	if !reflect.DeepEqual(report, domain.ConflictReport{}) {
		t.Errorf("report = %+v, want the zero report when every shadow is filtered out", report)
	}
}

func TestBuild_HarnessScoped_OtherHarnessUndeclaredShadow_IsNotAWarningLine(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	for _, r := range []string{high, low} {
		mustWrite(t, r, "HarnessInjections/"+catalogpaths.HarnessDirNameClaudeCode+"/HarnessInjections.md", "# x\n")
		addTestSubagent(t, r, "reviewer", "1.0.0")
	}
	c := newTestStack(t, mosaic, high, low)

	report := buildScoped(c, true, catalogpaths.HarnessDirNameOpenCode)

	for _, l := range report.Lines {
		if l.Severity == domain.ConflictWarning && l.Item.Kind == domain.CatalogItemHarnessInjections {
			t.Errorf("warning line for another harness's injections: %+v", l)
		}
	}
	lineFor(t, report, agentItem("reviewer"))
}
