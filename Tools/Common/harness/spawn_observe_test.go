package harness_test

// Tests that SpawnRequest.OnStdoutLine reaches Run from each of the three
// Common spawners (Claude Code, OpenCode, GHCP CLI), and that a request without
// it behaves as before.
//
// Each spawner is driven against a gated helper that emits a valid stream for
// that harness: the helper writes its first line, then waits for a file the
// observer creates on receiving it. The remainder of the stream, and so a
// successful Spawn, is only possible if the first line was delivered while the
// process was still running.

import (
	"context"
	"strings"
	"testing"
	"time"

	"mosaic-common/harness"
)

type spawnerCase struct {
	name      string
	helperCmd string
	// firstLine is the first stdout line the helper writes.
	firstLine string
	// newSpawner builds the spawner under test.
	newSpawner func(t *testing.T, sink harness.Sink) harness.Spawner
	request    func() harness.SpawnRequest
}

func spawnerCases() []spawnerCase {
	return []spawnerCase{
		{
			name:      "claude-code",
			helperCmd: "claude-stream-gated",
			firstLine: "[",
			newSpawner: func(t *testing.T, sink harness.Sink) harness.Spawner {
				return harness.NewClaudeCode(helperExe(t), harness.WithTimeout(20*time.Second), harness.WithSink(sink))
			},
			request: func() harness.SpawnRequest { return minimalRequest(ordinaryAgent()) },
		},
		{
			name:      "opencode",
			helperCmd: "opencode-stream-gated",
			firstLine: `{"type":"step_start"}`,
			newSpawner: func(t *testing.T, sink harness.Sink) harness.Spawner {
				return harness.NewOpenCode(helperExe(t), harness.WithTimeout(20*time.Second), harness.WithSink(sink))
			},
			request: func() harness.SpawnRequest { return minimalOpenCodeSpawnRequest(ordinaryAgent()) },
		},
		{
			name:      "ghcp-cli",
			helperCmd: "ghcpcli-stream-gated",
			firstLine: `{"type":"session.mcp_servers_loaded","data":{},"id":"id0","timestamp":"2026-08-12T21:00:00Z","parentId":"pid1"}`,
			newSpawner: func(t *testing.T, sink harness.Sink) harness.Spawner {
				return harness.NewGHCPCLI(helperExe(t), harness.WithTimeout(20*time.Second), harness.WithSink(sink))
			},
			request: func() harness.SpawnRequest { return minimalGHCPCLISpawnRequest(ordinaryAgent()) },
		},
	}
}

func TestSpawners_OnStdoutLine_DeliversLinesWhileProcessRuns(t *testing.T) {
	for _, tc := range spawnerCases() {
		t.Run(tc.name, func(t *testing.T) {
			setHelperEnv(t, tc.helperCmd)
			openGate := newGate(t)
			sink := &recordingSink{}
			c := &lineCollector{onLine: func(line string) {
				if line == tc.firstLine {
					openGate()
				}
			}}
			req := tc.request()
			req.OnStdoutLine = c.observe

			resp, err := tc.newSpawner(t, sink).Spawn(context.Background(), req)

			if err != nil {
				t.Fatalf("want the stream to complete after the first line was observed, got %v", err)
			}
			lines := c.lines()
			if len(lines) < 2 || lines[0] != tc.firstLine {
				t.Errorf("want the observer to receive the stream line by line starting with %q, got %q", tc.firstLine, lines)
			}
			if len(resp.Protocol) == 0 || !strings.Contains(string(resp.Protocol), "test-agent#1") {
				t.Errorf("want the protocol extracted as usual, got %q", resp.Protocol)
			}
			ev, ok := findEvent(sink.snapshot(), "spawn.stdout")
			if !ok || ev.Message != string(resp.Stdout) || !strings.HasPrefix(ev.Message, tc.firstLine) {
				t.Errorf("want spawn.stdout to carry the full stdout, got %q (found=%v)", ev.Message, ok)
			}
		})
	}
}

func TestSpawners_WithoutOnStdoutLine_BehavesAsBefore(t *testing.T) {
	for _, tc := range spawnerCases() {
		t.Run(tc.name, func(t *testing.T) {
			setHelperEnv(t, tc.helperCmd) // no gate configured: the helper does not wait
			sink := &recordingSink{}

			resp, err := tc.newSpawner(t, sink).Spawn(context.Background(), tc.request())

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.ExitCode != 0 || !strings.Contains(string(resp.Protocol), "test-agent#1") {
				t.Errorf("want a normal successful response, got %+v", resp)
			}
			ev, ok := findEvent(sink.snapshot(), "spawn.stdout")
			if !ok || ev.Message != string(resp.Stdout) {
				t.Errorf("want spawn.stdout to carry the full stdout, got %q (found=%v)", ev.Message, ok)
			}
			if _, ok := findEvent(sink.snapshot(), "spawn.stderr"); !ok {
				t.Errorf("want a spawn.stderr event")
			}
		})
	}
}
