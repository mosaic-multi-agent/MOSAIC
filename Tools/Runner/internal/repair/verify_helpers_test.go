package repair_test

// Builders for the repair-verification tests. The repaired artifact is derived
// from the incident-shaped fixture (83 Execution Log rows, row 77 split over
// three lines); dispatch logs are generated as JSONL in the shape the Logger
// writes. Everything lives in temp dirs; nothing outside the tool is read.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/repair"
)

const (
	splitRowFirstLine  = "| 77 | test-writer-tdd#77 | EXECUTION | 1.Test | 2 | SUCCESS | 2026-11-06T14:17:00Z | Tests written |"
	repairedRow77      = "| 77 | test-writer-tdd#77 | EXECUTION | 1.Test | 2 | SUCCESS | 2026-11-06T14:17:00Z | Tests written | - | - |"
	strayContinuation  = " continuation of the split row: the note text the orchestrator inserted"
	lastAgentLine      = `  last_agent: "test-writer-tdd#83"`
	lastStatusLine     = "  last_status: SUCCESS"
	errorCodeLine      = "  error_code: null"
	incidentNoteAnchor = "| 1 | Retry crash incident reproduction |"
)

var rowAgentPattern = regexp.MustCompile(`^\| (\d+) \| (\S+#\d+) \|`)

// incidentDamaged returns the incident fixture exactly as the orchestrator left it.
func incidentDamaged(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "artifact", "damaged_incident_split_row.md"))
	if err != nil {
		t.Fatalf("reading incident fixture: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// incidentRepaired is the incident fixture repaired as the orchestrator is
// asked to: row 77 whole again, the stray text moved to Workflow Notes, all 83
// rows kept.
func incidentRepaired(t *testing.T) string {
	t.Helper()
	lines := strings.Split(incidentDamaged(t), "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		switch {
		case lines[i] == splitRowFirstLine:
			out = append(out, repairedRow77)
			i += 2 // the second row fragment and the stray continuation line
		case lines[i] == incidentNoteAnchor:
			out = append(out, lines[i], "| 2 | "+strings.TrimSpace(strayContinuation)+" |")
		default:
			out = append(out, lines[i])
		}
	}
	doc := strings.Join(out, "\n")
	requireStrictlyReadable(t, doc)
	return doc
}

// requireStrictlyReadable fails the test setup when doc is not a clean artifact.
func requireStrictlyReadable(t *testing.T, doc string) {
	t.Helper()
	if _, err := artifact.Parse([]byte(doc)); err != nil {
		t.Fatalf("test setup: artifact should read strictly, got %v", err)
	}
}

// rowAgents returns the Agent cell of every Execution Log row in file order.
func rowAgents(doc string) []string {
	var agents []string
	for _, line := range strings.Split(doc, "\n") {
		if m := rowAgentPattern.FindStringSubmatch(line); m != nil {
			agents = append(agents, m[2])
		}
	}
	return agents
}

// rowLineNumber is the 1-based line of the Execution Log row with this Seq.
func rowLineNumber(t *testing.T, doc string, seq int) int {
	t.Helper()
	prefix := "| " + strconv.Itoa(seq) + " | "
	for i, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, prefix) && rowAgentPattern.MatchString(line) {
			return i + 1
		}
	}
	t.Fatalf("test setup: no Execution Log row %d", seq)
	return 0
}

// withRowCell replaces one cell (0-based column index) of the row with this Seq.
func withRowCell(t *testing.T, doc string, seq, column int, value string) string {
	t.Helper()
	line := rowLineNumber(t, doc, seq) - 1
	lines := strings.Split(doc, "\n")
	cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(lines[line], "| "), " |"), " | ")
	cells[column] = value
	lines[line] = "| " + strings.Join(cells, " | ") + " |"
	doc = strings.Join(lines, "\n")
	requireStrictlyReadable(t, doc)
	return doc
}

// withoutRow removes the row with this Seq.
func withoutRow(t *testing.T, doc string, seq int) string {
	t.Helper()
	line := rowLineNumber(t, doc, seq) - 1
	lines := strings.Split(doc, "\n")
	doc = strings.Join(append(lines[:line:line], lines[line+1:]...), "\n")
	requireStrictlyReadable(t, doc)
	return doc
}

