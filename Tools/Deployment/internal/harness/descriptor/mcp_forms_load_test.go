package descriptor_test

// mcp_forms_load_test.go verifies loading and validation of the tools.mcp block that declares
// how a harness writes MCP tool names, and the rejection of the retired custom_tool_template.
//
// Coverage:
//   - A tools.mcp block loads into ToolSpec.MCP, optional read forms and flags included.
//   - The optional keys default to empty / false.
//   - A descriptor using custom_tool_template fails to load with a message naming the
//     replacement keys.
//   - Malformed forms are validation errors located at tools.mcp.<key>.
//   - A block declaring only one of the two required forms is rejected.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// mcpDescriptorYAML builds a minimal list-shape descriptor whose tools block carries the given
// extra lines (already indented by two spaces).
func mcpDescriptorYAML(toolsExtra string) string {
	return `schema_version: "1"
id: "mcp-forms-test"
display_name: "MCP Forms Test"
tools:
  shape: list
` + toolsExtra + `frontmatter:
  tools_key: "tools"
`
}

const mcpFullBlock = `  mcp:
    server_form: "{server}_*"
    tool_form: "{server}_{tool}"
    extra_server_read_forms:
      - "{server}*"
    normalize_server_name: true
  report_unknown_tool_names: true
`

func TestParse_MCPBlock_PopulatesForms(t *testing.T) {
	d, err := descriptor.Parse([]byte(mcpDescriptorYAML(mcpFullBlock)), "inline:mcp-full")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := domain.MCPNameForms{
		ServerForm:           "{server}_*",
		ToolForm:             "{server}_{tool}",
		ExtraServerReadForms: []string{"{server}*"},
		NormalizeServerName:  true,
	}
	if !reflect.DeepEqual(d.Tools.MCP, want) {
		t.Errorf("Tools.MCP: want %+v, got %+v", want, d.Tools.MCP)
	}
	if !d.Tools.ReportUnknownNames {
		t.Error("Tools.ReportUnknownNames: want true")
	}
}

func TestParse_MCPBlock_OptionalKeysDefaultToEmpty(t *testing.T) {
	src := mcpDescriptorYAML(`  mcp:
    server_form: "{server}/*"
    tool_form: "{server}/{tool}"
`)
	d, err := descriptor.Parse([]byte(src), "inline:mcp-min")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(d.Tools.MCP.ExtraServerReadForms) != 0 || d.Tools.MCP.NormalizeServerName || d.Tools.ReportUnknownNames {
		t.Errorf("optional keys must default to empty/false, got %+v report=%v", d.Tools.MCP, d.Tools.ReportUnknownNames)
	}
	if d.Tools.MCP.ServerForm != "{server}/*" || d.Tools.MCP.ToolForm != "{server}/{tool}" {
		t.Errorf("forms not loaded: %+v", d.Tools.MCP)
	}
}

func TestParse_RetiredCustomToolTemplate_IsRejectedWithGuidance(t *testing.T) {
	src := mcpDescriptorYAML(`  custom_tool_template: "mcp__%s__*"
`)
	_, err := descriptor.Parse([]byte(src), "inline:old-template")
	if err == nil {
		t.Fatal("Parse: want an error for custom_tool_template, got nil")
	}
	for _, want := range []string{"custom_tool_template", "no longer supported", "tools.mcp.server_form", "tools.mcp.tool_form"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must contain %q", err.Error(), want)
		}
	}
}

func TestParse_RetiredCustomToolTemplate_IsRejectedWhenAlongsideForms(t *testing.T) {
	src := mcpDescriptorYAML(mcpFullBlock + `  custom_tool_template: "mcp__%s__*"
`)
	_, err := descriptor.Parse([]byte(src), "inline:both")
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("Parse: want the retired-field error even with forms declared, got %v", err)
	}
}

func TestLoad_RetiredCustomToolTemplate_IsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.yaml")
	src := mcpDescriptorYAML(`  custom_tool_template: "mcp__%s__*"
`)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := descriptor.Load(path)
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("Load: want the retired-field error, got %v", err)
	}
}

