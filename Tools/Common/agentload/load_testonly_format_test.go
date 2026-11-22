package agentload_test

// load_testonly_format_test.go proves a format registered test-only loads through
// the same entry point, with no loader change.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentformat/testjson"
)

const testJSONAgent = `{"mosaic_test_agent":1,"instructions":"You review code.\n","meta":[["name","reviewer"],["model","sonnet"]]}`

func TestLoad_TestOnlyFormat_LoadsThroughSameEntryPoint(t *testing.T) {
	testjson.Register()
	path := filepath.Join(t.TempDir(), "reviewer.json")
	if err := os.WriteFile(path, []byte(testJSONAgent), 0o644); err != nil {
		t.Fatal(err)
	}

	a := mustLoad(t, path, string(testjson.ID))

	if got := scalar(t, a, "name"); got != "reviewer" {
		t.Errorf("name = %q, want reviewer", got)
	}
	if got := scalar(t, a, "model"); got != "sonnet" {
		t.Errorf("model = %q, want sonnet", got)
	}
	if !strings.Contains(string(a.Canonical), "You review code.") {
		t.Errorf("Canonical does not carry the instructions: %q", a.Canonical)
	}
	if a.FormatID != string(testjson.ID) {
		t.Errorf("FormatID = %q, want %q", a.FormatID, testjson.ID)
	}
	if a.Settings != nil {
		t.Errorf("Settings = %v, want nil for a format without a settings reader", a.Settings)
	}
}
