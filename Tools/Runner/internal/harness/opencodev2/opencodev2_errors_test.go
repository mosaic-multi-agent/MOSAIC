package opencodev2_test

// Tests for how failures leave the adapter: each V2 failure cause arrives as
// its own named error, launch failures are wrapped as harness launch errors,
// cancellation comes back as the context error, and the debug log vocabulary
// matches the other adapters.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/opencodev2"
)

var v2Sentinels = []error{
	commonharness.ErrOpenCodeV2UnknownAgent,
	commonharness.ErrOpenCodeV2UnknownModel,
	commonharness.ErrOpenCodeV2UsageError,
	commonharness.ErrOpenCodeV2RunFailed,
	commonharness.ErrOpenCodeV2Interrupted,
	commonharness.ErrOpenCodeV2Terminated,
	commonharness.ErrOpenCodeV2StreamIncomplete,
}

type failureCase struct {
	scenario string
	want     error
	detail   string // fragment the error text must carry; "" when none
}

func failureCases() []failureCase {
	return []failureCase{
		{"error-agent", commonharness.ErrOpenCodeV2UnknownAgent, "no-such-agent"},
		{"error-model", commonharness.ErrOpenCodeV2UnknownModel, "no-such-model"},
		{"error-other", commonharness.ErrOpenCodeV2RunFailed, "upstream exploded"},
		{"exit1-stream", commonharness.ErrOpenCodeV2RunFailed, ""},
		{"usage", commonharness.ErrOpenCodeV2UsageError, "bogus-flag"},
		{"exit130", commonharness.ErrOpenCodeV2Interrupted, ""},
		{"exit143", commonharness.ErrOpenCodeV2Terminated, ""},
		{"incomplete", commonharness.ErrOpenCodeV2StreamIncomplete, ""},
	}
}

func assertOnlySentinel(t *testing.T, err, want error, detail string) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
	for _, other := range v2Sentinels {
		if other != want && errors.Is(err, other) {
			t.Errorf("error also matches %v, causes must stay distinct: %v", other, err)
		}
	}
	if detail != "" && !strings.Contains(err.Error(), detail) {
		t.Errorf("want the error to carry %q, got %v", detail, err)
	}
}

func TestInvoke_EachFailureCauseArrivesAsItsOwnNamedError(t *testing.T) {
	for _, tc := range failureCases() {
		t.Run(tc.scenario, func(t *testing.T) {
			setHelperEnv(t, tc.scenario)

			_, err := newAdapter(t, 10*time.Second).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

			assertOnlySentinel(t, err, tc.want, tc.detail)
		})
	}
}

func TestInvokeRaw_EachFailureCauseArrivesAsItsOwnNamedError(t *testing.T) {
	for _, tc := range failureCases() {
		t.Run(tc.scenario, func(t *testing.T) {
			setHelperEnv(t, tc.scenario)

			_, err := newAdapter(t, 10*time.Second).InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

			assertOnlySentinel(t, err, tc.want, tc.detail)
		})
	}
}

func TestInvokeAndInvokeRaw_ExitZeroWithoutOutput_AreEmptyResponses(t *testing.T) {
	setHelperEnv(t, "empty")
	adapter := newAdapter(t, 10*time.Second)

	_, invokeErr := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))
	_, rawErr := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	if !errors.Is(invokeErr, commonharness.ErrEmptyResponse) || !errors.Is(rawErr, commonharness.ErrEmptyResponse) {
		t.Errorf("want ErrEmptyResponse from both, got Invoke=%v InvokeRaw=%v", invokeErr, rawErr)
	}
}

func TestInvoke_MissingExecutable_IsAHarnessLaunchErrorNamingTheHarness(t *testing.T) {
	const missing = "/nonexistent/path/to/opencode"
	adapter := opencodev2.NewOpenCodeV2Adapter(missing, 5*time.Second)

	_, invokeErr := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))
	_, rawErr := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	for name, err := range map[string]error{"Invoke": invokeErr, "InvokeRaw": rawErr} {
		var launchErr *domain.HarnessLaunchError
		if !errors.As(err, &launchErr) {
			t.Errorf("%s: want *domain.HarnessLaunchError, got %v", name, err)
			continue
		}
		if launchErr.Harness != "opencode-v2" || launchErr.Executable != missing {
			t.Errorf("%s: want harness opencode-v2 and executable %q, got %+v", name, missing, launchErr)
		}
		if !errors.Is(err, commonharness.ErrExecutableNotFound) {
			t.Errorf("%s: want ErrExecutableNotFound kept in the chain, got %v", name, err)
		}
	}
}

