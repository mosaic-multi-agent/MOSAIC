package mcpname

import (
	"strings"

	"mosaic-deploy/internal/domain"
)

// toolMarker stands in for the tool while a single-tool form is cut into its fixed parts.
const toolMarker = "\x00"

// SplitAt reads text as a single-tool name of the given server: the form written for that server
// must be a prefix of text, with a non-empty tool after it. ok is false when text does not
// start with the server's form or leaves no tool.
func SplitAt(forms domain.MCPNameForms, text, server string) (n Name, ok bool) {
	probe, formed := Format(forms, Name{Server: server, Tool: toolMarker})
	if !formed {
		return Name{}, false
	}
	prefix, suffix, found := strings.Cut(probe, toolMarker)
	if !found || !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return Name{}, false
	}
	if len(text) <= len(prefix)+len(suffix) {
		return Name{}, false
	}
	tool := text[len(prefix) : len(text)-len(suffix)]
	if hasGlob(tool) {
		return Name{}, false
	}
	return Name{Server: server, Tool: tool}, true
}
