package app_test

// conflict_report_harness_test.go verifies that the conflict report of a run with a target
// harness lists harness-injection shadows only for that harness's injection directory, in every
// write path and in render, that no auto-confirm TODO gap is recorded for another harness's
// directory, and that a run whose registry cannot name the directory is left unscoped.

import (
	"context"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

const (
	crTargetDir = catalogpaths.HarnessDirNameOpenCode
	crOtherDir  = catalogpaths.HarnessDirNameClaudeCode
)

// crDirRegistry is a registry that also reports the injection directory of each harness id.
type crDirRegistry struct {
	registry.Registry
	dirs map[string]string
}

func (r crDirRegistry) InjectionDirName(id string) (string, bool) {
	d, ok := r.dirs[id]
	return d, ok
}

// crInjectionItem is the report item of a harness injection directory.
func crInjectionItem(dir string) domain.CatalogItem {
	return domain.CatalogItem{Kind: domain.CatalogItemHarnessInjections, Key: dir}
}

// crSupplyInjections makes both layers supply injections for the target and the other directory,
// so each is a shadowed, undeclared item.
func crSupplyInjections(t *testing.T, mosaic string, layers ...string) {
	t.Helper()
	roots := append([]string{filepath.Join(mosaic, catalogpaths.CatalogDirName)}, layers...)
	for _, root := range roots {
		for _, dir := range []string{crTargetDir, crOtherDir} {
			writeSysFile(t, filepath.Join(root, "HarnessInjections", dir, "HarnessInjections.md"), "# x\n")
		}
	}
}

// crHasLine reports whether the report has a line for item.
func crHasLine(report domain.ConflictReport, item domain.CatalogItem) bool {
	for _, l := range report.Lines {
		if l.Item == item {
			return true
		}
	}
	return false
}

// crInjectionDepsFor returns write-path deps over a stack whose layers shadow the injections of
// two harness directories, with the registry mapping "stub-harness" to dirs.
func crInjectionDepsFor(t *testing.T, stub *interactiontest.Stub, dirs map[string]string) (app.Deps, string) {
	t.Helper()
	deps, ws := newBaseDeps(t, stub)
	cat := deps.Catalog.(*stubCatalog)
	cat.orchestratorScript = domain.Agent{Key: "orchestrator-script", Version: "1.0", Name: "Orchestrator Script", Role: domain.RoleOrchestrator}
	cat.orchestratorScriptOK = true
	cat.agents = append(cat.agents, crOtherAgent)
	mosaic, layers := crLayers(t, crStackOpts{})
	crSupplyInjections(t, mosaic, layers[0].Root)
	useResolvedStack(t, &deps, mosaic, layers...)
	if dirs != nil {
		deps.Registry = crDirRegistry{Registry: deps.Registry, dirs: dirs}
	}
	return deps, ws
}

// crRenderInjectionDeps returns render deps over the same shadowing stack.
func crRenderInjectionDeps(t *testing.T, dirs map[string]string) (app.Deps, *spyTodo) {
	t.Helper()
	deps := newRenderDeps(t)
	spy := &spyTodo{}
	deps.Todo = spy
	cat := deps.Catalog.(*stubCatalog)
	cat.agents = append(cat.agents, crOtherAgent)
	mosaic, layers := crLayers(t, crStackOpts{})
	crSupplyInjections(t, mosaic, layers[0].Root)
	useResolvedStack(t, &deps, mosaic, layers...)
	if dirs != nil {
		deps.Registry = crDirRegistry{Registry: deps.Registry, dirs: dirs}
	}
	return deps, spy
}

func TestConflictReport_TargetHarness_OmitsOtherHarnessInjectionLines(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crInjectionDepsFor(t, interactiontest.NewBuilder().AnswerReview(true).Build(),
				map[string]string{"stub-harness": crTargetDir})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			if crHasLine(summary.ConflictReport, crInjectionItem(crOtherDir)) {
				t.Errorf("report lists the %q injection shadow for a run that targets %q: %+v", crOtherDir, crTargetDir, summary.ConflictReport.Lines)
			}
			crLineFor(t, summary.ConflictReport, crInjectionItem(crTargetDir))
			crLineFor(t, summary.ConflictReport, crAgentItem())
		})
	}
}

