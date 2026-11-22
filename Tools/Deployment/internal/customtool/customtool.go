// Package customtool reads and writes the neutral custom tool answer syntax:
// comma-separated entries, each "server" (every tool of that MCP server) or
// "server/tool" (one tool); "server/*" is the same as "server".
package customtool

import (
	"fmt"
	"strings"

	"mosaic-deploy/internal/domain"
)

const (
	reasonEmptyEntry  = "empty entry"
	reasonManySlashes = `more than one "/"`
	reasonEmptyServer = "empty server name"
	reasonEmptyTool   = "empty tool name"
	allowedChars      = `; allowed are letters, digits, "_", "-" and "."`
)

// Parse reads an answer. Entries are split at ",", and each entry and each
// side of its "/" are trimmed of surrounding whitespace. Exact duplicates
// (after "server/*" -> "server") are dropped, keeping the first. Order is the
// answer's order.
// A blank or whitespace-only text returns (nil, nil): no answer.
// The first entry that does not fit the syntax returns (nil, *EntryError).
func Parse(text string) ([]domain.MCPEntry, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var entries []domain.MCPEntry
	seen := map[domain.MCPEntry]bool{}
	for i, raw := range strings.Split(text, ",") {
		raw = strings.TrimSpace(raw)
		entry, reason := parseEntry(raw)
		if reason != "" {
			return nil, &EntryError{Index: i + 1, Entry: raw, Reason: reason}
		}
		if seen[entry] {
			continue
		}
		seen[entry] = true
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseEntry parses one trimmed entry; a non-empty reason means it is refused.
func parseEntry(raw string) (domain.MCPEntry, string) {
	if raw == "" {
		return domain.MCPEntry{}, reasonEmptyEntry
	}
	if strings.Count(raw, "/") > 1 {
		return domain.MCPEntry{}, reasonManySlashes
	}
	server, tool, hasTool := strings.Cut(raw, "/")
	server = strings.TrimSpace(server)
	tool = strings.TrimSpace(tool)
	if server == "" {
		return domain.MCPEntry{}, reasonEmptyServer
	}
	if hasTool && tool == "" {
		return domain.MCPEntry{}, reasonEmptyTool
	}
	if c, bad := firstBadChar(server); bad {
		return domain.MCPEntry{}, fmt.Sprintf("server name holds %q%s", c, allowedChars)
	}
	if tool == "*" {
		tool = ""
	} else if c, bad := firstBadChar(tool); bad {
		return domain.MCPEntry{}, fmt.Sprintf("tool name holds %q%s", c, allowedChars)
	}
	return domain.MCPEntry{Server: server, Tool: tool}, ""
}

// firstBadChar returns the first character outside A-Za-z0-9_.- in s.
func firstBadChar(s string) (string, bool) {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '_', r == '-', r == '.':
		default:
			return string(r), true
		}
	}
	return "", false
}

// Text renders entries in the neutral syntax, joined by ", ": "server" for a
// whole server, "server/tool" otherwise. Parse(Text(e)) == e for parsed e.
func Text(entries []domain.MCPEntry) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = EntryText(e)
	}
	return strings.Join(parts, ", ")
}

// EntryText renders one entry: "server" or "server/tool".
func EntryText(e domain.MCPEntry) string {
	if e.Tool == "" {
		return e.Server
	}
	return e.Server + "/" + e.Tool
}

// Validate is Parse reduced to its error; it is the TextQuestion.Validate hook.
// A blank text is valid.
func Validate(text string) error {
	_, err := Parse(text)
	return err
}

// EntryError names the refused entry and why.
type EntryError struct {
	Index  int    // 1-based position of the entry in the answer
	Entry  string // the entry as typed, trimmed
	Reason string // the reason text
}

// Error returns: entry <Index> "<Entry>": <Reason>
func (e *EntryError) Error() string {
	return fmt.Sprintf("entry %d %q: %s", e.Index, e.Entry, e.Reason)
}
