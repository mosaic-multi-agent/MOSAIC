package main

// e2e_injection_migration_test.go runs the built mosaic-deploy binary to verify the in-place
// migration of a deployed workspace to the two-region layout. The workspace models a real one:
// subagents with and without a harness region, both orchestrators in the layout deployed before
// the fix (one merged harness region tagged with the orchestrator injection file version), and
// project content in project regions. The catalog then moves to the two-region layout and the
// shared injection file gains a constraint. Fixtures are built in temporary folders; nothing
// reads the shipped catalog.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// migAction is the part of one run summary action the migration tests read.
type migAction struct {
	Ref   struct{ Key string }
	Taken string
	Stale []struct{ Field, Deployed, Source string }
}

// migSummary is the part of the --output json run summary the migration tests read.
type migSummary struct {
	Outcome string
	Actions []migAction
}

// action returns the action recorded for the agent key.
func (s migSummary) action(t *testing.T, key string) migAction {
	t.Helper()
	for _, a := range s.Actions {
		if a.Ref.Key == key {
			return a
		}
	}
	t.Fatalf("run summary has no action for %q", key)
	return migAction{}
}

// staleCount returns how many staleness deltas of the field the action carries.
func (a migAction) staleCount(field string) int {
	n := 0
	for _, d := range a.Stale {
		if d.Field == field {
			n++
		}
	}
	return n
}

// migWorkspace is a deployed workspace in the legacy layout together with the mosaic root whose
// catalog has already moved to the two-region layout.
type migWorkspace struct {
	mosaicRoot string
	ws         string
	before     map[string]string // deployed text of every agent before the first update
}

// newMigWorkspace deploys the fixture from the pre-fix catalog, converts the orchestrators to
// the merged layout, fills the project regions, and then moves the catalog to the new layout.
func newMigWorkspace(t *testing.T) migWorkspace {
	t.Helper()
	mosaicRoot := newStackMosaicRoot(t)
	migWriteCatalog(t, mosaicRoot, false)
	ws := t.TempDir()
	deployWithStack(t, mosaicRoot, ws, migWorkflowID)
	m := migWorkspace{mosaicRoot: mosaicRoot, ws: ws, before: map[string]string{}}
	all := append(append(append([]string{}, migOrchestrators...), migRegionAgents...), migNoRegionAgents...)
	for _, key := range all {
		text := migReadDeployed(t, ws, key)
		if key == migOrchestrator || key == migOrchScript {
			text = migToLegacyLayout(t, text)
		}
		text = migWithProjectNote(t, text, key)
		migWriteDeployed(t, ws, key, text)
		m.before[key] = text
	}
	migClearOwnedContentRecords(t, ws)
	migWriteCatalog(t, mosaicRoot, true)
	return m
}

// update runs a real workspace update and returns its summary.
func (m migWorkspace) update(t *testing.T) migSummary {
	t.Helper()
	run := runInWorkspace(t, m.mosaicRoot, m.ws, "update", "--auto-confirm", "--output", "json")
	requireSuccess(t, run)
	var s migSummary
	if err := json.Unmarshal([]byte(run.Stdout), &s); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s", err, run.Stdout)
	}
	return s
}

func TestBinary_FirstUpdateAfterMigration_MarksEachAgentWithRegionStaleOnce(t *testing.T) {
	m := newMigWorkspace(t)

	s := m.update(t)

	for _, key := range append(append([]string{}, migRegionAgents...), migOrchestrators...) {
		a := s.action(t, key)
		if a.Taken != "updated" || a.staleCount("injections_version") != 1 {
			t.Errorf("%s: taken %q with %d injections_version deltas, want updated with exactly one", key, a.Taken, a.staleCount("injections_version"))
		}
	}
}

func TestBinary_FirstUpdateAfterMigration_LeavesAgentsWithoutRegionUnchanged(t *testing.T) {
	m := newMigWorkspace(t)

	s := m.update(t)

	for _, key := range migNoRegionAgents {
		a := s.action(t, key)
		if a.Taken != "unchanged" || len(a.Stale) != 0 {
			t.Errorf("%s: taken %q with deltas %+v, want unchanged and not stale", key, a.Taken, a.Stale)
		}
		if got := migReadDeployed(t, m.ws, key); got != m.before[key] {
			t.Errorf("%s: deployed bytes changed:\n%s", key, got)
		}
	}
}

func TestBinary_FirstUpdateAfterMigration_DeliversSharedConstraintsOncePerAgent(t *testing.T) {
	m := newMigWorkspace(t)

	m.update(t)

	for _, key := range append(append([]string{}, migRegionAgents...), migOrchestrators...) {
		text := migReadDeployed(t, m.ws, key)
		if n := strings.Count(text, migHeredocRule); n != 1 {
			t.Errorf("%s: shared constraint appears %d times, want 1", key, n)
		}
		if tag := migSharedRegionTag(text); tag != migSharedNewVersion {
			t.Errorf("%s: HarnessConstraints tag = %q, want %q", key, tag, migSharedNewVersion)
		}
	}
}