func TestConflictReport_TargetHarness_AutoConfirmRecordsNoGapForOtherHarnessInjections(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crInjectionDepsFor(t, interactiontest.NewBuilder().Build(),
				map[string]string{"stub-harness": crTargetDir})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
			spy := &spyTodo{}
			deps.Todo = spy

			_, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			subjects := map[string]bool{}
			for _, g := range crShadowGaps(spy) {
				subjects[g.Subject] = true
			}
			if other := string(domain.CatalogItemHarnessInjections) + ":" + crOtherDir; subjects[other] {
				t.Errorf("gap recorded for another harness's injections %q; gaps: %+v", other, spy.gaps)
			}
			if own := string(domain.CatalogItemHarnessInjections) + ":" + crTargetDir; !subjects[own] {
				t.Errorf("no gap for the target harness's own undeclared injection shadow %q; gaps: %+v", own, spy.gaps)
			}
			if !subjects["agent:"+crAgentKey] {
				t.Errorf("agent shadow gap missing; gaps: %+v", spy.gaps)
			}
		})
	}
}

func TestConflictReport_HarnessWithoutStackInjections_ReportsNoInjectionShadows(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crInjectionDepsFor(t, interactiontest.NewBuilder().AnswerReview(true).Build(),
				map[string]string{"some-other-harness": crOtherDir})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			for _, dir := range []string{crTargetDir, crOtherDir} {
				if crHasLine(summary.ConflictReport, crInjectionItem(dir)) {
					t.Errorf("report lists the %q injection shadow for a harness that has no stack injection directory", dir)
				}
			}
			crLineFor(t, summary.ConflictReport, crAgentItem())
		})
	}
}

func TestConflictReport_RegistryWithoutDirReporter_ReportIsUnscoped(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crInjectionDepsFor(t, interactiontest.NewBuilder().AnswerReview(true).Build(), nil)
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			for _, dir := range []string{crTargetDir, crOtherDir} {
				crLineFor(t, summary.ConflictReport, crInjectionItem(dir))
			}
		})
	}
}

func TestRenderAgent_TargetHarness_OmitsOtherHarnessInjectionLines(t *testing.T) {
	deps, spy := crRenderInjectionDeps(t, map[string]string{renderHarness.ID: crTargetDir})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	result, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want the agent and target injection lines")
	}
	if crHasLine(*result.ConflictReport, crInjectionItem(crOtherDir)) {
		t.Errorf("render report lists the %q injection shadow for a render targeting %q: %+v", crOtherDir, crTargetDir, result.ConflictReport.Lines)
	}
	crLineFor(t, *result.ConflictReport, crInjectionItem(crTargetDir))
	crLineFor(t, *result.ConflictReport, crAgentItem())
	if len(spy.gaps) != 0 {
		t.Errorf("render recorded gaps %+v, want none", spy.gaps)
	}
}

func TestRenderAgent_HarnessWithoutStackInjections_ReportsNoInjectionShadows(t *testing.T) {
	deps, _ := crRenderInjectionDeps(t, map[string]string{"some-other-harness": crOtherDir})
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	result, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want the agent line")
	}
	for _, d := range []string{crTargetDir, crOtherDir} {
		if crHasLine(*result.ConflictReport, crInjectionItem(d)) {
			t.Errorf("render report lists the %q injection shadow for a harness without stack injections", d)
		}
	}
	crLineFor(t, *result.ConflictReport, crAgentItem())
}

func TestRenderAgent_RegistryWithoutDirReporter_ReportIsUnscoped(t *testing.T) {
	deps, _ := crRenderInjectionDeps(t, nil)
	dir := t.TempDir()
	src := writeRenderSourceFile(t, dir, "render-test-subagent.md", minimalGenericSubagentBytes())

	result, err := app.New(deps).RenderAgent(context.Background(), newPreAnsweredRenderRequest(src, filepath.Join(dir, "out.md")))

	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if result.ConflictReport == nil {
		t.Fatal("ConflictReport = nil, want every shadow line")
	}
	for _, d := range []string{crTargetDir, crOtherDir} {
		crLineFor(t, *result.ConflictReport, crInjectionItem(d))
	}
}
