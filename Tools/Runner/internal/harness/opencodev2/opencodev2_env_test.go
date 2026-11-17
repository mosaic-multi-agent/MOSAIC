package opencodev2_test

// Tests for the environment and working directory every dispatch gives the V2
// child: the MOSAIC role environment (as for V1), the inherited environment,
// and a working directory that is the workspace root with a PWD that agrees
// with it, so V2 resolves the project from the right place.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/opencodev2"
	"mosaic-run/internal/harness/roleenv"
)

const (
	runIDConstructed = "20261003T185044Z-bbbb"
	runIDRequest     = "20261003T185044Z-aaaa"
)

func newRoleAdapter(t *testing.T, runID string) *opencodev2.OpenCodeV2Adapter {
	t.Helper()
	var opts []roleenv.Option
	if runID != "" {
		opts = append(opts, roleenv.WithRunID(runID))
	}
	return opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, nil, opts...)
}

// captureEnv makes the helper record its environment and clears the MOSAIC role
// variables from the test process so the child sees only what the adapter adds.
func captureEnv(t *testing.T, scenario string) (envFile string) {
	t.Helper()
	setHelperEnv(t, scenario)
	envFile = captureFile(t, "GO_HELPER_ENV_FILE", "env.json")
	for _, key := range []string{roleenv.EnvRunID, roleenv.EnvRole, roleenv.EnvAgentInstanceID} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck
	}
	return envFile
}

func assertEnvValue(t *testing.T, env map[string]string, key, want string) {
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

func invokeWithRun(t *testing.T, adapter *opencodev2.OpenCodeV2Adapter, instanceID, runID string) {
	t.Helper()
	req := minimalRequest(instanceID)
	req.RunID = runID
	if _, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), req); err != nil {
		t.Fatalf("unexpected Invoke error: %v", err)
	}
}

func invokeRawOK(t *testing.T, adapter *opencodev2.OpenCodeV2Adapter) {
	t.Helper()
	if _, err := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload); err != nil {
		t.Fatalf("unexpected InvokeRaw error: %v", err)
	}
}

func TestInvoke_SpawnCarriesSubagentRoleEnvironment(t *testing.T) {
	envFile := captureEnv(t, "success")

	invokeWithRun(t, newRoleAdapter(t, runIDConstructed), "Research#3", runIDRequest)

	env := readEnv(t, envFile)
	assertEnvValue(t, env, "MOSAIC_RUN_ID", runIDRequest)
	assertEnvValue(t, env, "MOSAIC_ROLE", "subagent")
	assertEnvValue(t, env, "MOSAIC_AGENT_INSTANCE_ID", "Research#3")
}

func TestInvoke_EmptyRequestRunID_FallsBackToConstructionRunID(t *testing.T) {
	envFile := captureEnv(t, "success")

	invokeWithRun(t, newRoleAdapter(t, runIDConstructed), "Research#3", "")

	assertEnvValue(t, readEnv(t, envFile), "MOSAIC_RUN_ID", runIDConstructed)
}

func TestInvoke_WithoutAnyRunID_OmitsRunIDButKeepsRole(t *testing.T) {
	envFile := captureEnv(t, "success")

	invokeWithRun(t, newRoleAdapter(t, ""), "Research#3", "")

	env := readEnv(t, envFile)
	assertEnvAbsent(t, env, "MOSAIC_RUN_ID")
	assertEnvValue(t, env, "MOSAIC_ROLE", "subagent")
	assertEnvValue(t, env, "MOSAIC_AGENT_INSTANCE_ID", "Research#3")
}

func TestInvokeRaw_SpawnCarriesOrchestratorRoleEnvironment(t *testing.T) {
	envFile := captureEnv(t, "raw-text")

	invokeRawOK(t, newRoleAdapter(t, runIDConstructed))

	env := readEnv(t, envFile)
	assertEnvValue(t, env, "MOSAIC_RUN_ID", runIDConstructed)
	assertEnvValue(t, env, "MOSAIC_ROLE", "orchestrator")
	assertEnvAbsent(t, env, "MOSAIC_AGENT_INSTANCE_ID")
}

func TestInvokeRaw_WithoutRunID_OmitsRunIDButKeepsRole(t *testing.T) {
	envFile := captureEnv(t, "raw-text")

	invokeRawOK(t, newRoleAdapter(t, ""))

	env := readEnv(t, envFile)
	assertEnvAbsent(t, env, "MOSAIC_RUN_ID")
	assertEnvValue(t, env, "MOSAIC_ROLE", "orchestrator")
}