func TestParse_MCPBlock_MissingOneRequiredFormIsRejected(t *testing.T) {
	cases := []struct {
		name  string
		block string
	}{
		{"server form only", "  mcp:\n    server_form: \"{server}/*\"\n"},
		{"tool form only", "  mcp:\n    tool_form: \"{server}/{tool}\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := descriptor.Parse([]byte(mcpDescriptorYAML(tc.block)), "inline:one-form"); err == nil {
				t.Error("Parse: want an error when only one of server_form and tool_form is declared")
			}
		})
	}
}

func TestParse_MCPBlock_MalformedFormsAreRejected(t *testing.T) {
	cases := []struct {
		name      string
		server    string
		tool      string
		extra     string
		wantField string
	}{
		{"server form without placeholder", "srv/*", "{server}/{tool}", "", "tools.mcp.server_form"},
		{"server form with tool placeholder", "{server}/{tool}", "{server}/{tool}", "", "tools.mcp.server_form"},
		{"tool form without tool placeholder", "{server}/*", "{server}/x", "", "tools.mcp.tool_form"},
		{"tool form with tool before server", "{server}/*", "{tool}/{server}", "", "tools.mcp.tool_form"},
		{"tool form without separator", "{server}/*", "{server}{tool}", "", "tools.mcp.tool_form"},
		{"extra read form with two server placeholders", "{server}/*", "{server}/{tool}", "{server}{server}", "tools.mcp.extra_server_read_forms"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			block := "  mcp:\n    server_form: \"" + tc.server + "\"\n    tool_form: \"" + tc.tool + "\"\n"
			if tc.extra != "" {
				block += "    extra_server_read_forms:\n      - \"" + tc.extra + "\"\n"
			}
			_, err := descriptor.Parse([]byte(mcpDescriptorYAML(block)), "inline:bad-forms")
			if err == nil {
				t.Fatal("Parse: want a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("error %q must name the field %q", err.Error(), tc.wantField)
			}
		})
	}
}

func TestValidate_MalformedMCPForms_ReportsErrorAtFieldPath(t *testing.T) {
	d := &domain.HarnessDescriptor{
		SchemaVersion: "1",
		ID:            "mcp-validate",
		DisplayName:   "MCP Validate",
		Tools: domain.ToolSpec{
			MCP: domain.MCPNameForms{ServerForm: "no-placeholder", ToolForm: "{server}/{tool}"},
		},
	}
	errs := descriptor.Validate(d)
	if firstErrorForField(errs, "tools.mcp.server_form") == nil {
		t.Errorf("want a validation error at tools.mcp.server_form, got %v", errs)
	}
}

func TestValidate_WellFormedMCPForms_ProducesNoErrors(t *testing.T) {
	d := &domain.HarnessDescriptor{
		SchemaVersion: "1",
		ID:            "mcp-validate",
		DisplayName:   "MCP Validate",
		Tools: domain.ToolSpec{
			MCP: domain.MCPNameForms{
				ServerForm:           "{server}_*",
				ToolForm:             "{server}_{tool}",
				ExtraServerReadForms: []string{"{server}*"},
			},
		},
	}
	if errs := descriptor.Validate(d); len(errs) != 0 {
		t.Errorf("want no validation errors, got %v", errs)
	}
}

func TestValidate_MCPBlockWithoutToolForm_ReportsToolFormError(t *testing.T) {
	// A tools.mcp block that declares only a server form cannot write a single-tool name.
	d := &domain.HarnessDescriptor{
		SchemaVersion: "1",
		ID:            "mcp-validate",
		DisplayName:   "MCP Validate",
		Tools:         domain.ToolSpec{MCP: domain.MCPNameForms{ServerForm: "{server}/*"}},
	}
	if firstErrorForField(descriptor.Validate(d), "tools.mcp.tool_form") == nil {
		t.Error("want a validation error at tools.mcp.tool_form for a block without a tool form")
	}
}
