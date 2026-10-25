package digest_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// allowedKeys is the digest format's key allowlist, per object kind.
var allowedKeys = map[string][]string{
	"top": {"format", "digest_version", "tool_version", "written_at", "runs", "unattributable", "findings"},
	"run": {"run_id", "provisional", "harnesses", "schema_versions", "adapter_version", "orchestrator_model",
		"orchestrator_provider", "started_at", "ended_at", "outcome", "sessions", "orchestrator", "agents",
		"invocations", "turns", "notifications", "compactions"},
	"session":      {"session_id", "started_at", "ended_at", "resumed", "reason", "model", "provider"},
	"orchestrator": {"usage", "tool_calls"},
	"agent":        {"instance_id", "agent_type", "usage"},
	"invocation": {"instance_id", "agent_type", "state", "status_code", "model", "provider", "started_at",
		"ended_at", "end_events", "tool_calls"},
	"usage": {"record_id", "at", "model", "provider", "effort", "service_tier", "harness", "tokens"},
	"tokens": {"input_tokens", "cache_read_tokens", "cache_creation_tokens", "cache_creation_5m_tokens",
		"cache_creation_1h_tokens", "output_tokens", "reasoning_tokens"},
	"tool_call":    {"call_id", "tool_name", "status", "started_at", "ended_at", "pairing"},
	"turn":         {"actor", "role", "at"},
	"notification": {"actor", "at", "notification_type", "requires_response", "resolution", "notification_id"},
	"compaction":   {"actor", "at", "trigger", "tokens_before", "tokens_after"},
	"actor":        {"kind", "instance_id", "agent_type"},
	"finding":      {"kind", "severity", "scope", "run_id", "actor"},
}

// childKinds maps (object kind, key) to the kind of the nested object(s).
var childKinds = map[string]map[string]string{
	"top":          {"runs": "run", "unattributable": "run", "findings": "finding"},
	"run":          {"sessions": "session", "orchestrator": "orchestrator", "agents": "agent", "invocations": "invocation", "turns": "turn", "notifications": "notification", "compactions": "compaction"},
	"orchestrator": {"usage": "usage", "tool_calls": "tool_call"},
	"agent":        {"usage": "usage"},
	"invocation":   {"tool_calls": "tool_call"},
	"usage":        {"tokens": "tokens"},
	"turn":         {"actor": "actor"},
	"notification": {"actor": "actor"},
	"compaction":   {"actor": "actor"},
	"finding":      {"actor": "actor"},
}

// checkKeys fails for every key under obj that is not in the allowlist for kind.
func checkKeys(t *testing.T, path, kind string, obj map[string]any) {
	t.Helper()
	allowed := map[string]bool{}
	for _, k := range allowedKeys[kind] {
		allowed[k] = true
	}
	for key, val := range obj {
		if !allowed[key] {
			t.Errorf("%s: key %q is not in the digest format", path, key)
			continue
		}
		child, nested := childKinds[kind][key]
		if !nested {
			continue
		}
		switch v := val.(type) {
		case map[string]any:
			checkKeys(t, path+"."+key, child, v)
		case []any:
			for i, el := range v {
				if m, ok := el.(map[string]any); ok {
					checkKeys(t, fmt.Sprintf("%s.%s[%d]", path, key, i), child, m)
				}
			}
		}
	}
}

func TestEncode_FullDigestHasOnlyFormatNamedKeys(t *testing.T) {
	path := writeDigest(t, fullDigest())

	checkKeys(t, "digest", "top", readJSONObject(t, path))
}

func TestEncode_FindingDetailAndPathNeverAppearInFile(t *testing.T) {
	const detailMarker, pathMarker = "DETAIL-MARKER-91f3", "PATH-MARKER-77c2"
	d := fullDigest()
	d.Findings = []domain.Finding{
		{
			Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Run: domain.NamedRun(runA),
			Path: "C:/logs/" + pathMarker + "/events.jsonl", Line: 31337, Detail: "secret " + detailMarker,
		},
		{Kind: domain.FindingUnreadableFile, Severity: domain.SeverityWarning, Path: "/etc/" + pathMarker, Detail: detailMarker},
	}
	path := writeDigest(t, d)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	text := string(raw)
	for _, marker := range []string{detailMarker, pathMarker, "31337", "events.jsonl"} {
		if strings.Contains(text, marker) {
			t.Errorf("file leaks %q", marker)
		}
	}
	checkKeys(t, "digest", "top", readJSONObject(t, path))
}

func TestEncode_FileHasNoContentPathMoneyOrUnknownRunCountKeys(t *testing.T) {
	path := writeDigest(t, fullDigest())
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	text := string(raw)
	for _, key := range []string{"prompt", "response", "content", "tool_input", "tool_output", "message",
		"cwd", "path", "detail", "line", "cost", "price", "money", "usd", "merged", "residual"} {
		if strings.Contains(text, `"`+key+`"`) {
			t.Errorf("file contains forbidden key %q", key)
		}
	}
}

func TestEncode_FindingScopeAndRunIdFollowRunRef(t *testing.T) {
	d := domain.Digest{Findings: []domain.Finding{
		{Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Run: domain.NamedRun(runA)},
		{Kind: domain.FindingMissingModel, Severity: domain.SeverityInfo, Run: domain.UnattributableRun()},
		{Kind: domain.FindingUnrecognisedFolder, Severity: domain.SeverityWarning},
	}}
	path := writeDigest(t, d)

	doc := readJSONObject(t, path)

	findings, _ := doc["findings"].([]any)
	if len(findings) != 3 {
		t.Fatalf("findings = %v", doc["findings"])
	}
	want := []struct{ scope, severity, runID string }{{"run", "warning", runA}, {"unattributable", "info", ""}, {"general", "warning", ""}}
	for i, w := range want {
		f := findings[i].(map[string]any)
		if f["scope"] != w.scope || f["severity"] != w.severity {
			t.Errorf("finding %d scope/severity = %v/%v, want %s/%s", i, f["scope"], f["severity"], w.scope, w.severity)
		}
		runID, has := f["run_id"]
		if (w.runID == "") == has || (has && runID != w.runID) {
			t.Errorf("finding %d run_id = %v (present=%v), want %q", i, runID, has, w.runID)
		}
	}
}
