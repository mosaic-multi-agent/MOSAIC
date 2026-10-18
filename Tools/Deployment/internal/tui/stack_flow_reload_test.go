package tui

// stack_flow_reload_test.go verifies what confirming the stack screen does: the service is
// rebuilt from exactly the confirmed stack together with its origin, rebuild and selected-harness
// injection failures stay on the stack screen, and the run uses the rebuilt service in every mode.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// addLayerOnStack adds dir as a user layer through the stack screen's path entry.
func (f *stackFlow) addLayerOnStack(dir string) {
	f.typeText("a")
	f.typeText(dir)
	f.key(tea.KeyEnter)
}

// requireOneReload fails unless the reload capability was called exactly once and returns the call.
func requireOneReload(t *testing.T, f *stackFlow) reloadCall {
	t.Helper()
	if len(f.rec.calls) != 1 {
		t.Fatalf("the reload capability was called %d times, want exactly once", len(f.rec.calls))
	}
	return f.rec.calls[0]
}

func TestStackFlow_Confirm_RebuildsFromExactlyTheConfirmedStack(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	cases := []struct {
		name   string
		launch stackconfig.Description
		given  bool
	}{
		{"default layer alone", stackconfig.Description{IncludeDefault: true}, false},
		{"user catalog, default layer left out", stackconfig.Description{UserLayers: []string{a}}, true},
		{"user catalogs, default layer kept", stackconfig.Description{UserLayers: []string{a, b}, IncludeDefault: true}, true},
		{"user catalogs in launch order", stackconfig.Description{UserLayers: []string{b, a}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStackFlow(t, domain.ModeDeployWorkspace, withLaunch(tc.launch, tc.given))
			f.toStack(t)

			f.confirmStack(t)

			if got := requireOneReload(t, f).d; !sameDescription(got, tc.launch) {
				t.Errorf("rebuilt from %+v, want exactly %+v", got, tc.launch)
			}
		})
	}
}

func TestStackFlow_Confirm_DefaultLayerIsIncludedOnlyWhenKept(t *testing.T) {
	user := t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace, withLaunch(stackconfig.Description{UserLayers: []string{user}}, true))
	f.toStack(t)
	f.key(tea.KeyDown)
	f.space() // keep the default layer

	f.confirmStack(t)

	want := stackconfig.Description{UserLayers: []string{user}, IncludeDefault: true}
	if got := requireOneReload(t, f).d; !sameDescription(got, want) {
		t.Errorf("rebuilt from %+v, want %+v", got, want)
	}
}

func TestStackFlow_Confirm_ChangesMadeOnTheScreenAreWhatIsRebuilt(t *testing.T) {
	added := filepath.Clean(t.TempDir())
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)
	f.addLayerOnStack(added)

	f.confirmStack(t)

	want := stackconfig.Description{UserLayers: []string{added}, IncludeDefault: true}
	if got := requireOneReload(t, f).d; !sameDescription(got, want) {
		t.Errorf("rebuilt from %+v, want %+v", got, want)
	}
}

func TestStackFlow_Confirm_AdoptsTheRebuiltServiceAndMovesToRunning(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)

	f.confirmStack(t)

	if f.m.screen != screenRunning {
		t.Errorf("screen = %v after a successful rebuild, want screenRunning", f.m.screen)
	}
	if f.m.svc != f.reloaded {
		t.Error("the session did not adopt the rebuilt service")
	}
}

