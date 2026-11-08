package claudecode_test

// Tests that the activity observer cannot change or stall an ClaudeCodeAdapter
// invocation: a misbehaving feed leaves the outcome unchanged, debug-log
// events are the same with and without an observer, and every outcome of
// Invoke and InvokeRaw ends with exactly one Ended.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/claudecode"
	"mosaic-run/internal/testutil"
)

func invokeOutcome(t *testing.T, a *claudecode.ClaudeCodeAdapter) string {
	t.Helper()
	resp, err := a.Invoke(context.Background(), ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
	return fmt.Sprintf("%+v|%v", resp, err)
}

func invokeRawOutcome(t *testing.T, a *claudecode.ClaudeCodeAdapter) string {
	t.Helper()
	out, err := a.InvokeRaw(context.Background(), orchestratorAgentRefCC(t), []byte("consult"))
	return fmt.Sprintf("%q|%v", out, err)
}

func TestClaudeCodeAdapter_Invoke_FeedFailuresDoNotChangeOutcome(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)

	testutil.AssertFeedFailuresHarmless(t, func(obs domain.ActivityObserver) string {
		return invokeOutcome(t, newActivityAdapter(t, obs))
	})
}

func TestClaudeCodeAdapter_InvokeRaw_FeedFailuresDoNotChangeOutcome(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)

	testutil.AssertFeedFailuresHarmless(t, func(obs domain.ActivityObserver) string {
		return invokeRawOutcome(t, newActivityAdapter(t, obs))
	})
}

func TestClaudeCodeAdapter_Invoke_DebugLogEventsIdenticalWithAndWithoutObserver(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)

	testutil.AssertLogsIdentical(t, func(lg domain.DebugLogger, obs domain.ActivityObserver, with bool) {
		a := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 30*time.Second, lg)
		if with {
			a.WithActivityObserver(obs)
		}
		invokeOutcome(t, a)
	})
}

func TestClaudeCodeAdapter_InvokeRaw_DebugLogEventsIdenticalWithAndWithoutObserver(t *testing.T) {
	setHelperEnv(t, "activity-paced")
	testutil.PresetOpenGate(t)

	testutil.AssertLogsIdentical(t, func(lg domain.DebugLogger, obs domain.ActivityObserver, with bool) {
		a := claudecode.NewClaudeCodeAdapterWithLogger(helperExe(t), 30*time.Second, lg)
		if with {
			a.WithActivityObserver(obs)
		}
		invokeRawOutcome(t, a)
	})
}

// outcomeCase is one way an invocation can end unsuccessfully.
type outcomeCase struct {
	scenario   string
	missingExe bool
	gateClosed bool // the process stays alive after its first lines
	timeout    time.Duration
	cancel     bool // cancel the context when the first tool call arrives
}

func TestClaudeCodeAdapter_EveryOutcomeAnnouncesStartAndEnd(t *testing.T) {
	cases := map[string]outcomeCase{
		"non-zero exit":        {scenario: "exit1", timeout: 30 * time.Second},
		"timeout":              {scenario: "activity-paced", gateClosed: true, timeout: 500 * time.Millisecond},
		"cancellation":         {scenario: "activity-paced", gateClosed: true, timeout: 30 * time.Second, cancel: true},
		"executable not found": {scenario: "activity-paced", missingExe: true, timeout: 5 * time.Second},
	}
	entries := map[string]func(*testing.T, context.Context, *claudecode.ClaudeCodeAdapter) error{
		"Invoke": func(t *testing.T, ctx context.Context, a *claudecode.ClaudeCodeAdapter) error {
			_, err := a.Invoke(ctx, ordinaryAgentRefCC(t), minimalClaudeRequest("test-agent#1"))
			return err
		},
		"InvokeRaw": func(t *testing.T, ctx context.Context, a *claudecode.ClaudeCodeAdapter) error {
			_, err := a.InvokeRaw(ctx, orchestratorAgentRefCC(t), []byte("consult"))
			return err
		},
	}
	for entryName, entry := range entries {
		for caseName, c := range cases {
			t.Run(entryName+"/"+caseName, func(t *testing.T) {
				setHelperEnv(t, c.scenario)
				if c.gateClosed {
					testutil.UnopenedGate(t)
				} else {
					testutil.PresetOpenGate(t)
				}
				obs := &testutil.Observer{}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				if c.cancel {
					obs.OnItem = func(it domain.ActivityItem) {
						if it.Kind == domain.ActivityToolCall {
							cancel()
						}
					}
				}
				exe := helperExe(t)
				if c.missingExe {
					exe = "/nonexistent/binary"
				}
				a := claudecode.NewClaudeCodeAdapter(exe, c.timeout).WithActivityObserver(obs)

				err := entry(t, ctx, a)

				if err == nil {
					t.Fatal("want this outcome to be an error")
				}
				_, _, events, ends := obs.Snapshot()
				testutil.AssertLifecycle(t, events, ends)
			})
		}
	}
}
