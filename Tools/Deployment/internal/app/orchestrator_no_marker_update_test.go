package app_test

// orchestrator_no_marker_update_test.go specifies repeated Update runs (real planner, real
// executor) over orchestrator sources with and without the OrchestratorHarnessConstraints marker,
// on harnesses with and without orchestrator content:
//
//   - no marker, no harness orchestrator content (nothing, or whitespace only): the second
//     update reports both orchestrators unchanged and warns about nothing;
//   - no marker, harness orchestrator content: both orchestrators are updated again and the
//     undelivered-content warning is repeated on every update;
//   - marker present: stale once when the deployed file lacks the region, then clean, also when
//     the harness region is empty.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/manifest/manifesttest"
	"mosaic-deploy/internal/plan"
)

// nmDeployedOrchestrator is a previously deployed orchestrator that carries an outdated shared
// harness region and no orchestrator harness region.
const nmDeployedOrchestrator = `---
version: "1.0"
---

<Constraints type="core">
## Constraints

Always be helpful.

<HarnessConstraints type="managed" version="0.1.0">
old shared text
</HarnessConstraints>
</Constraints>
`

// The sources carry the protocol region too, so the protocol never makes a file stale by itself.
const nmProtocolRegion = "\n<CommunicationProtocol type=\"managed\">\n</CommunicationProtocol>\n"

const (
	nmSourceNoMarker   = stampSourceSharedMarkerOnly + nmProtocolRegion
	nmSourceWithMarker = stampSourceBothMarkers + nmProtocolRegion
)

var nmOrchestratorFiles = []string{"orchestrator.md", "orchestrator-script.md"}

// nmModule is a harness whose orchestrator content is configurable.
type nmModule struct {
	*stubHarnessModule
	orchContent string
	orchOK      bool
}

func newNMModule(orchContent string, orchOK bool) *nmModule {
	mod := newMinimalModule()
	mod.descriptor.InjectionsVersion = stampSharedVersion
	mod.descriptor.OrchestratorInjectionsVersion = stampOrchVersion
	return &nmModule{stubHarnessModule: mod, orchContent: orchContent, orchOK: orchOK}
}

func (m *nmModule) Injection(req domain.InjectionRequest) (string, bool) {
	switch req.Name {
	case docformat.HarnessRegionName:
		return stampSharedText, true
	case docformat.OrchestratorHarnessRegionName:
		if req.Role == domain.RoleOrchestrator {
			return m.orchContent, m.orchOK
		}
	}
	return "", false
}

// nmWorkspace is a temp workspace updated repeatedly with one wiring.
type nmWorkspace struct {
	t         *testing.T
	deps      app.Deps
	workspace string
}

// newNMWorkspace wires Update over both orchestrators built from source on the given harness
// module, with the outdated deployed orchestrators already on disk.
func newNMWorkspace(t *testing.T, source string, mod *nmModule) *nmWorkspace {
	t.Helper()
	return newNMMixedWorkspace(t, source, source, mod)
}

