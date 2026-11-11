package builtin_test

// mcp_forms_test.go verifies the MCP name forms declared by the four built-in harness
// descriptors and that a user-supplied MCP server name is written in each harness's own
// whole-server syntax.
//
// Coverage:
//   - Each built-in descriptor declares the whole-server and single-tool forms, the OpenCode
//     read leniency, server-name normalisation, and the unknown-name reporting flag.
//   - No built-in descriptor still uses custom_tool_template.
//   - A custom server answer is written as mcp__<server>__* (claude-code), <server>_* allowed
//     in the permission map (opencode), and <server>/* (both GHCP targets).
//   - Server names with characters outside A-Za-z0-9_- are normalised where declared.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// mcpHarness is one built-in harness with its expected MCP declaration.
type mcpHarness struct {
	dir, file string
	forms     domain.MCPNameForms
	report    bool
	// wantWhole is the whole-server name written for the server "my-server".
	wantWhole string
}

var mcpHarnesses = []mcpHarness{
	{"claudecode", "claude-code.yaml",
		domain.MCPNameForms{ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}", NormalizeServerName: true},
		false, "mcp__my-server__*"},
	{"opencode", "opencode.yaml",
		domain.MCPNameForms{ServerForm: "{server}_*", ToolForm: "{server}_{tool}", ExtraServerReadForms: []string{"{server}*"}, NormalizeServerName: true},
		false, "my-server_*"},
	{"ghcpcli", "ghcp-cli.yaml",
		domain.MCPNameForms{ServerForm: "{server}/*", ToolForm: "{server}/{tool}"},
		true, "my-server/*"},
	{"vscodeghcp", "vscode-ghcp.yaml",
		domain.MCPNameForms{ServerForm: "{server}/*", ToolForm: "{server}/{tool}"},
		true, "my-server/*"},
}

func TestBuiltInDescriptors_DeclareMCPForms(t *testing.T) {
	for _, h := range mcpHarnesses {
		t.Run(h.dir, func(t *testing.T) {
			d := loadEmbeddedDescriptor(t, h.dir, h.file)
			if !reflect.DeepEqual(d.Tools.MCP, h.forms) {
				t.Errorf("Tools.MCP: want %+v, got %+v", h.forms, d.Tools.MCP)
			}
			if d.Tools.ReportUnknownNames != h.report {
				t.Errorf("Tools.ReportUnknownNames: want %v, got %v", h.report, d.Tools.ReportUnknownNames)
			}
		})
	}
}

func TestBuiltInDescriptors_DoNotUseCustomToolTemplate(t *testing.T) {
	root := repoRoot(t)
	for _, h := range mcpHarnesses {
		path := filepath.Join(root, "Tools", "Deployment", "internal", "harness", "builtin", h.dir, h.file)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(src), "custom_tool_template") {
			t.Errorf("%s still declares custom_tool_template; it is replaced by tools.mcp", h.file)
		}
	}
}

// customServerNames maps one custom server answer through the harness's tool mapping and
// returns every tool name written in the tools field, with allow/deny for permission shapes.
func customServerNames(t *testing.T, d *domain.HarnessDescriptor, server string) map[string]string {
	t.Helper()
	res, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:    "mcp-user",
		Generic:     []string{"user_feedback"},
		CustomNames: map[string]string{"user_feedback": server},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	written := map[string]string{}
	for _, f := range res.Fields {
		if f.Key != d.Frontmatter.ToolsKey {
			continue
		}
		switch f.Value.Kind {
		case domain.KindMapping:
			for _, p := range f.Value.Pairs {
				written[p.Key] = p.Value.Scalar
			}
		case domain.KindList:
			for _, it := range f.Value.Items {
				written[it.Scalar] = "allow"
			}
		case domain.KindScalar:
			for _, n := range strings.Split(f.Value.Scalar, ",") {
				written[strings.TrimSpace(n)] = "allow"
			}
		}
	}
	return written
}

func TestCustomServerAnswer_IsWrittenInTheTargetHarnessWholeServerForm(t *testing.T) {
	for _, h := range mcpHarnesses {
		t.Run(h.dir, func(t *testing.T) {
			d := loadEmbeddedDescriptor(t, h.dir, h.file)
			written := customServerNames(t, d, "my-server")

			disp, ok := written[h.wantWhole]
			if !ok {
				t.Fatalf("want %q among the written tool names, got %v", h.wantWhole, written)
			}
			if d.Tools.Shape == domain.ShapePermission && disp != "allow" {
				t.Errorf("%q must be allowed in the permission map, got %q", h.wantWhole, disp)
			}
			if _, bare := written["my-server"]; bare {
				t.Errorf("the bare server name must not be written; got %v", written)
			}
		})
	}
}

func TestCustomServerAnswer_NormalisedOnlyWhereTheDescriptorDeclaresIt(t *testing.T) {
	want := map[string]string{
		"claudecode": "mcp__my_server__*",
		"opencode":   "my_server_*",
		"ghcpcli":    "my server/*",
		"vscodeghcp": "my server/*",
	}
	for _, h := range mcpHarnesses {
		t.Run(h.dir, func(t *testing.T) {
			d := loadEmbeddedDescriptor(t, h.dir, h.file)
			written := customServerNames(t, d, "my server")
			if _, ok := written[want[h.dir]]; !ok {
				t.Errorf("want %q among the written tool names, got %v", want[h.dir], written)
			}
		})
	}
}
