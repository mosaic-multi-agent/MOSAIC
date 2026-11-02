package cli_test

// conflict_help_test.go verifies the --conflict flag help and the agents long help describe
// the real scope of conflicts: files the tool cannot confirm as its own.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

// helpText returns everything the CLI prints for "<sub> --help", whichever stream it uses.
func helpText(t *testing.T, sub string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	cli.Run(context.Background(), []string{sub, "--help"}, &spyService{}, &out, &errOut)
	return out.String() + errOut.String()
}

func TestConflictFlagHelp_StatesFilesTheToolCannotConfirmAsItsOwn(t *testing.T) {
	cases := map[string]string{
		"deploy":    "How to handle files the tool cannot confirm as its own (skip|overwrite|backup)",
		"update":    "How to handle files the tool cannot confirm as its own (skip|overwrite|backup)",
		"workflows": "How to handle files the tool cannot confirm as its own (skip|overwrite|backup)",
		"agents":    "Default decision for files the tool cannot confirm as its own (skip|overwrite|backup)",
		"hooks":     "Default decision for files the tool cannot confirm as its own (skip|overwrite|backup)",
	}
	for sub, want := range cases {
		t.Run(sub, func(t *testing.T) {
			help := helpText(t, sub)

			if !strings.Contains(help, "--conflict") {
				t.Fatalf("help for %s does not list --conflict:\n%s", sub, help)
			}
			if !strings.Contains(help, want) {
				t.Errorf("help for %s does not contain %q:\n%s", sub, want, help)
			}
		})
	}
}

func TestConflictFlagHelp_NoSubcommandPromisesLocallyModifiedProtection(t *testing.T) {
	for _, sub := range []string{"deploy", "update", "workflows", "agents", "hooks"} {
		t.Run(sub, func(t *testing.T) {
			help := helpText(t, sub)

			if strings.Contains(help, "locally-modified files") {
				t.Errorf("help for %s still says \"locally-modified files\":\n%s", sub, help)
			}
		})
	}
}

func TestAgentsLongHelp_OrchestratorConflictUsesCorrectedScope(t *testing.T) {
	// Long help is hard-wrapped, so compare with whitespace collapsed.
	help := strings.Join(strings.Fields(helpText(t, "agents")), " ")

	if strings.Contains(help, "locally modified orchestrator") {
		t.Errorf("agents help still contains \"locally modified orchestrator\":\n%s", help)
	}
	if !strings.Contains(help, "orchestrator the tool cannot confirm as its own") {
		t.Errorf("agents help does not state the corrected orchestrator scope:\n%s", help)
	}
	if !strings.Contains(help, "follows --conflict") {
		t.Errorf("agents help must still say the orchestrator follows --conflict:\n%s", help)
	}
}
