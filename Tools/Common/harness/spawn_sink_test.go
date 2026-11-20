package harness_test

// Tests that SpawnRequest.Sink, when set, receives a call's Run events instead
// of the spawner's configured sink, for each of the five spawners, and that a
// request without it keeps using the configured sink.

import (
	"context"
	"testing"
	"time"

	"mosaic-common/harness"
)

type sinkSpawnerCase struct {
	name       string
	helperMode string
	newSpawner func(t *testing.T, configured harness.Sink) harness.Spawner
	request    func(t *testing.T) harness.SpawnRequest
}

func sinkSpawnerCases() []sinkSpawnerCase {
	timeout := harness.WithTimeout(20 * time.Second)
	return []sinkSpawnerCase{
		{
			name: "claude-code", helperMode: "claude-stream-gated",
			newSpawner: func(t *testing.T, s harness.Sink) harness.Spawner {
				return harness.NewClaudeCode(helperExe(t), timeout, harness.WithSink(s))
			},
			request: func(t *testing.T) harness.SpawnRequest { return minimalRequest(ordinaryAgent()) },
		},
		{
			name: "codex", helperMode: "codex-success",
			newSpawner: func(t *testing.T, s harness.Sink) harness.Spawner {
				return harness.NewCodex(helperExe(t), timeout, harness.WithSink(s))
			},
			request: func(t *testing.T) harness.SpawnRequest {
				agentPath, _ := copyCodexAgent(t, "agent_full.toml")
				return codexRequest(t, agentPath)
			},
		},
		{
			name: "ghcp-cli", helperMode: "ghcpcli-stream-gated",
			newSpawner: func(t *testing.T, s harness.Sink) harness.Spawner {
				return harness.NewGHCPCLI(helperExe(t), timeout, harness.WithSink(s))
			},
			request: func(t *testing.T) harness.SpawnRequest { return minimalGHCPCLISpawnRequest(ordinaryAgent()) },
		},
		{
			name: "opencode", helperMode: "opencode-stream-gated",
			newSpawner: func(t *testing.T, s harness.Sink) harness.Spawner {
				return harness.NewOpenCode(helperExe(t), timeout, harness.WithSink(s))
			},
			request: func(t *testing.T) harness.SpawnRequest { return minimalOpenCodeSpawnRequest(ordinaryAgent()) },
		},
		{
			name: "opencode-v2", helperMode: "v2-success",
			newSpawner: func(t *testing.T, s harness.Sink) harness.Spawner {
				return harness.NewOpenCodeV2(helperExe(t), timeout, harness.WithSink(s))
			},
			request: func(t *testing.T) harness.SpawnRequest {
				return harness.SpawnRequest{Agent: ordinaryAgent(), Prompt: "go"}
			},
		},
	}
}

func TestSpawners_RequestSink_ReceivesRunEventsInsteadOfConfiguredSink(t *testing.T) {
	for _, tc := range sinkSpawnerCases() {
		t.Run(tc.name, func(t *testing.T) {
			setHelperEnv(t, tc.helperMode)
			configured, perCall := &recordingSink{}, &recordingSink{}
			req := tc.request(t)
			req.Sink = perCall

			if _, err := tc.newSpawner(t, configured).Spawn(context.Background(), req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			events := perCall.snapshot()
			if len(liveLines(t, events)) == 0 {
				t.Errorf("want the request sink to receive stdout lines, got %v", eventNames(events))
			}
			if end := lastEvent(t, events); end.Name != "spawn.end" {
				t.Errorf("want the request sink to end with spawn.end, got %v", eventNames(events))
			}
			if got := configured.snapshot(); len(got) != 0 {
				t.Errorf("want the configured sink untouched, got %v", eventNames(got))
			}
		})
	}
}

func TestSpawners_WithoutRequestSink_UsesConfiguredSink(t *testing.T) {
	for _, tc := range sinkSpawnerCases() {
		t.Run(tc.name, func(t *testing.T) {
			setHelperEnv(t, tc.helperMode)
			configured := &recordingSink{}

			if _, err := tc.newSpawner(t, configured).Spawn(context.Background(), tc.request(t)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if end := lastEvent(t, configured.snapshot()); end.Name != "spawn.end" {
				t.Errorf("want the configured sink to end with spawn.end, got %v", eventNames(configured.snapshot()))
			}
		})
	}
}
