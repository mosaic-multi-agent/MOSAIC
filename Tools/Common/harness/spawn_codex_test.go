package harness_test

// Fake-CLI tests for the Codex Spawner (helper-process re-exec pattern, see
// helper_test.go and helper_codex_test.go). They check the observable
// invocation shape, every end-of-invocation error kind, and that the agent file
// and workspace are left untouched.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mosaic-common/agentload"
	"mosaic-common/harness"
)

// copyCodexAgent copies an agent fixture into a temp dir and returns the copy's
// path and bytes, so a test can prove the invocation left the file unmodified.
func copyCodexAgent(t *testing.T, name string) (string, []byte) {
	t.Helper()
	data, err := os.ReadFile(codexAgentPath(name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return path, data
}

// codexRequest returns a request for the agent file at path, run in a fresh
// empty working directory.
func codexRequest(t *testing.T, agentPath string) harness.SpawnRequest {
	t.Helper()
	return harness.SpawnRequest{
		Agent:      harness.AgentRef{DefinitionPath: agentPath},
		Prompt:     "do the work",
		WorkingDir: t.TempDir(),
		Timeout:    30 * time.Second,
	}
}

func TestCodexSpawn_CompletedTurn_ReturnsProtocolAndText(t *testing.T) {
	setHelperEnv(t, "codex-success")
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")

	resp, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, agentPath))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != codexProtocolResp {
		t.Errorf("Text = %q, want the final agent message", resp.Text)
	}
	if !strings.Contains(string(resp.Protocol), `"status_code":"SUCCESS"`) {
		t.Errorf("Protocol = %s, want the extracted protocol JSON", resp.Protocol)
	}
}

func TestCodexSpawn_Invocation_AppliesAgentThroughArgvAndStdin(t *testing.T) {
	argsFile := setHelperEnv(t, "codex-success")
	stdinFile := filepath.Join(t.TempDir(), "stdin.txt")
	cwdFile := filepath.Join(t.TempDir(), "cwd.txt")
	t.Setenv("GO_HELPER_STDIN_FILE", stdinFile)
	t.Setenv("GO_HELPER_CWD_FILE", cwdFile)
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)

	if _, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	if len(args) == 0 || args[0] != "exec" || !containsArg(args, "--json") {
		t.Errorf("want a codex exec --json invocation, got %v", args)
	}
	if !containsSequence(args, "-m", "gpt-5") || !containsSequence(args, "-s", "workspace-write") {
		t.Errorf("want the agent's model and sandbox in argv, got %v", args)
	}
	requireOverride(t, args, "approval_policy", `approval_policy="never"`)
	for _, a := range args {
		if strings.ContainsAny(a, "\r\n") || strings.Contains(a, fullAgentInstruction) {
			t.Errorf("argv element carries a newline or instruction text: %q", a)
		}
	}
	stdin, _ := os.ReadFile(stdinFile)
	if !strings.Contains(string(stdin), fullAgentInstruction) || !strings.HasSuffix(string(stdin), "do the work") {
		t.Errorf("stdin must carry the instructions and the prompt, got %q", stdin)
	}
	cwd, _ := os.ReadFile(cwdFile)
	if resolved, _ := filepath.EvalSymlinks(string(cwd)); filepath.Base(resolved) != filepath.Base(req.WorkingDir) {
		t.Errorf("process ran in %q, want the working directory %q", cwd, req.WorkingDir)
	}
}

func TestCodexSpawn_Invocation_LeavesAgentFileAndWorkspaceUntouched(t *testing.T) {
	setHelperEnv(t, "codex-success")
	agentPath, original := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)

	if _, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(agentPath)
	if err != nil || string(after) != string(original) {
		t.Errorf("agent file changed by the invocation (err %v)", err)
	}
	if entries, _ := os.ReadDir(req.WorkingDir); len(entries) != 0 {
		t.Errorf("the invocation wrote into the workspace: %v", entries)
	}
}

func TestCodexSpawn_ExecutableMissing_IsExecutableNotFound(t *testing.T) {
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")

	_, err := harness.NewCodex(filepath.Join(t.TempDir(), "no-such-codex")).Spawn(context.Background(), codexRequest(t, agentPath))

	if !errors.Is(err, harness.ErrExecutableNotFound) {
		t.Fatalf("want ErrExecutableNotFound, got %v", err)
	}
}

func TestCodexSpawn_AgentFileCannotBeLoaded_IsAgentUnreadableAndNothingRuns(t *testing.T) {
	cases := map[string]string{
		"missing path":   filepath.Join(t.TempDir(), "absent.toml"),
		"empty path":     "",
		"malformed toml": filepath.Join("..", "agentload", "testdata", "codex", "malformed.toml"),
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			argsFile := setHelperEnv(t, "codex-success")

			_, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, path))

			if !errors.Is(err, harness.ErrCodexAgentUnreadable) {
				t.Fatalf("want ErrCodexAgentUnreadable, got %v", err)
			}
			if _, statErr := os.Stat(argsFile); statErr == nil {
				t.Errorf("Codex was started although its agent file cannot be loaded")
			}
		})
	}
}

func TestCodexSpawn_UnreadableAgent_WrapsTheLoadError(t *testing.T) {
	setHelperEnv(t, "codex-success")

	_, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, filepath.Join(t.TempDir(), "absent.toml")))

	var loadErr *agentload.LoadError
	if !errors.As(err, &loadErr) {
		t.Errorf("want the *agentload.LoadError reachable through the error, got %v", err)
	}
}

func TestCodexSpawn_NonZeroExit_IsNonZeroExit(t *testing.T) {
	setHelperEnv(t, "codex-exit1")
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")

	resp, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, agentPath))

	if !errors.Is(err, harness.ErrNonZeroExit) {
		t.Fatalf("want ErrNonZeroExit, got %v", err)
	}
	if errors.Is(err, harness.ErrCodexStreamError) {
		t.Errorf("no failure event was reported, so no stream error is expected: %v", err)
	}
	if resp.ExitCode != 1 || !strings.Contains(resp.Stderr, "codex crashed") {
		t.Errorf("want the populated Response alongside the error, got %+v", resp)
	}
}

func TestCodexSpawn_NonZeroExitWithFailedTurn_WrapsBothAndNamesTheMessage(t *testing.T) {
	setHelperEnv(t, "codex-turn-failed-exit1")
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")

	_, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, agentPath))

	if !errors.Is(err, harness.ErrNonZeroExit) || !errors.Is(err, harness.ErrCodexStreamError) {
		t.Fatalf("want ErrNonZeroExit and ErrCodexStreamError, got %v", err)
	}
	if !strings.Contains(err.Error(), "simulated model failure") {
		t.Errorf("want the failure message in the error, got %v", err)
	}
}

func TestCodexSpawn_StreamFailureShapes_MapToExistingKinds(t *testing.T) {
	cases := []struct {
		mode string
		want error
	}{
		{"codex-turn-failed-exit0", harness.ErrCodexStreamError},
		{"codex-incomplete", harness.ErrCodexStreamIncomplete},
		{"codex-empty", harness.ErrEmptyResponse},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			setHelperEnv(t, tc.mode)
			agentPath, _ := copyCodexAgent(t, "agent_full.toml")

			resp, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), codexRequest(t, agentPath))

			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if tc.want != harness.ErrEmptyResponse && len(resp.Stdout) == 0 {
				t.Errorf("want the populated Response alongside the error, got %+v", resp)
			}
		})
	}
}