func TestStackFlow_Origin_UnchangedStackKeepsTheStartingOrigin(t *testing.T) {
	user := t.TempDir()
	cases := []struct {
		name   string
		launch stackconfig.Description
		given  bool
		want   stackconfig.Origin
	}{
		{"launched without stack input", stackconfig.Description{IncludeDefault: true}, false, stackconfig.OriginDefault},
		{"launched with a user catalog", stackconfig.Description{UserLayers: []string{user}}, true, stackconfig.OriginStackInput},
		{"launched with the default-layer flag alone", stackconfig.Description{IncludeDefault: true}, true, stackconfig.OriginStackInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStackFlow(t, domain.ModeDeployWorkspace, withLaunch(tc.launch, tc.given))
			f.toStack(t)

			f.confirmStack(t)

			if got := requireOneReload(t, f).origin; got != tc.want {
				t.Errorf("origin = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStackFlow_Origin_ChangedStackIsStackInput(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace) // launched without stack input
	f.toStack(t)
	f.addLayerOnStack(t.TempDir())

	f.confirmStack(t)

	if got := requireOneReload(t, f).origin; got != stackconfig.OriginStackInput {
		t.Errorf("origin = %q after a change on the screen, want %q", got, stackconfig.OriginStackInput)
	}
}

func TestStackFlow_Origin_ChangeThatIsUndoneCountsAsUnchanged(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace) // launched without stack input
	f.toStack(t)
	f.addLayerOnStack(t.TempDir()) // so the default layer may be left out
	f.key(tea.KeyDown)             // onto the default row
	f.space()                      // leave it out
	f.space()                      // keep it again
	f.key(tea.KeyUp)
	f.typeText("d") // remove the added layer: the stack equals the start again

	f.confirmStack(t)

	call := requireOneReload(t, f)
	if want := (stackconfig.Description{IncludeDefault: true}); !sameDescription(call.d, want) {
		t.Fatalf("rebuilt from %+v, want %+v", call.d, want)
	}
	if call.origin != stackconfig.OriginDefault {
		t.Errorf("origin = %q for a change that was undone, want the starting origin %q", call.origin, stackconfig.OriginDefault)
	}
}

func TestStackFlow_Origin_ReorderedLayersAreAChange(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace,
		withLaunch(stackconfig.Description{UserLayers: []string{a, b}, IncludeDefault: true}, true))
	f.toStack(t)
	f.key(tea.KeyDown)
	f.key(tea.KeyShiftUp) // move the second layer above the first

	f.confirmStack(t)

	call := requireOneReload(t, f)
	if want := (stackconfig.Description{UserLayers: []string{b, a}, IncludeDefault: true}); !sameDescription(call.d, want) {
		t.Errorf("rebuilt from %+v, want %+v", call.d, want)
	}
	if call.origin != stackconfig.OriginStackInput {
		t.Errorf("origin = %q after a reorder, want %q", call.origin, stackconfig.OriginStackInput)
	}
}

func TestStackFlow_Origin_OnlyTogglingTheDefaultLayerIsAChange(t *testing.T) {
	a := t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace,
		withLaunch(stackconfig.Description{UserLayers: []string{a}}, true))
	f.toStack(t)
	f.key(tea.KeyDown) // onto the default row, which starts left out
	f.space()          // keep it

	f.confirmStack(t)

	call := requireOneReload(t, f)
	if want := (stackconfig.Description{UserLayers: []string{a}, IncludeDefault: true}); !sameDescription(call.d, want) {
		t.Fatalf("rebuilt from %+v, want %+v", call.d, want)
	}
	if call.origin != stackconfig.OriginStackInput {
		t.Errorf("origin = %q after only the default-layer toggle changed, want %q", call.origin, stackconfig.OriginStackInput)
	}
}

func TestStackFlow_Origin_OnlyRemovingALaunchLayerIsAChange(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	f := newStackFlow(t, domain.ModeDeployWorkspace,
		withLaunch(stackconfig.Description{UserLayers: []string{a, b}, IncludeDefault: true}, true))
	f.toStack(t)
	f.typeText("d") // remove the top layer, the cursor starts on it

	f.confirmStack(t)

	call := requireOneReload(t, f)
	if want := (stackconfig.Description{UserLayers: []string{b}, IncludeDefault: true}); !sameDescription(call.d, want) {
		t.Fatalf("rebuilt from %+v, want %+v", call.d, want)
	}
	if call.origin != stackconfig.OriginStackInput {
		t.Errorf("origin = %q after only a layer was removed, want %q", call.origin, stackconfig.OriginStackInput)
	}
}

func TestStackFlow_Run_UsesTheConfirmedStacksServiceInEveryMode(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)
			f.toStack(t)

			runStarted(f.confirmStack(t))

			if want := "reloaded:" + operationOf[mode]; len(f.log.entries) != 1 || f.log.entries[0] != want {
				t.Errorf("run log = %v, want only [%s]: the run must use the rebuilt service", f.log.entries, want)
			}
		})
	}
}

