// Package builtindesc exposes the descriptor YAML of every builtin harness, shared by all tools.
package builtindesc

import (
	"embed"
	"slices"
)

//go:embed claude-code.yaml opencode.yaml ghcp-cli.yaml vscode-ghcp.yaml codex.yaml opencode-v2.yaml
var files embed.FS

var ids = []string{"claude-code", "opencode", "ghcp-cli", "vscode-ghcp", "codex", "opencode-v2"}

// IDs returns every builtin harness id, in builtin order:
// "claude-code", "opencode", "ghcp-cli", "vscode-ghcp", "codex", "opencode-v2".
// The slice is fresh on every call.
func IDs() []string {
	return slices.Clone(ids)
}

// Raw returns the embedded descriptor YAML for id, byte-for-byte as stored in
// <id>.yaml, as a fresh slice. ok is false (and the bytes nil) for an id that
// is not a builtin, including "". Never panics.
func Raw(id string) (src []byte, ok bool) {
	if !slices.Contains(ids, id) {
		return nil, false
	}
	src, err := files.ReadFile(id + ".yaml")
	if err != nil {
		return nil, false
	}
	return src, true
}
