package descriptor

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/mcpname"
)

// wireMCPForms is the wire form of the tools.mcp block: how the harness writes MCP tool names.
type wireMCPForms struct {
	ServerForm           string   `yaml:"server_form"`
	ToolForm             string   `yaml:"tool_form"`
	ExtraServerReadForms []string `yaml:"extra_server_read_forms"`
	NormalizeServerName  bool     `yaml:"normalize_server_name"`
}

// retiredCustomToolTemplateError rejects a descriptor still using the retired
// custom_tool_template key and names the keys that replace it.
func retiredCustomToolTemplateError() ValidationError {
	return ValidationError{
		Field: "tools.custom_tool_template",
		Message: "custom_tool_template is no longer supported; declare the MCP name forms with " +
			"tools.mcp.server_form and tools.mcp.tool_form instead",
	}
}

// validateMCPForms checks the declared MCP name forms. A descriptor that declares none is
// valid; a descriptor that declares any must declare both the server form and the tool form,
// each well formed.
func validateMCPForms(m domain.MCPNameForms) []ValidationError {
	declared := m.ServerForm != "" || m.ToolForm != "" || len(m.ExtraServerReadForms) > 0
	if !declared {
		return nil
	}
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{Field: "tools.mcp." + field, Message: msg})
	}
	if m.ServerForm == "" {
		add("server_form", "required when tools.mcp is declared")
	} else if err := mcpname.ValidateForms(domain.MCPNameForms{ServerForm: m.ServerForm}); err != nil {
		add("server_form", err.Error())
	}
	if m.ToolForm == "" {
		add("tool_form", "required when tools.mcp is declared")
	} else if err := mcpname.ValidateForms(domain.MCPNameForms{ToolForm: m.ToolForm}); err != nil {
		add("tool_form", err.Error())
	}
	if err := mcpname.ValidateForms(domain.MCPNameForms{ExtraServerReadForms: m.ExtraServerReadForms}); err != nil {
		add("extra_server_read_forms", err.Error())
	}
	return errs
}
