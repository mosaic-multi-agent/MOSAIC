package orchfile_test

// Tests of the trigger vocabulary check made while enumerating the
// infrastructure declaration region: every row of every declared agent must
// name a trigger from the closed vocabulary, matched exactly.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/orchfile"
)

// infraTableHeader is the declaration table header with a Trigger column.
const infraTableHeader = "| Class | Trigger | Param | On Failure | Description |\n" +
	"|-------|---------|-------|------------|-------------|\n"

// infraRow renders one declaration row of the given class and trigger cell.
func infraRow(class, trigger string) string {
	return "| " + class + " | " + trigger + " | - | halt | row |\n"
}

// infraSection renders one InfrastructureAgent region around body.
func infraSection(name, body string) string {
	return "<InfrastructureAgent type=\"core\" name=\"" + name + "\" version=\"1.0.0\">\n" +
		body + "</InfrastructureAgent>\n"
}

// writeInfraOrch writes an orchestrator file holding the given agent regions
// and returns its path.
func writeInfraOrch(t *testing.T, sections ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("<InfrastructureAgents type=\"managed\">\n")
	for _, s := range sections {
		b.WriteString(s)
	}
	b.WriteString("</InfrastructureAgents>\n")
	path := filepath.Join(t.TempDir(), "orchestrator.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write orchestrator file: %v", err)
	}
	return path
}

func TestEnumerateInfrastructureAgents_EveryVocabularyTrigger_Accepted(t *testing.T) {
	body := infraTableHeader +
		infraRow("commit", "INVOCATION_INTERVAL") +
		infraRow("commit", "ITERATION_END") +
		infraRow("commit", "PHASE_END") +
		infraRow("commit", "MANUAL")
	path := writeInfraOrch(t, infraSection("agent-x", body))

	agents, err := orchfile.EnumerateInfrastructureAgents(path)

	if err != nil {
		t.Fatalf("want no error for the four vocabulary triggers, got %v", err)
	}
	if len(agents) != 1 || len(agents[0].Triggers) != 4 {
		t.Fatalf("want one agent with 4 triggers, got %+v", agents)
	}
	want := []string{"INVOCATION_INTERVAL", "ITERATION_END", "PHASE_END", "MANUAL"}
	for i, w := range want {
		if got := agents[0].Triggers[i].Trigger; got != w {
			t.Errorf("trigger %d: want %q, got %q", i, w, got)
		}
	}
}

func TestEnumerateInfrastructureAgents_UnknownTrigger_RefusedNamingAgentAndTrigger(t *testing.T) {
	path := writeInfraOrch(t, infraSection("commit-manager-x", infraTableHeader+infraRow("commit", "WHENEVER")))

	agents, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if agents != nil {
		t.Errorf("want no partial agent list on refusal, got %+v", agents)
	}
	if re.Component != "orchfile" {
		t.Errorf("Component: want orchfile, got %q", re.Component)
	}
	if re.Resource != "commit-manager-x" {
		t.Errorf("Resource: want the agent name, got %q", re.Resource)
	}
	if re.Token != "WHENEVER" {
		t.Errorf("Token: want the offending trigger, got %q", re.Token)
	}
	if !strings.Contains(re.Reason, "commit-manager-x") || !strings.Contains(re.Reason, "WHENEVER") {
		t.Errorf("Reason must name the agent and the trigger, got %q", re.Reason)
	}
}

func TestEnumerateInfrastructureAgents_InvalidTriggerCases_Refused(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
	}{
		{"legacy stage end name", "STAGE_END"},
		{"lowercase vocabulary name", "iteration_end"},
		{"mixed case vocabulary name", "Iteration_End"},
		{"quote wrapped vocabulary name", "\"ITERATION_END\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeInfraOrch(t, infraSection("agent-x", infraTableHeader+infraRow("commit", tc.trigger)))

			_, err := orchfile.EnumerateInfrastructureAgents(path)

			re := asRefusalError(t, err)
			if re.Resource != "agent-x" || re.Token != tc.trigger {
				t.Errorf("want Resource agent-x and Token %q, got Resource %q Token %q", tc.trigger, re.Resource, re.Token)
			}
		})
	}
}

