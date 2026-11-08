package dispatchlog

// Tests of how the dispatch log shows an invocation that ended without a usable
// subagent response (a harness error) next to a subagent-returned BLOCKED.

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestLogger_LogHarnessError_EntryCarriesStatusAndFailureKind(t *testing.T) {
	logger := New(t.TempDir())
	logger.SetRunID(validRunID)

	logger.LogHarnessError("agent#1", domain.HarnessFailure{Kind: domain.HarnessFailureTimeout, Detail: "stalled"}, "harness timed out")
	logger.Close()

	lines := readLogLines(t, logger)
	if len(lines) != 1 {
		t.Fatalf("want one entry, got %d: %v", len(lines), lines)
	}
	m := unmarshalLine(t, lines[0])
	if m["status"] != "HARNESS_ERROR" {
		t.Errorf("status = %v, want HARNESS_ERROR", m["status"])
	}
	if m["failure_kind"] != "timeout" {
		t.Errorf("failure_kind = %v, want timeout", m["failure_kind"])
	}
}

func TestLogger_LogHarnessError_KeepsTheErrorEntryShape(t *testing.T) {
	logger := New(t.TempDir())
	logger.SetRunID(validRunID)

	logger.LogHarnessError("agent#7", domain.HarnessFailure{Kind: domain.HarnessFailureProcess}, "exit status 2")
	logger.Close()

	m := unmarshalLine(t, readLogLines(t, logger)[0])
	if m["type"] != "error" || m["agent_instance_id"] != "agent#7" || m["error"] != "exit status 2" {
		t.Errorf("entry = %v, want type error, agent_instance_id agent#7, error text kept", m)
	}
	if ts, _ := m["timestamp"].(string); ts == "" {
		t.Errorf("entry lacks a timestamp: %v", m)
	}
}

func TestLogger_LogError_StaysFreeOfStatusAndFailureKind(t *testing.T) {
	logger := New(t.TempDir())
	logger.SetRunID(validRunID)

	logger.LogError("agent#1", "consultation transport failed")
	logger.Close()

	m := unmarshalLine(t, readLogLines(t, logger)[0])
	if _, ok := m["status"]; ok {
		t.Errorf("LogError entry carries status: %v", m)
	}
	if _, ok := m["failure_kind"]; ok {
		t.Errorf("LogError entry carries failure_kind: %v", m)
	}
}

func TestLogger_SubagentReturnedBlocked_IsLoggedAsResponseNotAsHarnessError(t *testing.T) {
	logger := New(t.TempDir())
	logger.SetRunID(validRunID)

	logger.LogResponse(domain.ProtocolResponse{
		AgentInstanceID: "agent#1", StatusCode: domain.StatusBLOCKED,
		ErrorCode: domain.ErrorTOOL_UNAVAILABLE, StatusMessage: "tool missing",
	})
	logger.Close()

	m := unmarshalLine(t, readLogLines(t, logger)[0])
	if m["type"] != "response" {
		t.Errorf("type = %v, want response", m["type"])
	}
	if _, ok := m["failure_kind"]; ok {
		t.Errorf("response entry carries failure_kind: %v", m)
	}
	resp, _ := m["response"].(map[string]interface{})
	if resp["status_code"] != "BLOCKED" {
		t.Errorf("response status_code = %v, want BLOCKED", resp["status_code"])
	}
}
