package codex_test

// Tests for the MOSAIC role environment the Codex adapter gives every process
// it spawns: protocol invocations carry the subagent role, consultations carry
// the orchestrator role, and both keep the inherited environment.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/harness/codex"
	"mosaic-run/internal/harness/roleenv"
)

const (
	constructedRunID = "20261003T185044Z-bbbb"
	requestRunID     = "20261003T185044Z-aaaa"
)

// captureSpawnEnv makes the fake CLI record its environment after clearing
// the MOSAIC role variables, so only the adapter can supply them.
func captureSpawnEnv(t *testing.T, scenario string) (envFile string) {
	t.Helper()
	setHelperEnv(t, scenario)
	envFile = filepath.Join(t.TempDir(), "env.json")
	t.Setenv("GO_HELPER_ENV_FILE", envFile)
	for _, key := range []string{roleenv.EnvRunID, roleenv.EnvRole, roleenv.EnvAgentInstanceID} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck
	}
	return envFile
}

func readSpawnEnv(t *testing.T, envFile string) map[string]string {
	t.Helper()
	var entries []string
	if err := json.Unmarshal([]byte(readFile(t, envFile)), &entries); err != nil {
		t.Fatalf("decode recorded environment: %v", err)
	}
	env := map[string]string{}
	for _, e := range entries {
		if i := strings.Index(e, "="); i > 0 {
			env[e[:i]] = e[i+1:]
		}
	}
	return env
}

func assertEnv(t *testing.T, env map[string]string, key, want string) {
	t.Helper()
	if got, ok := env[key]; !ok || got != want {
		t.Errorf("%s = %q (present=%v), want %q", key, got, ok, want)
	}
}

func assertEnvAbsent(t *testing.T, env map[string]string, key string) {
	t.Helper()
	if got, ok := env[key]; ok {
		t.Errorf("%s = %q is present, want it absent", key, got)
	}
}

func adapterWithRunID(t *testing.T, runID string) *codex.CodexAdapter {
	t.Helper()
	var opts []roleenv.Option
	if runID != "" {
		opts = append(opts, roleenv.WithRunID(runID))
	}
	return codex.NewCodexAdapterWithLogger(helperExe(t), adapterTimeout, nil, opts...)
}

func TestCodexAdapter_Invoke_SpawnCarriesSubagentRoleEnvironment(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-success")
	req := request("Research#3")
	req.RunID = requestRunID

	if _, err := adapterWithRunID(t, constructedRunID).Invoke(context.Background(), agentRef(t), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := readSpawnEnv(t, envFile)
	assertEnv(t, env, "MOSAIC_RUN_ID", requestRunID)
	assertEnv(t, env, "MOSAIC_ROLE", "subagent")
	assertEnv(t, env, "MOSAIC_AGENT_INSTANCE_ID", "Research#3")
}

func TestCodexAdapter_Invoke_EmptyRequestRunID_FallsBackToConstructionRunID(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-success")

	if _, err := adapterWithRunID(t, constructedRunID).Invoke(context.Background(), agentRef(t), request("Research#3")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, readSpawnEnv(t, envFile), "MOSAIC_RUN_ID", constructedRunID)
}

func TestCodexAdapter_Invoke_WithoutAnyRunID_OmitsRunIDButKeepsRole(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-success")

	if _, err := adapterWithRunID(t, "").Invoke(context.Background(), agentRef(t), request("Research#3")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := readSpawnEnv(t, envFile)
	assertEnvAbsent(t, env, "MOSAIC_RUN_ID")
	assertEnv(t, env, "MOSAIC_ROLE", "subagent")
}

func TestCodexAdapter_Invoke_KeepsInheritedEnvironment(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-success")
	t.Setenv("MOSAIC_TEST_INHERITED_MARKER", "kept-value")

	if _, err := adapterWithRunID(t, constructedRunID).Invoke(context.Background(), agentRef(t), request("Research#3")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEnv(t, readSpawnEnv(t, envFile), "MOSAIC_TEST_INHERITED_MARKER", "kept-value")
}

func TestCodexAdapter_InvokeRaw_SpawnCarriesOrchestratorRoleEnvironment(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-raw-success")

	if _, err := adapterWithRunID(t, constructedRunID).InvokeRaw(context.Background(), agentRef(t), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := readSpawnEnv(t, envFile)
	assertEnv(t, env, "MOSAIC_RUN_ID", constructedRunID)
	assertEnv(t, env, "MOSAIC_ROLE", "orchestrator")
	assertEnvAbsent(t, env, "MOSAIC_AGENT_INSTANCE_ID")
}

func TestCodexAdapter_InvokeRaw_WithoutRunID_OmitsRunIDButKeepsRole(t *testing.T) {
	envFile := captureSpawnEnv(t, "codex-raw-success")

	if _, err := adapterWithRunID(t, "").InvokeRaw(context.Background(), agentRef(t), rawPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := readSpawnEnv(t, envFile)
	assertEnvAbsent(t, env, "MOSAIC_RUN_ID")
	assertEnv(t, env, "MOSAIC_ROLE", "orchestrator")
}

func TestCodexAdapter_RunID_ReturnsTheConstructionRunID(t *testing.T) {
	if got := adapterWithRunID(t, constructedRunID).RunID(); got != constructedRunID {
		t.Errorf("RunID() = %q, want %q", got, constructedRunID)
	}
	if got := adapterWithRunID(t, "").RunID(); got != "" {
		t.Errorf("RunID() without option = %q, want empty", got)
	}
}
