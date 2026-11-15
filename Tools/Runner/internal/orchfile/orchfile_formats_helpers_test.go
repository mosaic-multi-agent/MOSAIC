package orchfile_test

// Helpers for reading orchestrator scripts deployed in a format other than
// Markdown. The test-only JSON agent format is a format the Runner has no code
// for: the script body is carried verbatim in the file's instructions member.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentformat/testjson"

	"mosaic-run/internal/agentfile"
)

// testJSONFormat is the format data of a harness whose agent files are the
// test-only JSON format.
var testJSONFormat = agentfile.Format{
	AgentsDir: ".fmt/agents",
	FormatID:  string(testjson.ID),
	Extension: ".json",
}

// useTestJSON makes the test-only JSON format loadable and returns its Format.
func useTestJSON(t *testing.T) agentfile.Format {
	t.Helper()
	testjson.Register()
	return testJSONFormat
}

// markdownBody returns the text of a Markdown script after its frontmatter
// block (the whole text when it has none).
func markdownBody(text string) string {
	const fence = "---\n"
	if !strings.HasPrefix(text, fence) {
		return text
	}
	rest := text[len(fence):]
	end := strings.Index(rest, "\n"+fence)
	if end < 0 {
		return text
	}
	return rest[end+len("\n"+fence):]
}

// testJSONFileText renders an orchestrator script with the given body as a
// test-only JSON agent file.
func testJSONFileText(t *testing.T, body string) []byte {
	t.Helper()
	instructions, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal script body: %v", err)
	}
	return []byte(`{"mosaic_test_agent":1,"instructions":` + string(instructions) +
		`,"meta":[["name","orchestrator-script"],["model","m1"]]}`)
}

// writeAsTestJSON writes the script at markdownPath as a test-only JSON agent
// file in a temporary directory and returns the new path. The two files hold
// the same script body.
func writeAsTestJSON(t *testing.T, markdownPath string) string {
	t.Helper()
	data, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read %s: %v", markdownPath, err)
	}
	path := filepath.Join(t.TempDir(), "orchestrator-script.json")
	if err := os.WriteFile(path, testJSONFileText(t, markdownBody(string(data))), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// writeScriptFile writes raw text as the orchestrator script file with the
// given name in a temporary directory and returns its path.
func writeScriptFile(t *testing.T, name, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// requireNamesLoadFailure asserts the refusal identifies the loader failure:
// either through its cause chain or by quoting the failure text in its Reason.
func requireNamesLoadFailure(t *testing.T, op string, err error, sentinel error) {
	t.Helper()
	re := asRefusalError(t, err)
	if errors.Is(err, sentinel) {
		return
	}
	if !strings.Contains(re.Reason, sentinel.Error()) {
		t.Errorf("%s: want the refusal to name the load failure %q, got Reason %q", op, sentinel.Error(), re.Reason)
	}
}