func TestEnumerateInfrastructureAgents_UnknownTriggerOnLaterRow_Refused(t *testing.T) {
	body := infraTableHeader +
		infraRow("commit", "ITERATION_END") +
		infraRow("commit", "MANUAL") +
		infraRow("commit", "EVERY_FULL_MOON")
	path := writeInfraOrch(t, infraSection("agent-x", body))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if re.Token != "EVERY_FULL_MOON" {
		t.Errorf("Token: want EVERY_FULL_MOON, got %q", re.Token)
	}
}

func TestEnumerateInfrastructureAgents_UnknownTriggerOnLaterAgent_Refused(t *testing.T) {
	path := writeInfraOrch(t,
		infraSection("good-agent", infraTableHeader+infraRow("checkpoint", "PHASE_END")),
		infraSection("bad-agent", infraTableHeader+infraRow("commit", "NEVER")))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if re.Resource != "bad-agent" {
		t.Errorf("Resource: want bad-agent, got %q", re.Resource)
	}
}

func TestEnumerateInfrastructureAgents_UnknownTriggerOnRestoreClassAgent_Refused(t *testing.T) {
	path := writeInfraOrch(t, infraSection("restore-agent", infraTableHeader+infraRow("restore", "AT_DAWN")))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if re.Resource != "restore-agent" || re.Token != "AT_DAWN" {
		t.Errorf("want Resource restore-agent and Token AT_DAWN, got %q / %q", re.Resource, re.Token)
	}
}

func TestEnumerateInfrastructureAgents_StageEnd_RefusalGuidesToIterationEndAndRedeploy(t *testing.T) {
	path := writeInfraOrch(t, infraSection("agent-x", infraTableHeader+infraRow("commit", "STAGE_END")))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	reason := strings.ToLower(re.Reason)
	for _, token := range []string{"stage_end", "iteration_end", "redeploy", "agent-x"} {
		if !strings.Contains(reason, token) {
			t.Errorf("Reason must contain %q, got %q", token, re.Reason)
		}
	}
}

func TestEnumerateInfrastructureAgents_BlankTriggerCell_Refused(t *testing.T) {
	body := infraTableHeader + infraRow("commit", "ITERATION_END") + infraRow("commit", "   ")
	path := writeInfraOrch(t, infraSection("agent-x", body))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if re.Resource != "agent-x" {
		t.Errorf("Resource: want agent-x, got %q", re.Resource)
	}
	if re.Token != "" {
		t.Errorf("Token: want empty for a blank trigger, got %q", re.Token)
	}
}

func TestEnumerateInfrastructureAgents_TableWithRowsAndNoTriggerColumn_Refused(t *testing.T) {
	body := "| Class | Param | On Failure | Description |\n" +
		"|-------|-------|------------|-------------|\n" +
		"| commit | - | halt | row |\n"
	path := writeInfraOrch(t, infraSection("agent-x", body))

	_, err := orchfile.EnumerateInfrastructureAgents(path)

	re := asRefusalError(t, err)
	if re.Resource != "agent-x" {
		t.Errorf("Resource: want agent-x, got %q", re.Resource)
	}
	if !strings.Contains(re.Reason, "agent-x") {
		t.Errorf("Reason must name the agent, got %q", re.Reason)
	}
}

func TestEnumerateInfrastructureAgents_SectionWithoutTable_EnumeratesAgentWithNoTriggers(t *testing.T) {
	path := writeInfraOrch(t, infraSection("agent-x", "A section with prose and no table.\n"))

	agents, err := orchfile.EnumerateInfrastructureAgents(path)

	if err != nil {
		t.Fatalf("want no error for a section without a table, got %v", err)
	}
	if len(agents) != 1 || agents[0].Name != "agent-x" || len(agents[0].Triggers) != 0 {
		t.Errorf("want one agent agent-x with no triggers, got %+v", agents)
	}
}

func TestEnumerateInfrastructureAgents_TableWithoutRows_EnumeratesAgentWithNoTriggers(t *testing.T) {
	path := writeInfraOrch(t, infraSection("agent-x", infraTableHeader))

	agents, err := orchfile.EnumerateInfrastructureAgents(path)

	if err != nil {
		t.Fatalf("want no error for a table without rows, got %v", err)
	}
	if len(agents) != 1 || len(agents[0].Triggers) != 0 {
		t.Errorf("want one agent with no triggers, got %+v", agents)
	}
}
