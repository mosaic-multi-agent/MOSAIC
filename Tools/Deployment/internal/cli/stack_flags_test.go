package cli_test

// stack_flags_test.go verifies the catalog-stack flags that cobra must accept on every
// subcommand: a repeatable --catalog-folder and the bool --include-default-catalog. Cobra
// re-parses the arguments the composition root already pre-scanned, so a usage error here
// would turn a valid stack request into a failed run.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/cli"
)

// stackFlagSubcommands returns, per catalog-loading subcommand, its own arguments (no stack
// flags). The same shapes the catalog-folder tests use.
func stackFlagSubcommands(workspace string) map[string][]string {
	return map[string][]string{
		"deploy":      {"deploy", "--harness", "stub-harness", "--workspace", workspace, "--auto-confirm"},
		"deploy dry":  {"deploy", "--harness", "stub-harness", "--workspace", workspace, "--auto-confirm", "--dry-run"},
		"update":      {"update", "--harness", "stub-harness", "--workspace", workspace, "--auto-confirm"},
		"workflows":   {"workflows", "--harness", "stub-harness", "--workspace", workspace, "--workflows", "a", "--auto-confirm"},
		"agents":      {"agents", "--harness", "stub-harness", "--workspace", workspace, "--subagents", "", "--yes"},
		"hooks":       {"hooks", "--harness", "stub-harness", "--workspace", workspace, "--hooks", "", "--yes"},
		"render":      {"render", "--harness", "stub-harness", "--source-agent", "orchestrator", "--dest", "/tmp/output.md"},
		"promote":     {"promote", "--file", "/nonexistent/agent.md", "--harness", "stub-harness"},
		"transform":   {"transform", "--harness", "stub-harness", "--target-harness", "other-harness", "--path", "/nonexistent/agent.md"},
		"check-index": {"check-index"},
	}
}

// runCode runs the CLI with a fresh spy service and returns the exit code.
func runCode(t *testing.T, args []string) int {
	t.Helper()
	svc := &spyService{deployResp: successSummary("w")}
	return cli.Run(context.Background(), args, svc, &bytes.Buffer{}, &bytes.Buffer{})
}

// assertNoUsageErrorAdded fails when args with the stack flags spliced in end in ExitUsage
// while the same arguments without them do not.
func assertNoUsageErrorAdded(t *testing.T, base, stackFlags []string) {
	t.Helper()
	without := runCode(t, base)
	with := runCode(t, append(append([]string{}, stackFlags...), base...))
	if with == cli.ExitUsage && without != cli.ExitUsage {
		t.Errorf("exit code %d (usage error) with %v before %v, but %d without them; the stack flags must be accepted",
			with, stackFlags, base, without)
	}
}

func TestStackFlags_IncludeDefaultCatalog_AcceptedOnEverySubcommand(t *testing.T) {
	forms := map[string][]string{
		"bare":         {"--include-default-catalog"},
		"equals true":  {"--include-default-catalog=true"},
		"equals false": {"--include-default-catalog=false"},
	}
	for name, args := range stackFlagSubcommands(t.TempDir()) {
		for form, flags := range forms {
			t.Run(name+"/"+form, func(t *testing.T) {
				assertNoUsageErrorAdded(t, args, flags)
			})
		}
	}
}

func TestStackFlags_IncludeDefaultCatalog_BareFlagDoesNotSwallowTheSubcommand(t *testing.T) {
	workspace := t.TempDir()
	svc := &spyService{deployResp: successSummary(workspace)}

	code := cli.Run(context.Background(),
		[]string{"--include-default-catalog", "deploy", "--harness", "stub-harness", "--workspace", workspace, "--auto-confirm"},
		svc, &bytes.Buffer{}, &bytes.Buffer{})

	if code == cli.ExitUsage || svc.deployReq == nil {
		t.Fatalf("exit code %d, deploy request %v; the token after a bare --include-default-catalog must remain the subcommand",
			code, svc.deployReq)
	}
	if svc.deployReq.WorkspacePath != workspace {
		t.Errorf("deploy workspace = %q, want %q (stack flags must not alter other flags)", svc.deployReq.WorkspacePath, workspace)
	}
}

func TestStackFlags_CatalogFolder_RepeatedAcceptedOnEverySubcommand(t *testing.T) {
	a, b := catalogFolderDir(t), catalogFolderDir(t)
	for name, args := range stackFlagSubcommands(t.TempDir()) {
		t.Run(name, func(t *testing.T) {
			assertNoUsageErrorAdded(t, args, []string{"--catalog-folder", a, "--catalog-folder=" + b})
		})
	}
}

func TestStackFlags_CatalogFolderAndDefaultFlag_AcceptedTogetherOnEverySubcommand(t *testing.T) {
	a, b := catalogFolderDir(t), catalogFolderDir(t)
	for name, args := range stackFlagSubcommands(t.TempDir()) {
		t.Run(name, func(t *testing.T) {
			assertNoUsageErrorAdded(t, args,
				[]string{"--catalog-folder", a, "--include-default-catalog", "--catalog-folder", b})
		})
	}
}

func TestStackFlags_IncludeDefaultCatalog_InvalidValueIsAUsageError(t *testing.T) {
	code := runCode(t, []string{"--include-default-catalog=banana", "deploy", "--harness", "h", "--workspace", t.TempDir(), "--auto-confirm"})

	if code != cli.ExitUsage {
		t.Errorf("exit code %d, want ExitUsage (%d) for a bool value cobra rejects", code, cli.ExitUsage)
	}
}

func TestStackFlags_Help_DocumentsPrecedenceAndTheDefaultLayerRules(t *testing.T) {
	var errOut bytes.Buffer

	cli.Run(context.Background(), []string{"--help"}, &spyService{}, &bytes.Buffer{}, &errOut)

	help := strings.ToLower(errOut.String())
	for _, want := range []string{"--include-default-catalog", "highest", "default"} {
		if !strings.Contains(help, want) {
			t.Errorf("help output does not mention %q:\n%s", want, errOut.String())
		}
	}
	if !strings.Contains(help, "alone") {
		t.Errorf("help output does not say that --include-default-catalog alone gives the default catalog alone:\n%s", errOut.String())
	}
}
