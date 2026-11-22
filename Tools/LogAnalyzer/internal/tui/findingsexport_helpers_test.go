package tui

// findingsexport_helpers_test.go provides an in-memory findings file writer and
// a model builder for the findings export tests: a loaded session (so the digest
// export is available too), a report with findings of two runs, the unattributable
// bucket and no run, and a service that has both a digest store and a findings
// file writer.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

// tuiFakeFindingsWriter records findings writes and refuses to replace an
// existing path without overwrite.
type tuiFakeFindingsWriter struct {
	existing map[string]bool
	failWith error // returned by WriteFindings when set

	writes []tuiFindingsWrite
}

type tuiFindingsWrite struct {
	path      string
	findings  []domain.Finding
	overwrite bool
}

func (f *tuiFakeFindingsWriter) WriteFindings(_ context.Context, path string, findings []domain.Finding, overwrite bool) error {
	if f.failWith != nil {
		return f.failWith
	}
	if f.existing[path] && !overwrite {
		return fmt.Errorf("write %s: %w", path, domain.ErrFindingsFileExists)
	}
	f.writes = append(f.writes, tuiFindingsWrite{path: path, findings: append([]domain.Finding{}, findings...), overwrite: overwrite})
	return nil
}

var _ domain.FindingsFileWriter = (*tuiFakeFindingsWriter)(nil)

// newFindingsFlowModel returns a model on the runs list whose report carries
// findingsReportFor(exportRunA, exportRunB) and whose service writes digests to
// store and findings to writer.
func newFindingsFlowModel(t *testing.T, store *tuiFakeDigestStore, writer *tuiFakeFindingsWriter) Model {
	t.Helper()
	m := newExportModel(t, store)
	m.opts.Service = app.New(app.Deps{
		Source:        &tuiFakeLogSource{},
		Reader:        &tuiFakeEventReader{},
		Pricing:       &tuiFakePricingStore{path: "/test/pricing.yaml"},
		Digests:       store,
		FindingsFiles: writer,
		Clock:         &tuiFakeClock{},
		Interaction:   &tuiFakeInteraction{},
		ToolVersion:   "test",
	})
	m.height = 40
	m.report = findingsReportFor(exportRunA, exportRunB)
	m.runsScreen = screens.NewRunsScreen(m.report, m.width, m.height, stylesFromTheme(m.theme))
	m.screen = screenRuns
	return m
}

// perRunScreens are the per-run views the findings overlay and its export are in.
func perRunScreens() map[string]screens.ScreenID {
	return map[string]screens.ScreenID{
		"dispatches": screens.ScreenRunDetail, "overview": screens.ScreenOverview, "agent types": screens.ScreenAgentTypes,
		"status": screens.ScreenStatus, "tools": screens.ScreenTools, "timeline": screens.ScreenTimeline,
		"context": screens.ScreenContext,
	}
}

// inRunWithOverlay opens exportRunB in the view and opens its findings overlay.
func inRunWithOverlay(t *testing.T, m Model, id screens.ScreenID) Model {
	t.Helper()
	m = press(m.openRunView(exportRunB, id), screens.KeyFindings)
	if !findingsOpen(m.runView) {
		t.Fatalf("setup: f in the %q view must open the findings overlay", id)
	}
	return m
}

// onRunsWithOverlay opens the findings overlay of the runs list.
func onRunsWithOverlay(t *testing.T, m Model) Model {
	t.Helper()
	return requireRunsOverlayOpen(t, m)
}

// submitPath presses Enter on the open prompt and runs the resulting command
// (the asynchronous write), feeding its message back into the model.
func submitPath(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := pressKeyType(m, tea.KeyEnter)
	return runCmd(t, asModel(updated), cmd)
}

// confirmOverwrite presses y on the overwrite question and runs the resulting
// command (the asynchronous write), feeding its message back into the model.
func confirmOverwrite(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := pressKey(m, "y")
	return runCmd(t, asModel(updated), cmd)
}

// errDiskFull is a write failure other than "file exists".
var errDiskFull = errors.New("disk full")
