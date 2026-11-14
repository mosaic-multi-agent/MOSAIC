package mcpname_test

// parse_test.go verifies that Parse reads MCP names per harness: whole-server and single-tool
// forms, OpenCode's lenient read of "<server>*", ambiguous OpenCode and Claude Code splits, and
// names that are not MCP names at all.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

func TestParse_ReadsEachHarnessSyntax(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
		in    string
		want  []mcpname.Name
	}{
		{"claude whole server", claudeForms, "mcp__srv__*", []mcpname.Name{{Server: "srv"}}},
		{"claude hyphenated server", claudeForms, "mcp__my-server__*", []mcpname.Name{{Server: "my-server"}}},
		{"claude server with double underscore reads as whole server", claudeForms, "mcp__a__b__*", []mcpname.Name{{Server: "a__b"}}},
		{"claude single tool", claudeForms, "mcp__srv__send", []mcpname.Name{{Server: "srv", Tool: "send"}}},
		{"opencode whole server", openCodeForms, "contact-user_*", []mcpname.Name{{Server: "contact-user"}}},
		{"opencode single tool", openCodeForms, "contact-user_send", []mcpname.Name{{Server: "contact-user", Tool: "send"}}},
		{"ghcp whole server", ghcpForms, "contact-user/*", []mcpname.Name{{Server: "contact-user"}}},
		{"ghcp single tool", ghcpForms, "contact-user/send", []mcpname.Name{{Server: "contact-user", Tool: "send"}}},
		{"ghcp hierarchical tool name", ghcpForms, "edit/editFiles", []mcpname.Name{{Server: "edit", Tool: "editFiles"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mcpname.Parse(tc.forms, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q): want %v, got %v", tc.in, tc.want, got)
			}
		})
	}
}

func TestParse_OpenCodeReadsLooseServerGlobAsWholeServer(t *testing.T) {
	got := mcpname.Parse(openCodeForms, "contact-user*")
	want := []mcpname.Name{{Server: "contact-user"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(contact-user*): want %v, got %v", want, got)
	}
}

func TestParse_OpenCodeStrictFormWinsOverLooseForm(t *testing.T) {
	// "x_*" also fits "{server}*" with server "x_"; the strict form is tried first.
	got := mcpname.Parse(openCodeForms, "x_*")
	want := []mcpname.Name{{Server: "x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(x_*): want %v, got %v", want, got)
	}
}

func TestParse_OpenCodeSingleToolSplitAtSeveralSeparatorsIsAmbiguous(t *testing.T) {
	got := mcpname.Parse(openCodeForms, "azure-devops_get_pull_request")
	want := []mcpname.Name{
		{Server: "azure-devops", Tool: "get_pull_request"},
		{Server: "azure-devops_get", Tool: "pull_request"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want both splits, shortest server first:\n want %v\n got  %v", want, got)
	}
}

func TestParse_ClaudeSingleToolWithDoubleUnderscoreInNamesIsAmbiguous(t *testing.T) {
	got := mcpname.Parse(claudeForms, "mcp__a__b__tool")
	want := []mcpname.Name{
		{Server: "a", Tool: "b__tool"},
		{Server: "a__b", Tool: "tool"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want both splits, shortest server first:\n want %v\n got  %v", want, got)
	}
}

func TestParse_NamesThatAreNotMcpNamesYieldNil(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
		in    string
	}{
		{"claude plain tool", claudeForms, "Read"},
		{"claude empty", claudeForms, ""},
		{"claude prefix only", claudeForms, "mcp__"},
		{"claude empty server whole", claudeForms, "mcp____*"},
		{"claude missing tool", claudeForms, "mcp__srv__"},
		{"claude overlapping prefix and suffix", claudeForms, "mcp__*"},
		{"claude missing prefix", claudeForms, "srv__*"},
		{"claude tool with glob", claudeForms, "mcp__srv__se*"},
		{"opencode plain tool", openCodeForms, "read"},
		{"opencode empty", openCodeForms, ""},
		{"opencode only separator", openCodeForms, "_"},
		{"opencode glob only", openCodeForms, "*"},
		{"opencode trailing separator", openCodeForms, "srv_"},
		{"ghcp plain tool", ghcpForms, "read"},
		{"ghcp empty server", ghcpForms, "/send"},
		{"ghcp empty tool", ghcpForms, "srv/"},
		{"ghcp glob only", ghcpForms, "*"},
		{"ghcp tool with glob", ghcpForms, "srv/se*"},
		{"ghcp tool with question mark", ghcpForms, "srv/se?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mcpname.Parse(tc.forms, tc.in); got != nil {
				t.Errorf("Parse(%q): want nil, got %v", tc.in, got)
			}
		})
	}
}

func TestParse_UndeclaredFormsNeverMatch(t *testing.T) {
	cases := []struct {
		name  string
		forms domain.MCPNameForms
	}{
		{"no forms", domain.MCPNameForms{}},
		{"only extra read form", domain.MCPNameForms{ExtraServerReadForms: []string{"{server}*"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, in := range []string{"srv/*", "srv/tool", "srv*", "mcp__srv__*", "read", ""} {
				if got := mcpname.Parse(tc.forms, in); got != nil {
					t.Errorf("Parse(%q): want nil for undeclared forms, got %v", in, got)
				}
			}
		})
	}
}

func TestParse_ServerFormDeclaredWithoutToolFormReadsOnlyWholeServer(t *testing.T) {
	forms := domain.MCPNameForms{ServerForm: "{server}/*"}
	if got := mcpname.Parse(forms, "srv/*"); !reflect.DeepEqual(got, []mcpname.Name{{Server: "srv"}}) {
		t.Errorf("Parse(srv/*): want whole server srv, got %v", got)
	}
	if got := mcpname.Parse(forms, "srv/tool"); got != nil {
		t.Errorf("Parse(srv/tool) without a tool form: want nil, got %v", got)
	}
}

func TestParse_FormatRoundTripsForEachHarness(t *testing.T) {
	names := []mcpname.Name{{Server: "contact-user"}, {Server: "contact-user", Tool: "send"}}
	forms := map[string]domain.MCPNameForms{"claude": claudeForms, "opencode": openCodeForms, "ghcp": ghcpForms}
	for label, f := range forms {
		for _, n := range names {
			text, ok := mcpname.Format(f, n)
			if !ok {
				t.Fatalf("%s: Format(%+v) not ok", label, n)
			}
			got := mcpname.Parse(f, text)
			if len(got) == 0 || got[0] != n {
				t.Errorf("%s: Parse(Format(%+v)=%q): want first result %v, got %v", label, n, text, n, got)
			}
		}
	}
}