func TestDispatches_KeepTheInheritedEnvironment(t *testing.T) {
	envFile := captureEnv(t, "success")
	t.Setenv("MOSAIC_TEST_INHERITED_MARKER", "kept-value")
	adapter := newRoleAdapter(t, runIDConstructed)

	invokeWithRun(t, adapter, "Research#3", runIDRequest)

	env := readEnv(t, envFile)
	assertEnvValue(t, env, "MOSAIC_TEST_INHERITED_MARKER", "kept-value")
	assertEnvValue(t, env, "MOSAIC_ROLE", "subagent")
}

func TestAdapter_RunID_ReturnsTheConstructionRunIDOrEmpty(t *testing.T) {
	if got := newRoleAdapter(t, runIDConstructed).RunID(); got != runIDConstructed {
		t.Errorf("RunID() = %q, want %q", got, runIDConstructed)
	}
	if got := newRoleAdapter(t, "").RunID(); got != "" {
		t.Errorf("RunID() without option = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Working directory and PWD
// ---------------------------------------------------------------------------

// assertProjectDir checks the recorded working directory is wantDir and that
// PWD, when present, names the same directory.
func assertProjectDir(t *testing.T, cwdFile, envFile, wantDir string) {
	t.Helper()
	cwd := string(readFileOrFail(t, cwdFile))
	if !sameDir(cwd, wantDir) {
		t.Errorf("working directory = %q, want %q", cwd, wantDir)
	}
	if pwd, ok := readEnv(t, envFile)["PWD"]; ok && !sameDir(pwd, cwd) {
		t.Errorf("PWD = %q disagrees with the working directory %q", pwd, cwd)
	}
}

func sameDir(a, b string) bool {
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}

// staleDir plants a PWD that names a different directory, as a shell would
// leave behind when the Runner was started elsewhere.
func staleDir(t *testing.T) {
	t.Helper()
	t.Setenv("PWD", t.TempDir())
}

func TestInvoke_RunsInTheWorkspaceRootWithAgreeingPWD(t *testing.T) {
	envFile := captureEnv(t, "success")
	cwdFile := captureFile(t, "GO_HELPER_CWD_FILE", "cwd.txt")
	staleDir(t)
	root := t.TempDir()

	invokeWithRun(t, newRoleAdapter(t, runIDConstructed).WithWorkspaceRoot(root), "Research#3", runIDRequest)

	assertProjectDir(t, cwdFile, envFile, root)
}

func TestInvokeRaw_RunsInTheWorkspaceRootWithAgreeingPWD(t *testing.T) {
	envFile := captureEnv(t, "raw-text")
	cwdFile := captureFile(t, "GO_HELPER_CWD_FILE", "cwd.txt")
	staleDir(t)
	root := t.TempDir()

	invokeRawOK(t, newRoleAdapter(t, runIDConstructed).WithWorkspaceRoot(root))

	assertProjectDir(t, cwdFile, envFile, root)
}

func TestInvoke_WithoutWorkspaceRoot_RunsInTheCurrentDirectoryWithAgreeingPWD(t *testing.T) {
	envFile := captureEnv(t, "success")
	cwdFile := captureFile(t, "GO_HELPER_CWD_FILE", "cwd.txt")
	staleDir(t)
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	invokeWithRun(t, newRoleAdapter(t, runIDConstructed), "Research#3", runIDRequest)

	assertProjectDir(t, cwdFile, envFile, here)
}

func TestInvoke_EmptyWorkspaceRoot_MeansTheCurrentDirectory(t *testing.T) {
	envFile := captureEnv(t, "success")
	cwdFile := captureFile(t, "GO_HELPER_CWD_FILE", "cwd.txt")
	here, _ := os.Getwd()

	invokeWithRun(t, newRoleAdapter(t, runIDConstructed).WithWorkspaceRoot(""), "Research#3", runIDRequest)

	assertProjectDir(t, cwdFile, envFile, here)
}

func TestInvoke_EnvironmentBlockNamesTheWorkspaceRootNotAStaleDirectory(t *testing.T) {
	setHelperEnv(t, "success")
	stdinFile := captureFile(t, "GO_HELPER_STDIN_FILE", "stdin.txt")
	staleDir(t)
	root := t.TempDir()

	_, err := newAdapter(t, 10*time.Second).WithWorkspaceRoot(root).Invoke(context.Background(), ordinaryAgentRef(), domain.ProtocolRequest{
		AgentInstanceID: "a#1", TaskDescription: "x", InputArtifacts: []string{}, OutputArtifacts: []string{},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdin := string(readFileOrFail(t, stdinFile)); !strings.Contains(stdin, "Working directory: "+root) {
		t.Errorf("want the <env> block to name %q, got %q", root, stdin)
	}
}