func TestInvoke_Timeout_IsErrTimeoutNotAV2Sentinel(t *testing.T) {
	setHelperEnv(t, "hang")

	_, err := newAdapter(t, shortTimeout).Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if !errors.Is(err, commonharness.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
	for _, s := range v2Sentinels {
		if errors.Is(err, s) {
			t.Errorf("a timeout must not read as %v", s)
		}
	}
}

func TestInvokeAndInvokeRaw_ContextCancellation_ReturnCtxErr(t *testing.T) {
	setHelperEnv(t, "hang")
	adapter := newAdapter(t, longTimeout)
	calls := map[string]func(ctx context.Context) error{
		"Invoke": func(ctx context.Context) error {
			_, err := adapter.Invoke(ctx, ordinaryAgentRef(), minimalRequest("test-agent#1"))
			return err
		},
		"InvokeRaw": func(ctx context.Context) error {
			_, err := adapter.InvokeRaw(ctx, orchestratorRef(), rawPayload)
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}()

			err := call(ctx)

			if !errors.Is(err, context.Canceled) {
				t.Errorf("want context.Canceled, got %v", err)
			}
		})
	}
}

func TestInvoke_SuccessLogsStartAndOKAndNoError(t *testing.T) {
	setHelperEnv(t, "success")
	logger := &recordingLogger{}
	adapter := opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, logger)

	_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger.count(domain.EventHarnessInvokeStart) != 1 || logger.count(domain.EventHarnessInvokeOK) != 1 {
		t.Errorf("want one start and one ok event, got %+v", logger.entries)
	}
	if logger.count(domain.EventHarnessInvokeError) != 0 || logger.count(domain.EventHarnessParseFailed) != 0 {
		t.Errorf("want no error or parse-failed event, got %+v", logger.entries)
	}
}

func TestInvoke_V2ReportedFailure_LogsOnlyAnInvokeError(t *testing.T) {
	for _, scenario := range []string{"error-agent", "error-model", "error-other", "usage", "exit130"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			logger := &recordingLogger{}
			adapter := opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, logger)

			_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

			if err == nil {
				t.Fatal("want a failure")
			}
			if logger.count(domain.EventHarnessInvokeError) != 1 || logger.count(domain.EventHarnessParseFailed) != 0 {
				t.Errorf("want one invoke-error and no parse-failed event, got %+v", logger.entries)
			}
		})
	}
}

func TestInvoke_UnusableReplies_LogParseFailedBeforeTheInvokeError(t *testing.T) {
	for _, scenario := range []string{"incomplete", "empty", "not-protocol"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			logger := &recordingLogger{}
			adapter := opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, logger)

			_, err := adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalRequest("test-agent#1"))

			if err == nil {
				t.Fatal("want a failure")
			}
			if logger.count(domain.EventHarnessParseFailed) == 0 || logger.count(domain.EventHarnessInvokeError) != 1 {
				t.Fatalf("want a parse-failed and one invoke-error event, got %+v", logger.entries)
			}
			names := make([]string, len(logger.entries))
			for i, e := range logger.entries {
				names[i] = e.Event
			}
			if idx(names, domain.EventHarnessParseFailed) > idx(names, domain.EventHarnessInvokeError) {
				t.Errorf("parse-failed must be logged before the invoke error, got %v", names)
			}
		})
	}
}

func TestInvokeRaw_SuccessLogsStartAndOK(t *testing.T) {
	setHelperEnv(t, "raw-text")
	logger := &recordingLogger{}
	adapter := opencodev2.NewOpenCodeV2AdapterWithLogger(helperExe(t), 10*time.Second, logger)

	_, err := adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger.count(domain.EventHarnessInvokeStart) != 1 || logger.count(domain.EventHarnessInvokeOK) != 1 {
		t.Errorf("want one start and one ok event, got %+v", logger.entries)
	}
}

func idx(names []string, want string) int {
	for i, n := range names {
		if n == want {
			return i
		}
	}
	return len(names) + 1
}
