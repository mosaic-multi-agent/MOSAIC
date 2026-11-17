package app

// opencode_mcp_config.go reads the MCP servers declared in OpenCode's configuration. They
// decide how an ambiguous OpenCode tool name splits into server and tool. The configuration
// is only read, never written; a missing, unreadable or invalid file declares nothing.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
)

// openCodeConfigFiles are the configuration files at the workspace root, both read as JSONC.
var openCodeConfigFiles = []string{"opencode.json", "opencode.jsonc"}

// openCodeV2ConfigFiles are the V2 project configuration files, at the workspace root and in
// .opencode/, all read as JSONC.
var openCodeV2ConfigFiles = []string{
	"opencode.json", "opencode.jsonc",
	".opencode/opencode.json", ".opencode/opencode.jsonc",
}

// readOpenCodeDeclaredServers returns the keys of the top-level mcp object of OpenCode's
// configuration files at the workspace root, combined and sorted. It never fails: a file that
// cannot be read or parsed contributes nothing.
func readOpenCodeDeclaredServers(workspace string) []string {
	return readDeclaredServers(workspace, openCodeConfigFiles, func(mcp json.RawMessage) json.RawMessage { return mcp })
}

// readOpenCodeV2DeclaredServers returns the keys of the mcp.servers object of the V2 project
// configuration files, combined and sorted. Keys directly under mcp (the V1 form) are not
// servers. It never fails: a file that cannot be read or parsed contributes nothing.
func readOpenCodeV2DeclaredServers(workspace string) []string {
	return readDeclaredServers(workspace, openCodeV2ConfigFiles, func(mcp json.RawMessage) json.RawMessage {
		var v2 struct {
			Servers json.RawMessage `json:"servers"`
		}
		if json.Unmarshal(mcp, &v2) != nil {
			return nil
		}
		return v2.Servers
	})
}

// readDeclaredServers collects the keys of the object that pick selects from the mcp value of
// each configuration file.
func readDeclaredServers(workspace string, files []string, pick func(mcp json.RawMessage) json.RawMessage) []string {
	var servers []string
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		var cfg struct {
			MCP json.RawMessage `json:"mcp"`
		}
		if json.Unmarshal(stripJSONC(data), &cfg) != nil || cfg.MCP == nil {
			continue
		}
		var keys map[string]json.RawMessage
		if raw := pick(cfg.MCP); raw == nil || json.Unmarshal(raw, &keys) != nil {
			continue
		}
		for server := range keys {
			if server != "" && !slices.Contains(servers, server) {
				servers = append(servers, server)
			}
		}
	}
	slices.Sort(servers)
	return servers
}

// stripJSONC removes line and block comments and trailing commas from JSONC text, leaving
// string contents untouched.
func stripJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '"':
			end := endOfJSONString(src, i)
			out = append(out, src[i:end]...)
			i = end - 1
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			i--
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			i++
		case c == ',' && nextIsClosing(src, i+1):
			// trailing comma
		default:
			out = append(out, c)
		}
	}
	return out
}

// endOfJSONString returns the index just past the string literal that starts at start.
func endOfJSONString(src []byte, start int) int {
	for i := start + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return len(src)
}

// nextIsClosing reports whether the next significant text after from, skipping white space and
// comments, is a closing bracket or brace.
func nextIsClosing(src []byte, from int) bool {
	for i := from; i < len(src); i++ {
		switch {
		case src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n':
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			i++
		default:
			return src[i] == '}' || src[i] == ']'
		}
	}
	return false
}
