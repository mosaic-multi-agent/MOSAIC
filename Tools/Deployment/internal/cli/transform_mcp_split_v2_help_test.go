package cli_test

// transform_mcp_split_v2_help_test.go covers the --mcp-split help text of the transform command:
// the flag covers an OpenCode V2 source as it covers an OpenCode one, and says so.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

func TestTransformCmd_HelpText_McpSplitCoversOpenCodeV2(t *testing.T) {
	errBuf := &bytes.Buffer{}

	cli.Run(context.Background(), []string{"transform", "--help"}, &spyService{}, &bytes.Buffer{}, errBuf)

	help := strings.Join(strings.Fields(errBuf.String()), " ")
	for _, want := range []string{
		"OpenCode or OpenCode V2 MCP tool name",
		"sourceName=server",
		"azure-devops_get_pull_request=azure-devops",
		"repeat once per name",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("transform --help lacks %q in the --mcp-split text, got:\n%s", want, errBuf.String())
		}
	}
}
