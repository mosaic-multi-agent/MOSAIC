package cli_test

// End-to-end content check: a digest exported from logs whose content fields
// all hold marker strings must contain none of them.

import (
	"strings"
	"testing"
)

var e2eContentMarkers = []string{
	"LEAK_PROMPT",
	"LEAK_RESPONSE",
	"LEAK_TURN_USER",
	"LEAK_TURN_ASSISTANT",
	"LEAK_TURN_AGENT",
	"LEAK_TOOL_INPUT",
	"LEAK_TOOL_INPUT_OBJECT",
	"LEAK_TOOL_OUTPUT",
	"LEAK_TOOL_OUTPUT_OBJECT",
	"LEAK_TOOL_ERROR",
	"LEAK_TOOL_ERROR_AGENT",
	"LEAK_NOTIFICATION",
	"LEAK_NOTIFICATION_AGENT",
	"LEAK_CWD_RUN",
	"LEAK_CWD_SESSION",
}

func TestDigestExport_ContainsNoContentMarker(t *testing.T) {
	digestPath := e2eExport(t, e2eFixture("marker"), "")

	content := e2eReadFile(t, digestPath)

	if !strings.Contains(content, "20261022T090000Z-b001") {
		t.Fatalf("the digest must hold the marker run; got %d bytes", len(content))
	}
	for _, m := range e2eContentMarkers {
		if strings.Contains(content, m) {
			t.Errorf("digest contains content marker %q", m)
		}
	}
	if strings.Contains(content, "LEAK_") {
		t.Error("digest contains an unlisted content marker (LEAK_ prefix)")
	}
}

func TestDigestExport_MarkerRunStillReportsItsFacts(t *testing.T) {
	digestPath := e2eExport(t, e2eFixture("marker"), "")

	runs := e2eRunsOf(e2eReport(t, digestPath))

	run := runs["20261022T090000Z-b001"]
	if run == nil {
		t.Fatalf("marker run missing from the digest report; got %v", runs)
	}
	if v := e2eTokens(run, "input"); v != float64(105) {
		t.Errorf("marker run input = %v, want 105 (orchestrator 100 plus worker 5)", v)
	}
}
