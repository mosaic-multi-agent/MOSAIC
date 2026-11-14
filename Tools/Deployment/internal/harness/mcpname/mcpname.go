// Package mcpname reads and writes MCP tool names in a harness's own syntax, using the whole-server
// and single-tool forms a descriptor declares.
package mcpname

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
)

const (
	serverPlaceholder = "{server}"
	toolPlaceholder   = "{tool}"

	// maxAmbiguousReadings caps the readings reported for a tool name that splits at several
	// separators; a later transform stage resolves which reading is right.
	maxAmbiguousReadings = 2
)

// Name is one MCP tool name: a server plus an optional tool. Tool "" = the whole server.
type Name struct {
	Server string
	Tool   string
}

// IsWholeServer reports whether n names a whole server.
func (n Name) IsWholeServer() bool { return n.Tool == "" }

// Format writes n in forms (ServerForm when n.Tool == "", else ToolForm). The server is
// normalised when forms.NormalizeServerName. ok is false when the needed form is empty or
// n.Server is empty.
func Format(forms domain.MCPNameForms, n Name) (text string, ok bool) {
	form := forms.ServerForm
	if n.Tool != "" {
		form = forms.ToolForm
	}
	if form == "" || n.Server == "" {
		return "", false
	}
	server := n.Server
	if forms.NormalizeServerName {
		server = normalizeServer(server)
	}
	text = strings.Replace(form, serverPlaceholder, server, 1)
	text = strings.Replace(text, toolPlaceholder, n.Tool, 1)
	return text, true
}

// normalizeServer replaces every character outside A-Za-z0-9_- with "_".
func normalizeServer(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		}
		return '_'
	}, s)
}

// Parse reads text as an MCP name in forms. Returns nil when text is not an MCP name in these
// forms; one Name when unambiguous; several when the tool form can be split at more than one
// separator, ordered by shortest server first.
func Parse(forms domain.MCPNameForms, text string) []Name {
	return parseLimited(forms, text, maxAmbiguousReadings)
}

// ParseAll is Parse without the cap on the readings of an ambiguous single-tool name.
func ParseAll(forms domain.MCPNameForms, text string) []Name {
	return parseLimited(forms, text, 0)
}

func parseLimited(forms domain.MCPNameForms, text string, limit int) []Name {
	if forms.ServerForm != "" {
		for _, form := range append([]string{forms.ServerForm}, forms.ExtraServerReadForms...) {
			if server, ok := matchServer(form, text); ok {
				return []Name{{Server: server}}
			}
		}
	}
	if forms.ToolForm != "" {
		return splitTool(forms.ToolForm, text, limit)
	}
	return nil
}

// matchServer reads text against a whole-server form with a single {server} placeholder.
func matchServer(form, text string) (string, bool) {
	prefix, suffix, found := strings.Cut(form, serverPlaceholder)
	if !found || len(text) <= len(prefix)+len(suffix) {
		return "", false
	}
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return "", false
	}
	server := text[len(prefix) : len(text)-len(suffix)]
	if hasGlob(server) {
		return "", false
	}
	return server, true
}

// splitTool reads text against a tool form, returning every split of the text into a non-empty
// server and tool, shortest server first.
func splitTool(form, text string, limit int) []Name {
	prefix, rest, found := strings.Cut(form, serverPlaceholder)
	if !found {
		return nil
	}
	sep, suffix, found := strings.Cut(rest, toolPlaceholder)
	if !found || sep == "" {
		return nil
	}
	if len(text) < len(prefix)+len(suffix) || !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return nil
	}
	middle := text[len(prefix) : len(text)-len(suffix)]
	var out []Name
	for i := 1; i+len(sep) < len(middle); i++ {
		if !strings.HasPrefix(middle[i:], sep) {
			continue
		}
		server, tool := middle[:i], middle[i+len(sep):]
		if hasGlob(server) || hasGlob(tool) {
			continue
		}
		out = append(out, Name{Server: server, Tool: tool})
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func hasGlob(s string) bool { return strings.ContainsAny(s, "*?") }

// ValidateForms returns the validation error for malformed forms, or nil.
func ValidateForms(forms domain.MCPNameForms) error {
	if forms.ServerForm != "" {
		if err := validateServerForm("server form", forms.ServerForm); err != nil {
			return err
		}
	}
	for _, f := range forms.ExtraServerReadForms {
		if err := validateServerForm("extra server read form", f); err != nil {
			return err
		}
	}
	if forms.ToolForm != "" {
		return validateToolForm(forms.ToolForm)
	}
	return nil
}

func validateServerForm(label, form string) error {
	if strings.Count(form, serverPlaceholder) != 1 || strings.Contains(form, toolPlaceholder) {
		return fmt.Errorf("%s %q must contain %s exactly once and no %s", label, form, serverPlaceholder, toolPlaceholder)
	}
	return nil
}

func validateToolForm(form string) error {
	if strings.Count(form, serverPlaceholder) != 1 || strings.Count(form, toolPlaceholder) != 1 {
		return fmt.Errorf("tool form %q must contain %s and %s exactly once each", form, serverPlaceholder, toolPlaceholder)
	}
	s, t := strings.Index(form, serverPlaceholder), strings.Index(form, toolPlaceholder)
	if t < s {
		return fmt.Errorf("tool form %q must place %s before %s", form, serverPlaceholder, toolPlaceholder)
	}
	if t == s+len(serverPlaceholder) {
		return fmt.Errorf("tool form %q needs literal text between %s and %s", form, serverPlaceholder, toolPlaceholder)
	}
	return nil
}