// newNMMixedWorkspace is newNMWorkspace with a separate source per orchestrator.
func newNMMixedWorkspace(t *testing.T, orchSource, scriptSource string, mod *nmModule) *nmWorkspace {
	t.Helper()
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newBaseDeps(t, stub)
	cat := scriptContentCatalog("/fake/orchestrator-script.md", "/fake/orchestrator.md")
	cat.agents = nil
	cat.sources["/fake/orchestrator.md"] = []byte(orchSource)
	cat.sources["/fake/orchestrator-script.md"] = []byte(scriptSource)
	deps.Catalog = cat
	deps.Planner = plan.New()
	deps.Registry = &stubRegistry{
		list:    []domain.HarnessRef{minimalHarness},
		modules: map[string]domain.HarnessModule{minimalHarness.ID: mod},
	}
	for _, name := range nmOrchestratorFiles {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(nmDeployedOrchestrator), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return &nmWorkspace{t: t, deps: deps, workspace: workspace}
}

// nmRun is the observable outcome of one Update run.
type nmRun struct {
	plan domain.Plan
	spy  *spyTodo
}

// update runs Update once. The manifest records the current bytes of both orchestrators, as the
// previous write would have, so only version staleness can mark them.
func (w *nmWorkspace) update() nmRun {
	w.t.Helper()
	var entries []domain.ManifestEntry
	for _, name := range nmOrchestratorFiles {
		data, err := os.ReadFile(filepath.Join(w.workspace, name))
		if err != nil {
			w.t.Fatalf("read %s: %v", name, err)
		}
		entries = append(entries, domain.ManifestEntry{
			Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: name[:len(name)-len(".md")]},
			TargetPath: name, Version: "1.0", ContentHash: manifest.Hash(data),
		})
	}
	w.deps.Manifest = &stubManifestStore{snap: manifest.Snapshot{State: manifest.StatePresent, Manifest: domain.Manifest{
		SchemaVersion: manifest.SchemaVersion, Harnesses: []domain.HarnessSection{{HarnessID: "stub-harness"}}, Entries: manifesttest.WithHarness("stub-harness", entries),
	}}}
	spy := &spyTodo{}
	w.deps.Todo = spy
	rec := &injectionRecorder{inner: deploy.NewExecutor(w.deps.Manifest, w.deps.Logger, spy)}
	w.deps.Executor = rec

	_, err := app.New(w.deps).Update(context.Background(), app.UpdateRequest{
		HarnessID:       "stub-harness",
		WorkspacePath:   w.workspace,
		AutoConfirmPlan: true,
		SkipAll:         map[domain.QuestionID]bool{domain.QTierModel: true, domain.QAgentModel: true},
	})
	if err != nil {
		w.t.Fatalf("Update: %v", err)
	}
	return nmRun{plan: rec.req.Plan, spy: spy}
}

func (r nmRun) action(key string) domain.PlanAction {
	for _, item := range r.plan.Items {
		if item.Ref.Kind == domain.ArtifactAgent && item.Ref.Key == key {
			return item.Action
		}
	}
	return ""
}

func (r nmRun) undeliveredWarnings(key string) int {
	n := 0
	for _, g := range r.spy.gaps {
		if g.Kind == domain.GapUndeliveredOrchestratorInjection && g.Owner == key {
			n++
		}
	}
	return n
}

func (r nmRun) reason(key string) string {
	for _, item := range r.plan.Items {
		if item.Ref.Key == key {
			return reasonText(item)
		}
	}
	return ""
}

func nmAssertNotUpdated(t *testing.T, label string, r nmRun) {
	t.Helper()
	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if got := r.action(key); got == domain.ActionUpdate {
			t.Errorf("%s: %s planned as %q, want it reported unchanged (%s)", label, key, got, r.reason(key))
		}
	}
}

func nmAssertUpdated(t *testing.T, label string, r nmRun) {
	t.Helper()
	for _, key := range []string{"orchestrator", "orchestrator-script"} {
		if got := r.action(key); got != domain.ActionUpdate {
			t.Errorf("%s: %s planned as %q, want %q", label, key, got, domain.ActionUpdate)
		}
	}
}

// No marker and a harness without orchestrator content: the first update writes the files, the
// second one finds nothing to do.
func TestUpdate_NoMarker_HarnessWithoutOrchestratorContent_SecondUpdateUnchanged(t *testing.T) {
	cases := map[string]*nmModule{
		"no content":    newNMModule("", false),
		"empty content": newNMModule("", true),
		"blank content": newNMModule(" \n\t\n", true),
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			ws := newNMWorkspace(t, nmSourceNoMarker, mod)

			first := ws.update()
			nmAssertUpdated(t, "first update", first)
			second := ws.update()

			nmAssertNotUpdated(t, "second update", second)
			for _, key := range []string{"orchestrator", "orchestrator-script"} {
				if n := second.undeliveredWarnings(key); n != 0 {
					t.Errorf("%s: %d undelivered-content warnings, want none without harness content", key, n)
				}
			}
		})
	}
}

