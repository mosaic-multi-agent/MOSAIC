package screens_test

// Behaviour of the mark key of the tools view: the selected row's tool and
// harnesses become a pending request, rows that cannot be marked are refused
// with a status message, and the key is listed in the help and the overlay.

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/tui/screens"
)

const cannotMarkText = "cannot mark: tool name or harness unknown"

// markKeyEntry matches the mark key standing alone as a help entry key.
var markKeyEntry = regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(screens.KeyMarkUserTool) + `\s`)

func markTool(name string, calls int, harnesses ...domain.Harness) domain.ToolFigures {
	return domain.ToolFigures{Tool: name, Calls: calls, Harnesses: harnesses}
}

func newMarkScreen(t *testing.T, tools ...domain.ToolFigures) *screens.ToolsScreen {
	t.Helper()
	colourOff(t)
	return screens.NewToolsScreen(runWithTools(tools...), tableW, tallH, paletteStyles())
}

func TestToolsMark_MarkKeyRequestsTheSelectedToolWithItsHarness(t *testing.T) {
	s := newMarkScreen(t,
		markTool("Alpha", 20, domain.HarnessClaudeCode),
		markTool("Beta", 5, domain.HarnessGHCPCLI))

	press(s, screens.KeyMarkUserTool)

	tool, harnesses, ok := s.MarkRequested()
	if !ok || tool != "Alpha" || !reflect.DeepEqual(harnesses, []domain.Harness{domain.HarnessClaudeCode}) {
		t.Errorf("MarkRequested() = %q, %v, %v; want Alpha, [claude-code], true", tool, harnesses, ok)
	}
}

func TestToolsMark_RequestFollowsTheCursor(t *testing.T) {
	s := newMarkScreen(t,
		markTool("Alpha", 20, domain.HarnessClaudeCode),
		markTool("Beta", 5, domain.HarnessGHCPCLI))

	press(s, "down", screens.KeyMarkUserTool)

	tool, harnesses, ok := s.MarkRequested()
	if !ok || tool != "Beta" || !reflect.DeepEqual(harnesses, []domain.Harness{domain.HarnessGHCPCLI}) {
		t.Errorf("MarkRequested() = %q, %v, %v; want Beta, [ghcp-cli], true", tool, harnesses, ok)
	}
}

func TestToolsMark_RequestFollowsTheActiveSort(t *testing.T) {
	// Alpha leads under the calls sort, Beta under the duration sort.
	alpha := markTool("Alpha", 20, domain.HarnessClaudeCode)
	alpha.Durations = durationStats(20, time.Second, ms(50), ms(50), ms(50))
	beta := markTool("Beta", 5, domain.HarnessGHCPCLI)
	beta.Durations = durationStats(5, 50*time.Second, 10*time.Second, 10*time.Second, 10*time.Second)
	s := newMarkScreen(t, alpha, beta)
	selected := func() string {
		got, _ := s.SelectedTool()
		return got.Tool
	}
	before := selected()

	press(s, screens.KeySort)
	if after := selected(); after == before {
		t.Fatalf("setup: the sort key must change the row under the cursor, it stayed %q", after)
	}
	press(s, screens.KeyMarkUserTool)

	tool, harnesses, ok := s.MarkRequested()
	if !ok || tool != "Beta" || !reflect.DeepEqual(harnesses, []domain.Harness{domain.HarnessGHCPCLI}) {
		t.Errorf("MarkRequested() = %q, %v, %v; want Beta, [ghcp-cli], true (Alpha was first before sorting)",
			tool, harnesses, ok)
	}
}

func TestToolsMark_RowWithSeveralHarnessesRequestsAllOfThem(t *testing.T) {
	s := newMarkScreen(t, markTool("Read", 4, domain.HarnessClaudeCode, domain.HarnessGHCPCLI))

	press(s, screens.KeyMarkUserTool)

	_, harnesses, ok := s.MarkRequested()
	want := []domain.Harness{domain.HarnessClaudeCode, domain.HarnessGHCPCLI}
	if !ok || !reflect.DeepEqual(harnesses, want) {
		t.Errorf("MarkRequested() harnesses = %v (ok %v), want %v", harnesses, ok, want)
	}
}

func TestToolsMark_NoRequestUntilTheKeyIsPressed(t *testing.T) {
	s := newMarkScreen(t, markTool("Alpha", 20, domain.HarnessClaudeCode))

	if _, _, ok := s.MarkRequested(); ok {
		t.Error("MarkRequested() reported a request before any key was pressed")
	}
}

func TestToolsMark_ResetMarkClearsTheRequest(t *testing.T) {
	s := newMarkScreen(t, markTool("Alpha", 20, domain.HarnessClaudeCode))
	press(s, screens.KeyMarkUserTool)
	if _, _, ok := s.MarkRequested(); !ok {
		t.Fatal("setup: the mark key set no request")
	}

	s.ResetMark()

	if _, _, ok := s.MarkRequested(); ok {
		t.Error("MarkRequested() still reports a request after ResetMark")
	}
}

func TestToolsMark_UnnamedToolIsRefusedWithAStatusMessage(t *testing.T) {
	s := newMarkScreen(t, markTool("", 3, domain.HarnessClaudeCode))

	press(s, screens.KeyMarkUserTool)

	if _, _, ok := s.MarkRequested(); ok {
		t.Error("a row with no tool name must not produce a request")
	}
	assertContains(t, stripStyling(s.View(tableW, tallH)), cannotMarkText)
}

func TestToolsMark_RowWithoutHarnessIsRefusedWithAStatusMessage(t *testing.T) {
	s := newMarkScreen(t, markTool("Mystery", 3))

	press(s, screens.KeyMarkUserTool)

	if _, _, ok := s.MarkRequested(); ok {
		t.Error("a row with no harness must not produce a request")
	}
	assertContains(t, stripStyling(s.View(tableW, tallH)), cannotMarkText)
}

func TestToolsMark_RunWithoutToolsHasNothingToMark(t *testing.T) {
	s := newMarkScreen(t)

	press(s, screens.KeyMarkUserTool)

	if _, _, ok := s.MarkRequested(); ok {
		t.Error("a run without tool calls must not produce a request")
	}
}

func TestToolsMark_KeyIsListedInTheHelpLine(t *testing.T) {
	s := newMarkScreen(t, markTool("Alpha", 20, domain.HarnessClaudeCode))

	if !markKeyEntry.MatchString(s.Help()) {
		t.Errorf("help line %q must list the key %q as its own entry", s.Help(), screens.KeyMarkUserTool)
	}
}

func TestToolsMark_KeyIsListedInTheHelpOverlayWithItsPurpose(t *testing.T) {
	s := newMarkScreen(t, markTool("Alpha", 20, domain.HarnessClaudeCode))

	overlay := stripStyling(openOverlay(s, 100, 40))

	if !markKeyEntry.MatchString(overlay) {
		t.Errorf("overlay must list the key %q as its own entry\n%s", screens.KeyMarkUserTool, overlay)
	}
	assertContains(t, overlay, "user-interaction")
}
