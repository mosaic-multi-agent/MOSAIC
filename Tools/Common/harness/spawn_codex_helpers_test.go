package harness_test

// Shared helpers of the Codex argument-builder and spawner tests.

import (
	"path/filepath"
	"strings"
	"testing"

	"mosaic-common/agentload"
	"mosaic-common/harness"
)

// codexAgentPath returns the path of a Codex agent fixture under testdata/codex.
func codexAgentPath(name string) string {
	return filepath.Join("testdata", "codex", name)
}

// loadCodexAgent loads a Codex agent fixture through the read-only loader.
func loadCodexAgent(t *testing.T, name string) *agentload.Agent {
	t.Helper()
	agent, err := agentload.Load(codexAgentPath(name), "codex-toml")
	if err != nil {
		t.Fatalf("load codex fixture %s: %v", name, err)
	}
	return agent
}

// buildCodex builds the invocation for req and fails the test on error.
func buildCodex(t *testing.T, req harness.SpawnRequest, agent *agentload.Agent) ([]string, []byte) {
	t.Helper()
	args, stdin, err := harness.BuildCodexArgs(req, agent)
	if err != nil {
		t.Fatalf("BuildCodexArgs: %v", err)
	}
	return args, stdin
}

// configOverrides returns the value of every `-c <value>` pair in args, in
// argv order.
func configOverrides(args []string) []string {
	var out []string
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" {
			out = append(out, args[i+1])
		}
	}
	return out
}

// overridesFor returns the `-c` values whose config path is exactly path
// (the text before the first '=' outside quotes is the path).
func overridesFor(args []string, path string) []string {
	var out []string
	for _, kv := range configOverrides(args) {
		if strings.HasPrefix(kv, path+"=") {
			out = append(out, kv)
		}
	}
	return out
}

// requireOverride fails unless exactly one `-c` value for path exists and it
// equals want, and returns nothing.
func requireOverride(t *testing.T, args []string, path, want string) {
	t.Helper()
	got := overridesFor(args, path)
	if len(got) != 1 {
		t.Fatalf("want exactly one -c override for %s, got %v (argv %v)", path, got, args)
	}
	if got[0] != want {
		t.Errorf("override for %s = %q, want %q", path, got[0], want)
	}
}
