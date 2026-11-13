package domain

import (
	"regexp"
	"strings"
)

// structuredInstanceID matches the structured form without the "not preceded by
// an identifier character" condition, which Go's regexp cannot express; the
// caller checks the preceding byte.
var structuredInstanceID = regexp.MustCompile(
	`agent_instance_id\s*["']?\s*[:=]\s*["']?\s*([A-Za-z][A-Za-z0-9_.-]*#[0-9]+)`)

// ExtractInstanceID returns the agent instance id named in text by the structured
// form only: the token agent_instance_id, not preceded by [A-Za-z0-9_], followed by
// optional whitespace, an optional ' or " quote, optional whitespace, ':' or '=',
// optional whitespace, an optional quote, optional whitespace, then an id of the
// form [A-Za-z][A-Za-z0-9_.-]*#[0-9]+. The id is the longest match at that point.
// The first such occurrence in text wins. A bare "Name#N" never matches.
// Whitespace means ASCII whitespace only. ok is false when no occurrence exists.
func ExtractInstanceID(text string) (id AgentInstanceID, ok bool) {
	offset := 0
	for offset < len(text) {
		m := structuredInstanceID.FindStringSubmatchIndex(text[offset:])
		if m == nil {
			return "", false
		}
		start := offset + m[0]
		if start == 0 || !isIdentifierByte(text[start-1]) {
			return AgentInstanceID(text[offset+m[2] : offset+m[3]]), true
		}
		offset = start + 1
	}
	return "", false
}

func isIdentifierByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// IsSubagentLaunchTool reports whether a tool_call_start tool name launches a
// subagent: agent, task, runsubagent or execution_subagent, compared
// case-insensitively (ASCII).
func IsSubagentLaunchTool(name string) bool {
	switch strings.ToLower(name) {
	case "agent", "task", "runsubagent", "execution_subagent":
		return true
	}
	return false
}
