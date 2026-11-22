package orchfile_test

// Reading an orchestrator script in a format other than Markdown yields the
// same workflow and infrastructure-agent regions as the equivalent Markdown
// script. A script that cannot be loaded is refused with the reason named.

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/agentload"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/orchfile"
)

// workflowFixtures are Markdown scripts whose regions the non-Markdown
// equivalent must reproduce.
var workflowFixtures = []string{
	"bare-single.md",
	"bare-two-workflows.md",
	"deployed.md",
	"deploy-managed.md",
	"mixed-workflows.md",
}

func TestEnumerateWorkflows_NonMarkdownScript_YieldsSameRegionsAsMarkdown(t *testing.T) {
	jsonFormat := useTestJSON(t)
	for _, name := range workflowFixtures {
		t.Run(name, func(t *testing.T) {
			mdPath := orchfileFixture(name)
			want, err := orchfile.EnumerateWorkflows(mdPath, agentfile.Format{})
			if err != nil {
				t.Fatalf("Markdown enumeration: %v", err)
			}

			got, err := orchfile.EnumerateWorkflows(writeAsTestJSON(t, mdPath), jsonFormat)

			if err != nil {
				t.Fatalf("EnumerateWorkflows on the non-Markdown script: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("region count: want %d, got %d", len(want), len(got))
			}
			for i := range want {
				if got[i].Info != want[i].Info {
					t.Errorf("region %d Info: want %+v, got %+v", i, want[i].Info, got[i].Info)
				}
				if !bytes.Equal(got[i].Content, want[i].Content) {
					t.Errorf("region %d Content differs\nwant: %q\ngot:  %q", i, want[i].Content, got[i].Content)
				}
			}
		})
	}
}

func TestGetWorkflow_NonMarkdownScript_ReturnsSameRegionAsMarkdown(t *testing.T) {
	jsonFormat := useTestJSON(t)
	mdPath := orchfileFixture("bare-two-workflows.md")
	want, err := orchfile.GetWorkflow(mdPath, agentfile.Format{}, "workflow-beta")
	if err != nil {
		t.Fatalf("Markdown GetWorkflow: %v", err)
	}

	got, err := orchfile.GetWorkflow(writeAsTestJSON(t, mdPath), jsonFormat, "workflow-beta")

	if err != nil {
		t.Fatalf("GetWorkflow on the non-Markdown script: %v", err)
	}
	if got.Info != want.Info || !bytes.Equal(got.Content, want.Content) {
		t.Errorf("region differs\nwant: %+v %q\ngot:  %+v %q", want.Info, want.Content, got.Info, got.Content)
	}
}

func TestGetWorkflow_NonMarkdownScript_AbsentID_RefusedWithStandardReason(t *testing.T) {
	jsonFormat := useTestJSON(t)

	_, err := orchfile.GetWorkflow(writeAsTestJSON(t, orchfileFixture("bare-single.md")), jsonFormat, "does-not-exist")

	re := asRefusalError(t, err)
	if !strings.Contains(re.Reason, orchfile.WorkflowNotFoundReason("does-not-exist")) {
		t.Errorf("Reason: want the standard workflow-not-found text, got %q", re.Reason)
	}
}

func TestEnumerateInfrastructureAgents_NonMarkdownScript_YieldsSameAgentsAsMarkdown(t *testing.T) {
	jsonFormat := useTestJSON(t)
	body := infraTableHeader +
		infraRow("commit", "ITERATION_END") +
		infraRow("commit", "MANUAL")
	mdPath := writeInfraOrch(t,
		infraSection("commit-manager-x", body),
		infraSection("checkpoint-manager-y", infraTableHeader+infraRow("checkpoint", "PHASE_END")))
	want, err := orchfile.EnumerateInfrastructureAgents(mdPath, agentfile.Format{})
	if err != nil {
		t.Fatalf("Markdown enumeration: %v", err)
	}

	got, err := orchfile.EnumerateInfrastructureAgents(writeAsTestJSON(t, mdPath), jsonFormat)

	if err != nil {
		t.Fatalf("EnumerateInfrastructureAgents on the non-Markdown script: %v", err)
	}
	if len(want) != 2 {
		t.Fatalf("fixture sanity: want 2 Markdown agents, got %d", len(want))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("agents differ\nwant: %+v\ngot:  %+v", want, got)
	}
}

func TestEnumerateInfrastructureAgents_NonMarkdownScript_UnknownTrigger_RefusedAsInMarkdown(t *testing.T) {
	jsonFormat := useTestJSON(t)
	mdPath := writeInfraOrch(t, infraSection("agent-x", infraTableHeader+infraRow("commit", "WHENEVER")))

	_, err := orchfile.EnumerateInfrastructureAgents(writeAsTestJSON(t, mdPath), jsonFormat)

	re := asRefusalError(t, err)
	if re.Resource != "agent-x" || re.Token != "WHENEVER" {
		t.Errorf("want Resource agent-x and Token WHENEVER, got Resource %q Token %q", re.Resource, re.Token)
	}
}

// undecodableScripts are files the test-only JSON format cannot decode.
var undecodableScripts = map[string]string{
	"not json":              "<Workflow type=\"core\" name=\"w\" version=\"1\">x</Workflow>\n",
	"marker missing":        `{"instructions":"body","meta":[["name","o"]]}`,
	"unknown member":        `{"mosaic_test_agent":1,"instructions":"b","meta":[],"extra":1}`,
	"instructions not text": `{"mosaic_test_agent":1,"instructions":7,"meta":[]}`,
}

func TestOrchfile_UndecodableNonMarkdownScript_RefusedNamingReason(t *testing.T) {
	jsonFormat := useTestJSON(t)
	for name, text := range undecodableScripts {
		t.Run(name, func(t *testing.T) {
			path := writeScriptFile(t, "orchestrator-script.json", text)

			_, wfErr := orchfile.EnumerateWorkflows(path, jsonFormat)
			_, infraErr := orchfile.EnumerateInfrastructureAgents(path, jsonFormat)
			_, getErr := orchfile.GetWorkflow(path, jsonFormat, "w")

			for op, err := range map[string]error{"EnumerateWorkflows": wfErr, "EnumerateInfrastructureAgents": infraErr, "GetWorkflow": getErr} {
				re := asRefusalError(t, err)
				if re.Component != "orchfile" {
					t.Errorf("%s Component: want orchfile, got %q", op, re.Component)
				}
				if re.Resource != path {
					t.Errorf("%s Resource: want the script path %q, got %q", op, path, re.Resource)
				}
				requireNamesLoadFailure(t, op, err, agentload.ErrUndecodable)
			}
		})
	}
}

func TestOrchfile_MissingNonMarkdownScript_RefusedAsUnreadable(t *testing.T) {
	jsonFormat := useTestJSON(t)
	path := orchfileFixture("does-not-exist.json")

	_, err := orchfile.EnumerateWorkflows(path, jsonFormat)

	re := asRefusalError(t, err)
	if !strings.Contains(re.Resource, "does-not-exist.json") {
		t.Errorf("Resource: want the script path, got %q", re.Resource)
	}
	requireNamesLoadFailure(t, "EnumerateWorkflows", err, agentload.ErrUnreadable)
}

func TestOrchfile_FormatWithoutDecoder_RefusedNamingReason(t *testing.T) {
	path := writeAsTestJSON(t, orchfileFixture("bare-single.md"))
	unknown := agentfile.Format{FormatID: "no-such-agent-format", Extension: ".json"}

	_, err := orchfile.EnumerateWorkflows(path, unknown)

	re := asRefusalError(t, err)
	if re.Component != "orchfile" || re.Resource != path {
		t.Errorf("want Component orchfile and Resource %q, got %q and %q", path, re.Component, re.Resource)
	}
	requireNamesLoadFailure(t, "EnumerateWorkflows", err, agentload.ErrUnknownFormat)
}

func TestOrchfile_MarkdownScript_UnparseableFrontmatter_RefusedAsUndecodable(t *testing.T) {
	path := writeScriptFile(t, "orchestrator-script.md", "---\nname: broken\nmodel: [unclosed\n---\n<Workflow type=\"core\" name=\"w\" version=\"1\">x</Workflow>\n")

	_, err := orchfile.EnumerateWorkflows(path, agentfile.Format{})

	re := asRefusalError(t, err)
	if re.Component != "orchfile" {
		t.Errorf("Component: want orchfile, got %q", re.Component)
	}
	requireNamesLoadFailure(t, "EnumerateWorkflows", err, agentload.ErrUndecodable)
}