func TestBinary_FirstUpdateAfterMigration_ConvertsOrchestratorsInPlaceWithEachSourceTextOnce(t *testing.T) {
	m := newMigWorkspace(t)

	m.update(t)

	for _, key := range migOrchestrators {
		text := migReadDeployed(t, m.ws, key)
		for _, rule := range []string{migOldRule, migHeredocRule, migOrchRule} {
			if n := strings.Count(text, rule); n != 1 {
				t.Errorf("%s: %q appears %d times, want 1", key, rule, n)
			}
		}
		assertContains(t, key, text,
			"</HarnessConstraints>\n<OrchestratorHarnessConstraints type=\"managed\" version=\""+migOrchNewVersion+"\">\n"+migOrchRule+"\n</OrchestratorHarnessConstraints>")
		assertOmits(t, key, text, migOrchOldVersion)
		if strings.Contains(migSharedRegionRe.FindString(text), migOrchRule) {
			t.Errorf("%s: the shared region still holds the orchestrator rule", key)
		}
	}
}

func TestBinary_InjectionOnlyUpdate_LeavesAllOtherContentUnchanged(t *testing.T) {
	m := newMigWorkspace(t)

	m.update(t)

	for key, before := range m.before {
		after := migReadDeployed(t, m.ws, key)
		if got, want := migStripHarnessRegions(after), migStripHarnessRegions(before); got != want {
			t.Errorf("%s: content outside the harness regions and the version line changed\nwant:\n%s\ngot:\n%s", key, want, got)
		}
		assertContains(t, key, after, migProjectNote+key)
	}
}

func TestBinary_SecondUpdateAfterMigration_ReportsNoInjectionUpdates(t *testing.T) {
	m := newMigWorkspace(t)
	m.update(t)
	afterFirst := map[string]string{}
	for key := range m.before {
		afterFirst[key] = migReadDeployed(t, m.ws, key)
	}

	s := m.update(t)

	for key := range m.before {
		a := s.action(t, key)
		if a.Taken != "unchanged" {
			t.Errorf("%s: taken %q with deltas %+v, want unchanged", key, a.Taken, a.Stale)
		}
		for _, field := range []string{"injections_version", "orchestrator_injections_version"} {
			if n := a.staleCount(field); n != 0 {
				t.Errorf("%s: %d %s deltas on the second run, want none", key, n, field)
			}
		}
		if got := migReadDeployed(t, m.ws, key); got != afterFirst[key] {
			t.Errorf("%s: the second run changed the deployed file", key)
		}
	}
}

// migTodo is the part of one TODO item of the run summary the warning test reads.
type migTodo struct{ Subject, Owner, Detail string }

func TestBinary_DeployOrchestratorSourceWithoutSecondMarkerFromUserLayer_WarnsAndDropsOrchestratorRules(t *testing.T) {
	mosaicRoot := newStackMosaicRoot(t)
	user := newStackUserCatalog(t, "user-a", "2.0.0")
	for _, name := range migOrchestrators {
		writeStackFile(t, user, "Orchestrator/"+name+".md", []byte(migOrchestratorSource(name, "8.2.2", false)))
	}
	migWriteInjections(t, user, migSharedNewVersion, migOrchNewVersion, "OrchestratorHarnessConstraints", []string{migOldRule})
	ws := t.TempDir()

	run := runInWorkspace(t, mosaicRoot, ws, "deploy", "--workflows", "user-a-wf", "--auto-confirm",
		"--output", "json", "--catalog-folder", user)

	var s struct{ Todos []migTodo }
	if err := json.Unmarshal([]byte(run.Stdout), &s); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s\nstderr: %s", err, run.Stdout, run.Stderr)
	}
	for _, key := range migOrchestrators {
		warnings := 0
		for _, td := range s.Todos {
			if td.Subject == "OrchestratorHarnessConstraints" && td.Owner == key {
				warnings++
				assertContains(t, "warning for "+key, td.Detail, key, "HarnessInjectionsOrchestrator.md")
			}
		}
		if warnings != 1 {
			t.Errorf("%s: %d missing-region warnings, want 1\ntodos: %+v", key, warnings, s.Todos)
		}
		if _, err := os.Stat(migDeployedPath(ws, key)); err != nil {
			t.Fatalf("%s was not deployed: %v", key, err)
		}
		assertOmits(t, key, migReadDeployed(t, ws, key), migOrchRule)
	}
}
