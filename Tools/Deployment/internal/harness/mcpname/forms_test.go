package mcpname_test

// forms_test.go holds the per-harness MCP name forms the tests of this package share.
// The values are the documented forms of each built-in harness.

import (
	"mosaic-deploy/internal/domain"
)

var claudeForms = domain.MCPNameForms{
	ServerForm:          "mcp__{server}__*",
	ToolForm:            "mcp__{server}__{tool}",
	NormalizeServerName: true,
}

var openCodeForms = domain.MCPNameForms{
	ServerForm:           "{server}_*",
	ToolForm:             "{server}_{tool}",
	ExtraServerReadForms: []string{"{server}*"},
	NormalizeServerName:  true,
}

var ghcpForms = domain.MCPNameForms{
	ServerForm: "{server}/*",
	ToolForm:   "{server}/{tool}",
}