func TestStackFlow_RebuildError_KeepsTheUserOnTheStackScreenWithTheServiceUnchanged(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)
			f.rec.err = errors.New("REBUILDFAILED-duplicate-catalog-id")
			f.toStack(t)

			cmd := f.confirmStack(t)

			if !f.onStack() {
				t.Fatalf("not on the stack screen after a rebuild error (screen = %v):\n%s", f.m.screen, f.m.View())
			}
			if !strings.Contains(f.m.View(), "REBUILDFAILED-duplicate-catalog-id") {
				t.Errorf("the rebuild error is not shown on the stack screen:\n%s", f.m.View())
			}
			if f.m.svc != f.orig {
				t.Error("the session's service was replaced although the rebuild failed")
			}
			runStarted(cmd)
			if len(f.log.entries) != 0 {
				t.Errorf("a run started although the rebuild failed: %v", f.log.entries)
			}
		})
	}
}

func TestStackFlow_RebuildError_UserCanConfirmAgainAfterFixing(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.rec.err = errors.New("REBUILDFAILED-numeric-id")
	f.toStack(t)
	f.confirmStack(t)
	f.rec.err = nil

	runStarted(f.confirmStack(t))

	if len(f.rec.calls) != 2 {
		t.Errorf("the reload capability was called %d times, want 2 (one failure, one retry)", len(f.rec.calls))
	}
	if len(f.log.entries) != 1 || f.log.entries[0] != "reloaded:deploy" {
		t.Errorf("run log = %v, want [reloaded:deploy]", f.log.entries)
	}
}

func TestStackFlow_MissingInjections_ShowsTheErrorOnTheStackScreenAndStaysThere(t *testing.T) {
	for _, mode := range entryModes {
		t.Run(string(mode), func(t *testing.T) {
			f := newStackFlow(t, mode)
			f.reloaded.injectionErr = errors.New("NOSUPPLIER-HarnessInjections-flow-harness")
			f.toStack(t)

			cmd := f.confirmStack(t)

			if !f.onStack() {
				t.Fatalf("not on the stack screen after missing injections (screen = %v):\n%s", f.m.screen, f.m.View())
			}
			if !strings.Contains(f.m.View(), "NOSUPPLIER-HarnessInjections-flow-harness") {
				t.Errorf("the injection error is not shown on the stack screen:\n%s", f.m.View())
			}
			runStarted(cmd)
			if len(f.log.entries) != 0 {
				t.Errorf("a run started although no layer supplies the injections: %v", f.log.entries)
			}
		})
	}
}

func TestStackFlow_InjectionCheck_AsksTheRebuiltServiceAboutTheSelectedHarness(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.toStack(t)

	f.confirmStack(t)

	if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != flowTestHarness.ID {
		t.Errorf("rebuilt service was asked about harnesses %v, want [%s]", f.reloaded.checked, flowTestHarness.ID)
	}
	if len(f.orig.checked) != 0 {
		t.Errorf("the launch service was asked about harnesses %v; the check belongs to the rebuilt stack", f.orig.checked)
	}
}

func TestStackFlow_InjectionCheck_PreselectedHarnessIsTheOneChecked(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace, withPreselectedHarness())
	f.reloaded.injectionErr = errors.New("NOSUPPLIER-preselected")
	f.toStack(t)

	f.confirmStack(t)

	if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != flowTestHarness.ID {
		t.Errorf("rebuilt service was asked about harnesses %v, want [%s]", f.reloaded.checked, flowTestHarness.ID)
	}
	if !f.onStack() || !strings.Contains(f.m.View(), "NOSUPPLIER-preselected") {
		t.Errorf("the injection error for the pre-selected harness is not shown on the stack screen:\n%s", f.m.View())
	}
}

func TestStackFlow_InjectionCheck_TransformChecksOnlyTheSourceHarness(t *testing.T) {
	f := newStackFlow(t, domain.ModeTransformHarness)
	f.toStack(t)

	f.confirmStack(t)

	if len(f.reloaded.checked) != 1 || f.reloaded.checked[0] != flowTestHarness.ID {
		t.Errorf("rebuilt service was asked about harnesses %v, want only the source harness [%s]",
			f.reloaded.checked, flowTestHarness.ID)
	}
}

func TestStackFlow_InjectionCheck_NotRunWhenTheRebuildFailed(t *testing.T) {
	f := newStackFlow(t, domain.ModeDeployWorkspace)
	f.rec.err = errors.New("REBUILDFAILED-x")
	f.toStack(t)

	f.confirmStack(t)

	if len(f.reloaded.checked) != 0 || len(f.orig.checked) != 0 {
		t.Errorf("injections were checked (rebuilt %v, launch %v) although there was no rebuilt service",
			f.reloaded.checked, f.orig.checked)
	}
}
