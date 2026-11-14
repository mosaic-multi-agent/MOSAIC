package mcpname_test

// validate_test.go verifies the shape rules for declared MCP name forms: placeholders must
// appear the right number of times in the right order.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

func TestValidateForms_AcceptsTheBuiltInHarnessForms(t *testing.T) {
	for name, f := range map[string]domain.MCPNameForms{"claude": claudeForms, "opencode": openCodeForms, "ghcp": ghcpForms} {
		if err := mcpname.ValidateForms(f); err != nil {
			t.Errorf("%s forms must be valid, got %v", name, err)
		}
	}
}

func TestValidateForms_RejectsMalformedForms(t *testing.T) {
	good := domain.MCPNameForms{ServerForm: "{server}/*", ToolForm: "{server}/{tool}"}
	with := func(mut func(*domain.MCPNameForms)) domain.MCPNameForms {
		f := good
		mut(&f)
		return f
	}
	cases := []struct {
		name  string
		forms domain.MCPNameForms
	}{
		{"server form without placeholder", with(func(f *domain.MCPNameForms) { f.ServerForm = "srv/*" })},
		{"server form with tool placeholder", with(func(f *domain.MCPNameForms) { f.ServerForm = "{server}/{tool}" })},
		{"server form with two server placeholders", with(func(f *domain.MCPNameForms) { f.ServerForm = "{server}/{server}" })},
		{"tool form without tool placeholder", with(func(f *domain.MCPNameForms) { f.ToolForm = "{server}/x" })},
		{"tool form without server placeholder", with(func(f *domain.MCPNameForms) { f.ToolForm = "x/{tool}" })},
		{"tool form with two tool placeholders", with(func(f *domain.MCPNameForms) { f.ToolForm = "{server}/{tool}/{tool}" })},
		{"tool form with two server placeholders", with(func(f *domain.MCPNameForms) { f.ToolForm = "{server}/{server}/{tool}" })},
		{"tool before server", with(func(f *domain.MCPNameForms) { f.ToolForm = "{tool}/{server}" })},
		{"no literal between server and tool", with(func(f *domain.MCPNameForms) { f.ToolForm = "{server}{tool}" })},
		{"extra read form without placeholder", with(func(f *domain.MCPNameForms) { f.ExtraServerReadForms = []string{"srv*"} })},
		{"extra read form with tool placeholder", with(func(f *domain.MCPNameForms) { f.ExtraServerReadForms = []string{"{server}{tool}"} })},
		{"second extra read form malformed", with(func(f *domain.MCPNameForms) { f.ExtraServerReadForms = []string{"{server}*", "{server}{server}"} })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := mcpname.ValidateForms(tc.forms); err == nil {
				t.Errorf("ValidateForms(%+v): want an error, got nil", tc.forms)
			}
		})
	}
}
