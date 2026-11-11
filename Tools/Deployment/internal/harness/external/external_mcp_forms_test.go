package external_test

// external_mcp_forms_test.go verifies that the reference external OpenCode module, built from
// the same source as the built-in OpenCode harness, writes a custom MCP server answer in
// OpenCode's whole-server form.

import (
	"testing"
	"time"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/external"
)

func TestReferenceModule_CustomServerAnswerIsWrittenInOpenCodeServerForm(t *testing.T) {
	binPath := buildReferenceModule(t)
	desc := openCodeDescriptor(t)

	m, err := external.New(binPath, desc, external.Options{
		Timeout:    10 * time.Second,
		MosaicRoot: findRepoRoot(t),
	})
	if err != nil {
		t.Fatalf("external.New with reference module binary: %v", err)
	}
	defer m.Close() //nolint:errcheck

	result, err := m.Tools(domain.ToolRequest{
		AgentKey:    "mcp-user",
		Generic:     []string{"user_feedback"},
		CustomNames: map[string]string{"user_feedback": "my server"},
	})
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	if len(result.Resolutions) != 1 || result.Resolutions[0].Outcome != domain.ToolCustom {
		t.Fatalf("want one custom resolution, got %+v", result.Resolutions)
	}
	got := result.Resolutions[0].HarnessTools
	if len(got) != 1 || got[0] != "my_server_*" {
		t.Errorf("HarnessTools: want [my_server_*] (OpenCode whole-server form, name normalised), got %v", got)
	}

	var allowed bool
	for _, f := range result.Fields {
		for _, p := range f.Value.Pairs {
			if p.Key == "my_server_*" && p.Value.Scalar == "allow" {
				allowed = true
			}
		}
	}
	if !allowed {
		t.Errorf("permission map must carry my_server_*: allow; fields: %+v", result.Fields)
	}
}
