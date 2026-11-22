package codex_test

// Tests that every Codex failure shape ends the invocation with an error of an
// existing sentinel family, on both Invoke and InvokeRaw.

import (
	"context"
	"errors"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/codex"
)

// invokeBoth runs the scenario through Invoke and InvokeRaw and returns both
// errors.
func invokeBoth(t *testing.T, scenario string) (invokeErr, rawErr error) {
	t.Helper()
	setHelperEnv(t, scenario)
	adapter := newAdapter(t)
	_, invokeErr = adapter.Invoke(context.Background(), agentRef(t), request("test-agent#1"))
	_, rawErr = adapter.InvokeRaw(context.Background(), agentRef(t), rawPayload)
	return invokeErr, rawErr
}

func requireIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("want an error wrapping %v, got %v", target, err)
	}
}

func TestCodexAdapter_NonZeroExit_ReturnsErrNonZeroExit(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-exit1")

	for _, err := range []error{invokeErr, rawErr} {
		requireIs(t, err, commonharness.ErrNonZeroExit)
		if errors.Is(err, codex.ErrStreamError) {
			t.Errorf("a plain non-zero exit must not claim a stream error: %v", err)
		}
	}
}

func TestCodexAdapter_NonZeroExitWithTurnFailed_WrapsBothSentinels(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-exit1-failed")

	for _, err := range []error{invokeErr, rawErr} {
		requireIs(t, err, commonharness.ErrNonZeroExit)
		requireIs(t, err, codex.ErrStreamError)
		if err != nil && !strings.Contains(err.Error(), "quota exceeded") {
			t.Errorf("want the turn.failed message in the error, got %v", err)
		}
	}
}

func TestCodexAdapter_TurnFailed_ReturnsStreamErrorWithMessage(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-turn-failed")

	for _, err := range []error{invokeErr, rawErr} {
		requireIs(t, err, codex.ErrStreamError)
		if err != nil && !strings.Contains(err.Error(), "quota exceeded") {
			t.Errorf("want the failure message in the error, got %v", err)
		}
	}
}

func TestCodexAdapter_TopLevelErrorEvent_ReturnsStreamError(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-error-event")

	requireIs(t, invokeErr, codex.ErrStreamError)
	requireIs(t, rawErr, codex.ErrStreamError)
}

func TestCodexAdapter_IncompleteStream_ReturnsStreamIncomplete(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-incomplete")

	requireIs(t, invokeErr, codex.ErrStreamIncomplete)
	requireIs(t, rawErr, codex.ErrStreamIncomplete)
}

func TestCodexAdapter_EmptyStream_ReturnsErrEmptyResponse(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-empty")

	requireIs(t, invokeErr, commonharness.ErrEmptyResponse)
	requireIs(t, rawErr, commonharness.ErrEmptyResponse)
}

func TestCodexAdapter_CompletedTurnWithoutMessage_ReturnsErrEmptyResponse(t *testing.T) {
	invokeErr, rawErr := invokeBoth(t, "codex-no-message")

	requireIs(t, invokeErr, commonharness.ErrEmptyResponse)
	requireIs(t, rawErr, commonharness.ErrEmptyResponse)
}

func TestCodexAdapter_Invoke_UnparseableFinalMessage_ReturnsUnparseableSentinel(t *testing.T) {
	setHelperEnv(t, "codex-unparseable")

	_, err := newAdapter(t).Invoke(context.Background(), agentRef(t), request("test-agent#1"))

	if !errors.Is(err, commonharness.ErrMalformedJSON) && !errors.Is(err, commonharness.ErrProtocolNotExtractable) {
		t.Errorf("want ErrMalformedJSON or ErrProtocolNotExtractable, got %v", err)
	}
}

func TestCodexAdapter_MissingExecutable_ReturnsLaunchError(t *testing.T) {
	setHelperEnv(t, "codex-success")
	missing := "/nonexistent/codex-binary"
	adapter := codex.NewCodexAdapter(missing, adapterTimeout)

	_, invokeErr := adapter.Invoke(context.Background(), agentRef(t), request("test-agent#1"))
	_, rawErr := adapter.InvokeRaw(context.Background(), agentRef(t), rawPayload)

	for _, err := range []error{invokeErr, rawErr} {
		var launch *domain.HarnessLaunchError
		if !errors.As(err, &launch) {
			t.Fatalf("want *domain.HarnessLaunchError, got %T: %v", err, err)
		}
		if launch.Harness != "codex" || launch.Executable != missing {
			t.Errorf("launch error = {Harness:%q Executable:%q}, want codex and %q", launch.Harness, launch.Executable, missing)
		}
		requireIs(t, err, commonharness.ErrExecutableNotFound)
	}
}

func TestCodexAdapter_UndecodableAgentFile_ReturnsAgentUnreadable(t *testing.T) {
	setHelperEnv(t, "codex-success")
	ref := agentWithContent(t, "name = \nthis is = not [valid toml\n")
	adapter := newAdapter(t)

	_, invokeErr := adapter.Invoke(context.Background(), ref, request("test-agent#1"))
	_, rawErr := adapter.InvokeRaw(context.Background(), ref, rawPayload)

	requireIs(t, invokeErr, codex.ErrAgentUnreadable)
	requireIs(t, rawErr, codex.ErrAgentUnreadable)
}

func TestCodexAdapter_MissingAgentFile_ReturnsAgentUnreadable(t *testing.T) {
	setHelperEnv(t, "codex-success")
	ref := domain.AgentReference{Identifier: "ghost", DefinitionPath: "/nonexistent/ghost.toml"}

	_, err := newAdapter(t).Invoke(context.Background(), ref, request("ghost#1"))

	requireIs(t, err, codex.ErrAgentUnreadable)
}

// Support coverage (not RED evidence): passes against a stub that never
// touches the file; the success-path byte-identical tests carry AC evidence.
func TestCodexAdapter_FailedInvocation_LeavesTheAgentFileByteIdentical(t *testing.T) {
	for _, scenario := range []string{"codex-turn-failed", "codex-exit1", "codex-incomplete"} {
		t.Run(scenario, func(t *testing.T) {
			setHelperEnv(t, scenario)
			ref, original := agentCopy(t)

			_, _ = newAdapter(t).Invoke(context.Background(), ref, request("test-agent#1"))

			assertFileUnchanged(t, ref.DefinitionPath, original)
		})
	}
}
