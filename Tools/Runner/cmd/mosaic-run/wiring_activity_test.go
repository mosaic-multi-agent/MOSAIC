package main

// The TUI path hands the TUI's activity observer to the adapters it builds;
// the CLI path keeps no observer. Neither may break an invocation, also while
// the TUI program is not (yet) running.

import (
	"context"
	"os"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
	"mosaic-run/internal/tui"
	"mosaic-run/internal/tui/screens/runconfig"
)

// The interactive wiring can only pass the TUI's ProgramRef as the activity
// observer if it implements the port.
var _ domain.ActivityObserver = (*tui.ProgramRef)(nil)

// captureAdapterObserver replaces the adapter construction seam for the test and
// returns a pointer to the activity observer of the most recent construction,
// plus a counter of constructions. The real constructor still runs.
func captureAdapterObserver(t *testing.T) (*domain.ActivityObserver, *int) {
	t.Helper()
	var got domain.ActivityObserver
	calls := 0
	orig := buildAdapterFn
	buildAdapterFn = func(runFolder, harnessStr, execPathStr, ghcpMode string, timeouts domain.InvocationTimeouts, activity domain.ActivityObserver, loggers ...domain.DebugLogger) domain.HarnessAdapter {
		got = activity
		calls++
		return orig(runFolder, harnessStr, execPathStr, ghcpMode, timeouts, activity, loggers...)
	}
	t.Cleanup(func() { buildAdapterFn = orig })
	return &got, &calls
}

func TestInteractiveWiring_ActivityObserver_IsTheWiringsOwnProgramRef(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			got, calls := captureAdapterObserver(t)
			in := testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t))
			w := buildInteractiveWiring(in)
			cfg := runconfig.ConfigSelection{Harness: id, ExecutablePath: os.Args[0]}

			w.NewDeps(testRunFolder(t, "20260101T000000Z-0010"), true, "", cfg)

			if *calls != 1 {
				t.Fatalf("adapter constructed %d times, want 1", *calls)
			}
			if *got == nil {
				t.Fatalf("TUI wiring handed no activity observer to adapter construction")
			}
			if *got != domain.ActivityObserver(in.ProgramRef) {
				t.Fatalf("activity observer = %T (%v), want the wiring's own ProgramRef %p", *got, *got, in.ProgramRef)
			}
		})
	}
}

func TestInteractiveWiring_ActivityObserver_EveryRebuildGetsTheSameProgramRef(t *testing.T) {
	got, calls := captureAdapterObserver(t)
	in := testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t))
	w := buildInteractiveWiring(in)
	cfg := runconfig.ConfigSelection{Harness: commonharness.HarnessIDClaudeCode, ExecutablePath: os.Args[0]}

	for i, folder := range []string{"20260101T000000Z-0011", "20260101T000000Z-0012"} {
		w.NewDeps(testRunFolder(t, folder), true, "", cfg)

		if *calls != i+1 {
			t.Fatalf("after rebuild %d: %d constructions", i+1, *calls)
		}
		if *got != domain.ActivityObserver(in.ProgramRef) {
			t.Fatalf("rebuild %d: activity observer = %v, want the wiring's own ProgramRef", i+1, *got)
		}
	}
}

func TestInteractiveWiring_ActivityObserver_InvocationsFinishWhileTheProgramIsNotRunning(t *testing.T) {
	for _, id := range cliHarnessIDs {
		t.Run(id, func(t *testing.T) {
			useStreamTranscript(t, id)
			w := buildInteractiveWiring(testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t)))
			cfg := runconfig.ConfigSelection{
				Harness: id, ExecutablePath: os.Args[0], Timeout: guardTimeout, HITLTimeout: guardTimeout,
			}
			deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0008"), true, "", cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			_, err := deps.Harness.Invoke(ctx, timeoutAgentRef(t, domain.InvocationOrdinary),
				domain.ProtocolRequest{AgentInstanceID: "test-agent#1", TaskDescription: "x"})

			if ctx.Err() != nil {
				t.Fatalf("invocation did not finish through the TUI wiring: %v", err)
			}
		})
	}
}

func TestInteractiveWiring_Consultation_DoesNotBreakWithTheTUIObserver(t *testing.T) {
	useStreamTranscript(t, commonharness.HarnessIDClaudeCode)
	w := buildInteractiveWiring(testWiringInput(t, session.NewStopSignal(), resolvedTestIdentity(t)))
	cfg := runconfig.ConfigSelection{
		Harness: commonharness.HarnessIDClaudeCode, ExecutablePath: os.Args[0],
		Timeout: guardTimeout, HITLTimeout: guardTimeout,
	}
	deps := w.NewDeps(testRunFolder(t, "20260101T000000Z-0009"), true, "", cfg)
	raw, ok := deps.Harness.(domain.RawInvoker)
	if !ok {
		t.Fatalf("%T does not implement domain.RawInvoker", deps.Harness)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := raw.InvokeRaw(ctx, timeoutAgentRef(t, domain.InvocationOrchestrator), []byte(`{}`))

	if ctx.Err() != nil {
		t.Fatalf("consultation did not finish through the TUI wiring: %v", err)
	}
}
