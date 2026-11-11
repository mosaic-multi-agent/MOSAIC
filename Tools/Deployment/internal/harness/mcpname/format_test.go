package mcpname_test

// format_test.go verifies that Format writes a server, with or without a tool, in the forms of
// each harness, including server-name normalisation for the harnesses that need it.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

func TestFormat_WritesEachHarnessSyntax(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
		in    mcpname.Name
		want  string
	}{
		{"claude whole server", claudeForms, mcpname.Name{Server: "contact-user"}, "mcp__contact-user__*"},
		{"claude single tool", claudeForms, mcpname.Name{Server: "contact-user", Tool: "send"}, "mcp__contact-user__send"},
		{"opencode whole server", openCodeForms, mcpname.Name{Server: "contact-user"}, "contact-user_*"},
		{"opencode single tool", openCodeForms, mcpname.Name{Server: "contact-user", Tool: "send"}, "contact-user_send"},
		{"ghcp whole server", ghcpForms, mcpname.Name{Server: "contact-user"}, "contact-user/*"},
		{"ghcp single tool", ghcpForms, mcpname.Name{Server: "contact-user", Tool: "send"}, "contact-user/send"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mcpname.Format(tc.forms, tc.in)
			if !ok || got != tc.want {
				t.Errorf("Format(%+v): want (%q, true), got (%q, %v)", tc.in, tc.want, got, ok)
			}
		})
	}
}

func TestFormat_NormalisesServerNameWhereDeclared(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
		in    mcpname.Name
		want  string
	}{
		{"claude space", claudeForms, mcpname.Name{Server: "my server"}, "mcp__my_server__*"},
		{"claude dot", claudeForms, mcpname.Name{Server: "my.server"}, "mcp__my_server__*"},
		{"claude keeps hyphen and underscore", claudeForms, mcpname.Name{Server: "a-b_c"}, "mcp__a-b_c__*"},
		{"claude normalises in tool form", claudeForms, mcpname.Name{Server: "my server", Tool: "go"}, "mcp__my_server__go"},
		{"opencode space", openCodeForms, mcpname.Name{Server: "my server"}, "my_server_*"},
		{"opencode slash", openCodeForms, mcpname.Name{Server: "a/b", Tool: "go"}, "a_b_go"},
		{"ghcp keeps the name as given", ghcpForms, mcpname.Name{Server: "my server"}, "my server/*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mcpname.Format(tc.forms, tc.in)
			if !ok || got != tc.want {
				t.Errorf("Format(%+v): want (%q, true), got (%q, %v)", tc.in, tc.want, got, ok)
			}
		})
	}
}

func TestFormat_NotOKWhenServerEmptyOrFormUndeclared(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
		in    mcpname.Name
	}{
		{"empty server whole", ghcpForms, mcpname.Name{}},
		{"empty server with tool", ghcpForms, mcpname.Name{Tool: "send"}},
		{"no forms whole server", domain.MCPNameForms{}, mcpname.Name{Server: "srv"}},
		{"no forms single tool", domain.MCPNameForms{}, mcpname.Name{Server: "srv", Tool: "t"}},
		{"tool form missing", domain.MCPNameForms{ServerForm: "{server}/*"}, mcpname.Name{Server: "srv", Tool: "t"}},
		{"server form missing", domain.MCPNameForms{ToolForm: "{server}/{tool}"}, mcpname.Name{Server: "srv"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := mcpname.Format(tc.forms, tc.in); ok {
				t.Errorf("Format(%+v): want ok=false, got (%q, true)", tc.in, got)
			}
		})
	}
}
