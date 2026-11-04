package tui

// Tests that the TUI entry path refuses to start a run whose model screen was
// passed without a subject model, through the real pre-flight: the run does
// not start and the user is told why. A run with a model chosen on the model
// screen proceeds.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/authoring"
	"mosaic-agent-test/internal/preflight"
)

// guardSuitePath returns the absolute path of the tool-local fixture suite
// used to drive the real pre-flight.
func guardSuitePath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "testdata", "model_guard", "suite", "s.suite.yaml"))
	if err != nil {
		t.Fatalf("resolving fixture suite path: %v", err)
	}
	return p
}

// realPreflight wraps preflight.Validate with the fixture root the fixture
// suite needs, which the composition root supplies in production.
func realPreflight(t *testing.T) PreflightFunc {
	t.Helper()
	fixtureRoot := filepath.Join(filepath.Dir(guardSuitePath(t)), "fixtures")
	return func(in preflight.Input) (preflight.Plan, authoring.Report) {
		in.FixtureRoot = fixtureRoot
		return preflight.Validate(in)
	}
}

// TestSubjectSelection_ModelScreenPassedWithoutChoice_RunDoesNotStart drives
// the empty-catalog path, where the user can press Enter through both model
// phases without choosing, and asserts the run is refused and explained.
func TestSubjectSelection_ModelScreenPassedWithoutChoice_RunDoesNotStart(t *testing.T) {
	runner := newFakeSuiteRunner()
	o := newHarnessFixtureOptions([]string{guardSuitePath(t)}, runner)
	o.Preflight = realPreflight(t)

	m := NewModel(o)
	m = advanceToModelSelect(t, m)
	m = advanceThroughModelSelect(t, m)
	m = triggerSuiteStart(t, m)

	if m.Screen() == ScreenProgress {
		t.Fatalf("Screen() = %q; a run with no subject model must not start", m.Screen())
	}
	runner.mu.Lock()
	called := runner.called
	runner.mu.Unlock()
	if called {
		t.Error("suite runner was invoked for a run with no subject model")
	}
	_, body, shown := m.FailureDetail()
	if !shown {
		t.Fatal("FailureDetail() shown = false; the user must be told why the run did not start")
	}
	if !strings.Contains(body, "missing-subject-model") {
		t.Errorf("failure detail does not report missing-subject-model:\n%s", body)
	}
}

// TestSubjectSelection_ModelChosenOnModelScreen_RunStarts is the mirror case.
func TestSubjectSelection_ModelChosenOnModelScreen_RunStarts(t *testing.T) {
	runner := newFakeSuiteRunner()
	o := newModelSelectOptions([]string{guardSuitePath(t)}, runner)
	o.Preflight = realPreflight(t)

	m := NewModel(o)
	m = advanceToModelSelect(t, m)
	m = advanceThroughModelSelect(t, m) // first catalog model for the subject, stub follows it
	m = triggerSuiteStart(t, m)

	if m.Screen() != ScreenProgress {
		_, body, _ := m.FailureDetail()
		t.Fatalf("Screen() = %q, want %q for a run with a subject model chosen; failure detail:\n%s", m.Screen(), ScreenProgress, body)
	}
}