// No marker but harness orchestrator content: the content cannot be delivered, so the tool keeps
// reporting the files stale and warns on every update.
func TestUpdate_NoMarker_HarnessWithOrchestratorContent_UpdatesAndWarnsEveryTime(t *testing.T) {
	ws := newNMWorkspace(t, nmSourceNoMarker, newNMModule(stampOrchText, true))

	for i, label := range []string{"first update", "second update", "third update"} {
		run := ws.update()

		nmAssertUpdated(t, label, run)
		for _, key := range []string{"orchestrator", "orchestrator-script"} {
			if n := run.undeliveredWarnings(key); n != 1 {
				t.Errorf("%s (run %d): %s has %d undelivered-content warnings, want exactly 1", label, i+1, key, n)
			}
		}
	}
}

// Marker present: stale once because the deployed file lacks the region, clean afterwards, with
// real or empty harness orchestrator content.
func TestUpdate_WithMarker_StaleOnce_ThenClean(t *testing.T) {
	cases := map[string]*nmModule{
		"content":       newNMModule(stampOrchText, true),
		"empty content": newNMModule("", true),
		"no content":    newNMModule("", false),
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			ws := newNMWorkspace(t, nmSourceWithMarker, mod)

			first := ws.update()
			nmAssertUpdated(t, "first update", first)
			second := ws.update()

			nmAssertNotUpdated(t, "second update", second)
			for _, key := range []string{"orchestrator", "orchestrator-script"} {
				if n := second.undeliveredWarnings(key); n != 0 {
					t.Errorf("%s: %d undelivered-content warnings with the marker present, want none", key, n)
				}
			}
		})
	}
}

// Mixed sources, harness without orchestrator content: the marker decision is per orchestrator,
// and neither orchestrator is stale or warned about on the second update.
func TestUpdate_MixedMarkers_HarnessWithoutOrchestratorContent_SecondUpdateUnchanged(t *testing.T) {
	cases := map[string][2]string{
		"marker only in orchestrator":        {nmSourceWithMarker, nmSourceNoMarker},
		"marker only in orchestrator-script": {nmSourceNoMarker, nmSourceWithMarker},
	}
	for name, sources := range cases {
		t.Run(name, func(t *testing.T) {
			ws := newNMMixedWorkspace(t, sources[0], sources[1], newNMModule("", false))

			first := ws.update()
			nmAssertUpdated(t, "first update", first)
			second := ws.update()

			nmAssertNotUpdated(t, "second update", second)
			for _, key := range []string{"orchestrator", "orchestrator-script"} {
				if n := second.undeliveredWarnings(key); n != 0 {
					t.Errorf("%s: %d undelivered-content warnings, want none without harness content", key, n)
				}
			}
		})
	}
}

// Mixed sources, harness with orchestrator content: only the orchestrator whose source lacks the
// marker is updated and warned about on every update; the other one is clean after one write.
func TestUpdate_MixedMarkers_HarnessWithOrchestratorContent_OnlyMarkerlessRepeats(t *testing.T) {
	cases := map[string]struct {
		sources    [2]string
		markerless string
		marked     string
	}{
		"marker only in orchestrator": {
			sources: [2]string{nmSourceWithMarker, nmSourceNoMarker}, markerless: "orchestrator-script", marked: "orchestrator",
		},
		"marker only in orchestrator-script": {
			sources: [2]string{nmSourceNoMarker, nmSourceWithMarker}, markerless: "orchestrator", marked: "orchestrator-script",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ws := newNMMixedWorkspace(t, tc.sources[0], tc.sources[1], newNMModule(stampOrchText, true))

			first := ws.update()
			nmAssertUpdated(t, "first update", first)
			second := ws.update()

			if got := second.action(tc.markerless); got != domain.ActionUpdate {
				t.Errorf("markerless %s planned as %q, want %q", tc.markerless, got, domain.ActionUpdate)
			}
			if n := second.undeliveredWarnings(tc.markerless); n != 1 {
				t.Errorf("markerless %s has %d undelivered-content warnings, want exactly 1", tc.markerless, n)
			}
			if got := second.action(tc.marked); got == domain.ActionUpdate {
				t.Errorf("marked %s planned as %q, want it reported unchanged (%s)", tc.marked, got, second.reason(tc.marked))
			}
			if n := second.undeliveredWarnings(tc.marked); n != 0 {
				t.Errorf("marked %s has %d undelivered-content warnings, want none", tc.marked, n)
			}
		})
	}
}
