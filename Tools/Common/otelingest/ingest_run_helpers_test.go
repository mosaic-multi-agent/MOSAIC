package otelingest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"
)

const testSession = "vsc-session-6"

// ingestRun is a throw-away run folder plus an OTel file path for tests that do not use a conformance case.
type ingestRun struct {
	runDir   string
	otelPath string
}

// newIngestRun creates a run folder whose orchestrator stream carries one session, so root chat spans of that
// conversation are attributed to the orchestrator.
func newIngestRun(t *testing.T) *ingestRun {
	t.Helper()
	tmp := t.TempDir()
	r := &ingestRun{
		runDir:   filepath.Join(tmp, "OrchestrationLogs", "20261028T000000Z-0000"),
		otelPath: filepath.Join(tmp, "otel", "copilot-otel.jsonl"),
	}
	if err := os.MkdirAll(r.runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	start := fmt.Sprintf(`{"schema_version":"1.0.0","event":"session_start","timestamp":"2026-10-28T08:00:00.000Z","harness":"vscode-ghcp","session_id":%q,"run_id":"20261028T000000Z-0000"}`+"\n", testSession)
	if err := os.WriteFile(r.orchestratorLog(), []byte(start), 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *ingestRun) orchestratorLog() string {
	return filepath.Join(r.runDir, "00_orchestrator_events.jsonl")
}

func (r *ingestRun) statePath() string { return filepath.Join(r.runDir, ".otel-ingest", "state.json") }

func (r *ingestRun) lockPath() string { return filepath.Join(r.runDir, ".otel-ingest", "lock") }

// writeOTel replaces the OTel file with the given complete lines.
func (r *ingestRun) writeOTel(t *testing.T, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(r.otelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, l := range lines {
		data = append(data, l...)
		data = append(data, '\n')
	}
	if err := os.WriteFile(r.otelPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeLock creates a lock file with the given content.
func (r *ingestRun) writeLock(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(r.lockPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.lockPath(), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (r *ingestRun) ingest(final bool) otelingest.Result {
	return otelingest.Ingest(otelingest.Request{
		RunDir: r.runDir, OTelPath: r.otelPath, Final: final, Now: fixedNowFunc,
	})
}

func (r *ingestRun) read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return data
}

func (r *ingestRun) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fixedNowFunc() time.Time { return fixedNow }

// chatLine is a VS Code chat span with token usage. A chatSession of "" makes it a span of the orchestrator's own
// session; any other value is the chat session the span ran in (an unknown one keeps the span pending).
func chatLine(id, conv, chatSession string) string {
	cs := ""
	if chatSession != "" {
		cs = fmt.Sprintf(`"copilot_chat.chat_session_id":%q,`, chatSession)
	}
	return fmt.Sprintf(`{"traceId":"0123456789abcdef0123456789abcdef","spanId":%q,"name":"chat","kind":2,"attributes":{"gen_ai.operation.name":"chat",%s"gen_ai.conversation.id":%q,"gen_ai.response.model":"m","gen_ai.usage.input_tokens":10,"gen_ai.usage.output_tokens":1},"status":{"code":0},"events":[]}`, id, cs, conv)
}

func hasDiagnostic(res otelingest.Result, kind diag.Kind) bool {
	for _, d := range res.Diagnostics {
		if d.Kind == kind {
			return true
		}
	}
	return false
}
