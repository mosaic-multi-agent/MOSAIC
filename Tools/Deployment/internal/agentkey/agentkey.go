// Package agentkey derives an agent key from an agent file's base name.
package agentkey

import "strings"

const markdownSuffix = ".md"

// FromFileName returns the agent key of a Markdown agent file base name: a trailing
// ".agent.md" is removed if present, otherwise a trailing ".md" is removed, otherwise
// the name is returned unchanged. Matching is case-sensitive.
func FromFileName(name string) string {
	return FromFileNameExt(name, markdownSuffix)
}

// FromFileNameExt returns the agent key of an agent file base name for a harness whose
// declared agent extension is agentExt (for example ".md", ".agent.md" or ".toml").
// The suffix is the final dotted component of agentExt; an empty agentExt means ".md".
// A trailing ".agent"+suffix is removed if present, otherwise a trailing suffix is
// removed, otherwise the name is returned unchanged. Matching is case-sensitive.
func FromFileNameExt(name, agentExt string) string {
	suffix := finalSuffix(agentExt)
	if strings.HasSuffix(name, ".agent"+suffix) {
		return strings.TrimSuffix(name, ".agent"+suffix)
	}
	return strings.TrimSuffix(name, suffix)
}

// finalSuffix returns the last dotted component of ext including its dot.
func finalSuffix(ext string) string {
	if ext == "" {
		return markdownSuffix
	}
	if i := strings.LastIndex(ext, "."); i >= 0 {
		return ext[i:]
	}
	return ext
}
