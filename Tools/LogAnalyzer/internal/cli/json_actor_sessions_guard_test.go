package cli_test

// The actor session figures (findings and tool figures per actor) belong to the
// Session screen only: the report JSON is the same with or without them.

import (
	"reflect"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestReportJSON_ActorSessionsAreNotSerialised(t *testing.T) {
	plain := vwRun()
	withSessions := vwRun()
	withSessions.ActorSessions = []domain.ActorSession{{
		Actor:    domain.Orchestrator(),
		Findings: []domain.ActorFinding{{Kind: domain.ActorFindingFailedTool, Tool: "Bash", Count: 3}},
		Tools:    []domain.ToolFigures{{Tool: "Bash", Calls: 3, Errors: 3}},
	}}

	want := decodeReportJSON(t, vwReport(plain))
	got := decodeReportJSON(t, vwReport(withSessions))

	if !reflect.DeepEqual(want, got) {
		t.Errorf("the report JSON changes when the run holds actor sessions\nwithout: %v\nwith:    %v", want, got)
	}
}