// withUpTo keeps only the Execution Log rows with Seq <= seq and makes the
// last of them the last_agent, as a copy written after that step would be.
func withUpTo(t *testing.T, doc string, seq int) string {
	t.Helper()
	var out []string
	lastAgent := ""
	for _, line := range strings.Split(doc, "\n") {
		if m := rowAgentPattern.FindStringSubmatch(line); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > seq {
				continue
			}
			lastAgent = m[2]
		}
		out = append(out, line)
	}
	doc = strings.Join(out, "\n")
	if lastAgent != "" {
		doc = strings.Replace(doc, lastAgentLine+"\n", "  last_agent: \""+lastAgent+"\"\n", 1)
	}
	requireStrictlyReadable(t, doc)
	return doc
}

// withRowsAfter inserts Execution Log rows right after the row with this Seq,
// leaving current_state untouched, as steps that do not move it would.
func withRowsAfter(t *testing.T, doc string, seq int, rows ...string) string {
	t.Helper()
	at := rowLineNumber(t, doc, seq)
	lines := strings.Split(doc, "\n")
	out := append([]string{}, lines[:at]...)
	out = append(out, rows...)
	out = append(out, lines[at:]...)
	doc = strings.Join(out, "\n")
	requireStrictlyReadable(t, doc)
	return doc
}

// withLine replaces a whole line that must exist exactly once.
func withLine(t *testing.T, doc, old, replacement string) string {
	t.Helper()
	if strings.Count(doc, old+"\n") != 1 {
		t.Fatalf("test setup: line %q not found exactly once", old)
	}
	doc = strings.Replace(doc, old+"\n", replacement+"\n", 1)
	requireStrictlyReadable(t, doc)
	return doc
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// verifyDoc runs Verify on doc written as the run's artifact.
func verifyDoc(t *testing.T, doc, lastGood, dispatchLog string) repair.Verdict {
	t.Helper()
	store := artifact.NewFileStore(writeFile(t, "Orchestration.md", doc))
	return repair.Verify(repair.VerifyInput{Store: store, LastGoodCopy: lastGood, DispatchLog: dispatchLog})
}

// ---- dispatch-log builders ----

func jsonLine(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal log line: %v", err)
	}
	return string(b)
}

func versionLine(t *testing.T) string {
	return jsonLine(t, map[string]any{"type": "version", "tool_version": "1.12.0"})
}

func requestLine(t *testing.T, id string) string {
	return jsonLine(t, map[string]any{"type": "request", "request": map[string]any{"agent_instance_id": id, "task_description": "t"}})
}

func responseLine(t *testing.T, id string, status domain.StatusCode, code domain.ErrorCode) string {
	resp := map[string]any{"agent_instance_id": id, "status_code": string(status), "status_message": "m"}
	if code != domain.ErrorNone {
		resp["error_code"] = string(code)
	}
	return jsonLine(t, map[string]any{"type": "response", "agent_instance_id": id, "response": resp})
}

func harnessErrorLine(t *testing.T, id string) string {
	return jsonLine(t, map[string]any{
		"type": "error", "agent_instance_id": id, "error": "timed out",
		"status": "HARNESS_ERROR", "failure_kind": "timeout",
	})
}

// successResponses is one SUCCESS response per agent id, in order.
func successResponses(t *testing.T, ids []string) []string {
	lines := make([]string, len(ids))
	for i, id := range ids {
		lines[i] = responseLine(t, id, domain.StatusSUCCESS, domain.ErrorNone)
	}
	return lines
}

func writeLog(t *testing.T, lines ...string) string {
	t.Helper()
	return writeFile(t, "run-dispatch.log", strings.Join(lines, "\n")+"\n")
}

// ---- verdict assertions ----

func requireAccepted(t *testing.T, v repair.Verdict) {
	t.Helper()
	if !v.Accepted {
		t.Fatalf("want the repair accepted, got problems %v", v.Problems)
	}
	if len(v.Problems) != 0 {
		t.Errorf("an accepted verdict must carry no problems, got %v", v.Problems)
	}
}

// requireRejectedAt fails unless the verdict is a rejection naming a problem in
// section; a negative line accepts any line, otherwise the problem must be at that line.
func requireRejectedAt(t *testing.T, v repair.Verdict, section string, line int) {
	t.Helper()
	if v.Accepted {
		t.Fatalf("want the repair rejected (a problem in %s), got accepted", section)
	}
	if len(v.Problems) == 0 {
		t.Fatal("a rejection must carry at least one problem")
	}
	for _, p := range v.Problems {
		if p.String() == "" {
			t.Errorf("problem %+v renders empty: unusable as a damage report", p)
		}
		if p.Section == section && (line < 0 || p.Line == line) {
			return
		}
	}
	t.Fatalf("want a problem in %s at line %d, got %v", section, line, v.Problems)
}
