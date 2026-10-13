package ghcpcli_test

// Tests for the GHCP stream events that fail an invocation on both adapter
// paths (Invoke and InvokeRaw), and for the GHCP version in the debug log.
//
// Coverage:
//
//   - A stream with subagent.deselected fails with the deselection error even
//     when it ends in a valid protocol reply and a successful result; the
//     error text names the requested agent and errors.Is identifies the
//     deselection sentinel.
//   - A session.error stream fails with the event's type/code and message in
//     the returned error and in the harness.invoke.error log entry; a
//     session.error followed by result exitCode 0 also fails.
//   - The new errors are not raw-text harness errors (so they are not
//     bypassed by the session's raw-text handling).
//   - Invoke logs the stdout under harness.parse.failed for both new errors.
//   - The GHCP version found in stdout is logged as the ghcp_version field of
//     harness.invoke.ok and harness.invoke.error, on both paths; a stream with
//     no version path logs no such field.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/ghcpcli"
)

// invokeVia runs one invocation on the named path ("invoke" or "raw") against
// the helper scenario and returns the error and the recording logger.
func invokeVia(t *testing.T, path, scenario string) (error, *recordingLogger) {
	t.Helper()
	setHelperEnv(t, scenario)
	logger := &recordingLogger{}
	adapter := ghcpcli.NewGHCPCLIAdapterWithLogger(helperExe(t), 5*time.Second, logger)
	var err error
	if path == "invoke" {
		_, err = adapter.Invoke(context.Background(), ordinaryAgentRef(), minimalClaudeRequest("test-agent#1"))
	} else {
		_, err = adapter.InvokeRaw(context.Background(), orchestratorRef(), rawPayload)
	}
	return err, logger
}

var invokePaths = []string{"invoke", "raw"}

// requestedAgent is the identifier of the agent reference each path requests.
func requestedAgent(path string) string {
	if path == "invoke" {
		return ordinaryAgentRef().Identifier
	}
	return orchestratorRef().Identifier
}

func assertNotRawTextError(t *testing.T, err error) {
	t.Helper()
	for _, s := range []error{commonharness.ErrProtocolNotExtractable, commonharness.ErrMalformedJSON, commonharness.ErrEmptyResponse} {
		if errors.Is(err, s) {
			t.Errorf("error must not be a raw-text error (%v), got %v", s, err)
		}
	}
}

func TestGHCPCLIAdapter_DeselectedStreamWithValidProtocolReply_FailsNamingTheAgent(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			err, _ := invokeVia(t, path, "ghcpcli-deselected")

			if !errors.Is(err, commonharness.ErrGHCPCLIAgentDeselected) {
				t.Fatalf("want ErrGHCPCLIAgentDeselected, got %v", err)
			}
			if !strings.Contains(err.Error(), requestedAgent(path)) {
				t.Errorf("want error text to name the agent %q, got %q", requestedAgent(path), err.Error())
			}
			assertNotRawTextError(t, err)
		})
	}
}

func TestGHCPCLIAdapter_DeselectedStream_ErrorReachesInvokeErrorLogWithAgentName(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			_, logger := invokeVia(t, path, "ghcpcli-deselected")

			msg, ok := logger.messageFor(domain.EventHarnessInvokeError)
			if !ok {
				t.Fatalf("want a %s entry, got events %v", domain.EventHarnessInvokeError, logger.allEvents())
			}
			if !strings.Contains(msg, requestedAgent(path)) {
				t.Errorf("want the logged error to name the agent %q, got %q", requestedAgent(path), msg)
			}
			if logger.eventLogged(domain.EventHarnessInvokeOK) {
				t.Errorf("want no %s entry for a deselected stream", domain.EventHarnessInvokeOK)
			}
		})
	}
}

func TestGHCPCLIAdapter_QuotaStream_ErrorCarriesCodeAndMessageIntoReturnAndLog(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			err, logger := invokeVia(t, path, "ghcpcli-quota")

			if !errors.Is(err, commonharness.ErrGHCPCLISessionError) {
				t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
			}
			logged, _ := logger.messageFor(domain.EventHarnessInvokeError)
			for _, want := range []string{"quota_exceeded", "You have exceeded your monthly quota"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("want returned error to contain %q, got %q", want, err.Error())
				}
				if !strings.Contains(logged, want) {
					t.Errorf("want %s entry to contain %q, got %q", domain.EventHarnessInvokeError, want, logged)
				}
			}
			assertNotRawTextError(t, err)
			if errors.Is(err, commonharness.ErrGHCPCLIStreamError) {
				t.Errorf("the session error must not be reported as the plain stream error, got %v", err)
			}
		})
	}
}

func TestGHCPCLIAdapter_SessionErrorFollowedBySuccessfulResult_Fails(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			err, _ := invokeVia(t, path, "ghcpcli-session-error-exit0")

			if !errors.Is(err, commonharness.ErrGHCPCLISessionError) {
				t.Fatalf("want ErrGHCPCLISessionError, got %v", err)
			}
		})
	}
}

func TestGHCPCLIAdapter_Invoke_NewStreamErrors_LogStdoutAsParseFailed(t *testing.T) {
	for _, scenario := range []string{"ghcpcli-deselected", "ghcpcli-quota"} {
		t.Run(scenario, func(t *testing.T) {
			_, logger := invokeVia(t, "invoke", scenario)

			if !logger.eventLogged(domain.EventHarnessParseFailed) {
				t.Errorf("want %s logged, got events %v", domain.EventHarnessParseFailed, logger.allEvents())
			}
		})
	}
}

func TestGHCPCLIAdapter_SuccessfulInvocation_LogsRunningVersionWithInvocation(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			err, logger := invokeVia(t, path, "ghcpcli-versioned")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := logger.fieldValue(domain.EventHarnessInvokeOK, "ghcp_version")
			if !ok || got != "1.0.92" {
				t.Errorf("want %s ghcp_version=1.0.92, got %q (present=%v)", domain.EventHarnessInvokeOK, got, ok)
			}
		})
	}
}

func TestGHCPCLIAdapter_FailedInvocation_LogsVersionOnInvokeError(t *testing.T) {
	cases := map[string]string{"ghcpcli-quota": "1.0.92", "ghcpcli-deselected": "1.0.91"}
	for scenario, wantVersion := range cases {
		for _, path := range invokePaths {
			t.Run(scenario+"/"+path, func(t *testing.T) {
				_, logger := invokeVia(t, path, scenario)

				got, ok := logger.fieldValue(domain.EventHarnessInvokeError, "ghcp_version")
				if !ok || got != wantVersion {
					t.Errorf("want %s ghcp_version=%s, got %q (present=%v)", domain.EventHarnessInvokeError, wantVersion, got, ok)
				}
			})
		}
	}
}

func TestGHCPCLIAdapter_StreamWithoutVersionPath_LogsNoVersionAndIsNotAnError(t *testing.T) {
	for _, path := range invokePaths {
		t.Run(path, func(t *testing.T) {
			err, logger := invokeVia(t, path, "ghcpcli-success")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !logger.eventLogged(domain.EventHarnessInvokeOK) {
				t.Fatalf("want %s logged, got events %v", domain.EventHarnessInvokeOK, logger.allEvents())
			}
			if v, ok := logger.fieldValue(domain.EventHarnessInvokeOK, "ghcp_version"); ok {
				t.Errorf("want no ghcp_version field, got %q", v)
			}
		})
	}
}
