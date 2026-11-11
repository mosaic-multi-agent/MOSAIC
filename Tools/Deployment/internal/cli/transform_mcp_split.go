package cli

// transform_mcp_split.go parses the --mcp-split flag of the transform command, which
// pre-answers the question of how an ambiguous MCP tool name splits into server and tool.

import (
	"fmt"
	"strings"
)

// parseMCPSplits parses repeated sourceName=server values. A malformed or repeated value is an
// error naming it.
func parseMCPSplits(values []string) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, v := range values {
		name, server, found := strings.Cut(v, "=")
		name, server = strings.TrimSpace(name), strings.TrimSpace(server)
		if !found || name == "" || server == "" {
			return nil, fmt.Errorf("--mcp-split %q: expected format is sourceName=server, both parts non-empty", v)
		}
		if _, exists := result[name]; exists {
			return nil, fmt.Errorf("--mcp-split: source name %q appears more than once", name)
		}
		result[name] = server
	}
	return result, nil
}
