// Package agentkey derives an agent key from an agent file's base name.
package agentkey

import "strings"

// FromFileName returns the agent key of a file base name: a trailing ".agent.md" is
// removed if present, otherwise a trailing ".md" is removed, otherwise the name is
// returned unchanged. Matching is case-sensitive.
func FromFileName(name string) string {
	if strings.HasSuffix(name, ".agent.md") {
		return strings.TrimSuffix(name, ".agent.md")
	}
	return strings.TrimSuffix(name, ".md")
}
