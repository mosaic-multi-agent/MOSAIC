package tui

// The TUI reads the orchestrator script through the shared agent loader, with
// the format of the harness the user selected. These tests give a harness
// without a catalog entry the test-only JSON agent format (a format the Runner
// has no code for) and check that each screen reading the script lists what the
// script declares.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-common/agentformat/testjson"

	"mosaic-run/internal/agentfile"
)

const nonMarkdownHarnessID = "tui-json-script"

// useNonMarkdownHarness registers the test-only JSON format for a harness
// without a catalog entry and returns the harness id.
func useNonMarkdownHarness(t *testing.T) string {
	t.Helper()
	testjson.Register()
	unregister := agentfile.RegisterTestOnly(nonMarkdownHarnessID, agentfile.Format{
		AgentsDir: ".tuijson/agents",
		FormatID:  string(testjson.ID),
		Extension: ".json",
	})
	t.Cleanup(unregister)
	return nonMarkdownHarnessID
}

// writeJSONScript writes body as an orchestrator script in the test-only JSON
// agent format and returns the file path.
func writeJSONScript(t *testing.T, body string) string {
	t.Helper()
	instructions, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal script body: %v", err)
	}
	text := `{"mosaic_test_agent":1,"instructions":` + string(instructions) +
		`,"meta":[["name","orchestrator-script"],["model","m1"]]}`
	path := filepath.Join(t.TempDir(), "orchestrator-script.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// jsonScriptWithInfra is a script declaring one workflow and the named
// checkpoint-class infrastructure agents.
func jsonScriptWithInfra(t *testing.T, agents ...string) string {
	t.Helper()
	body := navCoreOrchestratorContent + "\n<InfrastructureAgents type=\"managed\">\n"
	for _, name := range agents {
		body += flowInfraAgent(name, "checkpoint")
	}
	body += "</InfrastructureAgents>\n"
	return writeJSONScript(t, body)
}

func TestSetupFile_NonMarkdownScript_ListsItsWorkflows(t *testing.T) {
	m := newTestModelNewRun()
	m.selections.config.Harness = useNonMarkdownHarness(t)
	m.screen = screenSetupFile
	path := writeJSONScript(t, navCoreOrchestratorContent)

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(path)})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenSetupWorkflow {
		t.Fatalf("screen = %v, want the workflow screen; view:\n%s", m.screen, m.View())
	}
	if len(m.workflows) != 1 || m.workflows[0].Info.ID != "test-workflow" {
		t.Errorf("workflows = %+v, want exactly test-workflow", m.workflows)
	}
}

func TestHistory_NonMarkdownScript_DeclaredInfraAgentsAreKnown(t *testing.T) {
	m := newTestModelNewRun()
	m.selections.config.Harness = useNonMarkdownHarness(t)
	m.selections.orchestratorFile = jsonScriptWithInfra(t, "cp-a")

	set := m.declaredInfraSet()

	if !set.IsInfrastructureAgent("cp-a") {
		t.Errorf("declared infrastructure set lacks cp-a: %v", set)
	}
}

func TestResumeSummary_NonMarkdownScript_NamesDeclaredCheckpointAgents(t *testing.T) {
	settings := recordedSettings()
	settings.InfraClassSelections = nil
	a := fullyRecorded()
	a.settings = settings
	f := newResumeFlow(t, a, flowOrchestrator{})
	f.selectRun()
	f.m.selections.config.Harness = useNonMarkdownHarness(t)
	f.m.selections.orchestratorFile = jsonScriptWithInfra(t, "cp-a", "cp-b")

	f.m.startOrSummarise()

	if f.m.screen != screenSetupSummary {
		t.Fatalf("screen = %v, want the settings summary", f.m.screen)
	}
	for _, name := range []string{"cp-a", "cp-b"} {
		if !f.showing(name) {
			t.Errorf("the summary does not name the declared checkpoint agent %q; view:\n%s", name, f.m.View())
		}
	}
}

