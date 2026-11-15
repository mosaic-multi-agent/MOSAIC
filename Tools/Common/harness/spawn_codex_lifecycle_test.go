package harness_test

// Lifecycle tests for the Codex Spawner with a fake Codex whose descendants hold
// its output: the invocation ends when Codex exits, and a timeout or a
// cancellation leaves no descendant process behind.

import (
	"context"
	"errors"
	"testing"
	"time"

	"mosaic-common/harness"
)

func TestCodexSpawn_CodexExitsWhileDescendantHoldsStdout_ReturnsPromptlyWithResult(t *testing.T) {
	setHelperEnv(t, "codex-exit-holds-stdout")
	pidFile := newPidFile(t)
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)
	req.Timeout = 60 * time.Second

	type result struct {
		resp harness.Response
		err  error
		took time.Duration
	}
	ch := make(chan result, 1)
	go func() {
		start := time.Now()
		resp, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), req)
		ch <- result{resp, err, time.Since(start)}
	}()
	var r result
	select {
	case r = <-ch:
	case <-time.After(returnPromptly):
		t.Fatalf("Spawn did not return within %s of Codex exiting: blocked on output held by a descendant", returnPromptly)
	}

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if r.resp.Text != codexProtocolResp {
		t.Errorf("Text = %q, want the final agent message", r.resp.Text)
	}
	if r.took > heldOutputBudget {
		t.Errorf("Spawn took %s; want within %s of Codex exiting", r.took, heldOutputBudget)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestCodexSpawn_Timeout_EndsDescendantsAndReportsTimeout(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)
	req.Timeout = 3 * time.Second

	start := time.Now()
	_, err := harness.NewCodex(helperExe(t)).Spawn(context.Background(), req)

	if !errors.Is(err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	if took := time.Since(start); took > req.Timeout+3*time.Second {
		t.Errorf("Spawn took %s for a %s timeout", took, req.Timeout)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestCodexSpawn_Cancellation_EndsDescendantsAndReportsCancelled(t *testing.T) {
	setHelperEnv(t, "tree-agent")
	pidFile := newPidFile(t)
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)
	req.Timeout = 60 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		deadline := time.Now().Add(pidFileWait)
		for time.Now().Before(deadline) {
			if _, ok := readPid(pidFile); ok {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()

	_, err := harness.NewCodex(helperExe(t)).Spawn(ctx, req)

	if !errors.Is(err, harness.ErrCancelled) || errors.Is(err, harness.ErrTimeout) {
		t.Fatalf("want ErrCancelled only, got %v", err)
	}
	requireGone(t, waitForPid(t, pidFile))
}

func TestCodexSpawn_WithTimeoutOption_AppliesWhenRequestHasNone(t *testing.T) {
	setHelperEnv(t, "hang")
	agentPath, _ := copyCodexAgent(t, "agent_full.toml")
	req := codexRequest(t, agentPath)
	req.Timeout = 0

	_, err := harness.NewCodex(helperExe(t), harness.WithTimeout(time.Second)).Spawn(context.Background(), req)

	if !errors.Is(err, harness.ErrTimeout) {
		t.Fatalf("want ErrTimeout from the spawner timeout, got %v", err)
	}
}
