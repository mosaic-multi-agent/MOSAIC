package harness_test

// Tests that the no-output watchdog is enabled through SpawnRequest by the
// OpenCode V2 spawner only. Every other spawner ignores SpawnRequest.NoOutputTimeout.

import (
	"context"
	"errors"
	"testing"
	"time"

	"mosaic-common/harness"
)

func TestOpenCodeV2Spawn_NoOutputTimeoutEndsSilentInvocation(t *testing.T) {
	_, _, err := spawnV2(t, "hang", harness.SpawnRequest{Prompt: "go", NoOutputTimeout: time.Second, Timeout: 60 * time.Second})

	if !errors.Is(err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
	if !errors.Is(err, harness.ErrTimeout) {
		t.Errorf("want the watchdog error to also satisfy ErrTimeout, got %v", err)
	}
}

func TestOpenCodeV2Spawn_NoOutputTimeoutEndsInvocationThatWentSilentAfterOutput(t *testing.T) {
	_, _, err := spawnV2(t, "wd-v2-start-then-hang", harness.SpawnRequest{Prompt: "go", NoOutputTimeout: time.Second, Timeout: 60 * time.Second})

	if !errors.Is(err, harness.ErrNoOutput) {
		t.Fatalf("want ErrNoOutput, got %v", err)
	}
}

func TestOpenCodeV2Spawn_NoOutputTimeoutDoesNotAffectInvocationThatAnswers(t *testing.T) {
	// regression: a watchdog far longer than the run changes nothing.
	resp, _, err := spawnV2(t, "v2-success", harness.SpawnRequest{Prompt: "go", NoOutputTimeout: 30 * time.Second})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Protocol) != v2HelperProtocol {
		t.Errorf("want protocol %q, got %q", v2HelperProtocol, resp.Protocol)
	}
}

func TestOpenCodeV2Spawn_WithoutNoOutputTimeoutSilenceEndsOnlyByTimeout(t *testing.T) {
	_, _, err := spawnV2(t, "hang", harness.SpawnRequest{Prompt: "go", Timeout: time.Second})

	if !errors.Is(err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if errors.Is(err, harness.ErrNoOutput) {
		t.Errorf("no watchdog was requested; want no ErrNoOutput, got %v", err)
	}
}

// otherSpawnerCases runs each non-V2 spawner against a silent process with a
// 500 ms NoOutputTimeout in the request and a 2 s wall-clock timeout. A spawner
// that ignores the field ends only by the wall-clock timeout.
func TestOtherSpawners_IgnoreNoOutputTimeout(t *testing.T) {
	type spawnCase struct {
		name  string
		build func(t *testing.T) (harness.Spawner, harness.SpawnRequest)
	}
	cases := []spawnCase{
		{"claude-code", func(t *testing.T) (harness.Spawner, harness.SpawnRequest) {
			return harness.NewClaudeCode(helperExe(t)), harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "go"}
		}},
		{"opencode", func(t *testing.T) (harness.Spawner, harness.SpawnRequest) {
			return harness.NewOpenCode(helperExe(t)), harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "go"}
		}},
		{"ghcp-cli", func(t *testing.T) (harness.Spawner, harness.SpawnRequest) {
			return harness.NewGHCPCLI(helperExe(t)), harness.SpawnRequest{
				Agent: ordinaryAgent(), Prompt: "go", GHCPCLIMode: harness.GHCPCLIModeBlanket,
			}
		}},
		{"codex", func(t *testing.T) (harness.Spawner, harness.SpawnRequest) {
			agentPath, _ := copyCodexAgent(t, "agent_full.toml")
			return harness.NewCodex(helperExe(t)), codexRequest(t, agentPath)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setHelperEnv(t, "hang")
			spawner, req := c.build(t)
			req.Timeout = 2 * time.Second
			req.NoOutputTimeout = 500 * time.Millisecond

			_, err := spawner.Spawn(context.Background(), req)

			if !errors.Is(err, harness.ErrTimeout) {
				t.Fatalf("want ErrTimeout from the wall-clock timeout, got %v", err)
			}
			if errors.Is(err, harness.ErrNoOutput) {
				t.Errorf("this spawner must ignore NoOutputTimeout; got %v", err)
			}
		})
	}
}
