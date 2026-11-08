package cli_test

// Tests for the --hitl-timeout flag: the invocation timeout of dispatches sent
// with human_in_the_loop: true. --timeout keeps its meaning (non-HITL value).

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

func registeredRunFlag(t *testing.T, name string) *pflag.Flag {
	t.Helper()
	fs := pflag.NewFlagSet("hitl-timeout-test", pflag.ContinueOnError)
	cli.RegisterRunFlags(fs)
	f := fs.Lookup(name)
	if f == nil {
		t.Fatalf("run flag --%s is not registered", name)
	}
	return f
}

func TestHITLTimeoutFlag_DefaultsToFourHours(t *testing.T) {
	f := registeredRunFlag(t, "hitl-timeout")

	if f.DefValue != "4h" {
		t.Errorf("--hitl-timeout default = %q, want %q", f.DefValue, "4h")
	}
	if got := domain.DefaultHITLTimeout.String(); got != "4h0m0s" {
		t.Errorf("DefaultHITLTimeout = %s, want 4h0m0s", got)
	}
}

func TestTimeoutFlag_KeepsThirtyMinuteDefault(t *testing.T) {
	f := registeredRunFlag(t, "timeout")

	if f.DefValue != "30m" {
		t.Errorf("--timeout default = %q, want %q", f.DefValue, "30m")
	}
}

func TestHITLTimeoutFlag_IsDeclaredValueBearing(t *testing.T) {
	var spec *cli.FlagSpec
	for _, s := range cli.RunFlagSpecs() {
		if s.Name == "--hitl-timeout" {
			s := s
			spec = &s
		}
	}
	if spec == nil {
		t.Fatal("RunFlagSpecs() has no entry for --hitl-timeout")
	}
	if !spec.TakesValue {
		t.Error("--hitl-timeout must be value-bearing so pre-scans skip its value token")
	}
	found := false
	for _, n := range cli.ValueBearingFlagNames() {
		if n == "--hitl-timeout" {
			found = true
		}
	}
	if !found {
		t.Error("ValueBearingFlagNames() does not contain --hitl-timeout")
	}
}

func TestHITLTimeoutFlag_ValidDurationAccepted(t *testing.T) {
	sess := &scriptedSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
	args := append(baseHarnessArgs(), "--hitl-timeout", "2h")

	code, _, errOut := runCLIWithStore(t, args, &spyStore{}, sess)

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d); stderr: %q", code, cli.ExitSuccess, errOut)
	}
	if !sess.called {
		t.Error("session.Start was not called for valid --hitl-timeout value")
	}
}

func TestHITLTimeoutFlag_InvalidDurationRejectsWithUsageError(t *testing.T) {
	sess := &scriptedSession{}
	args := append(baseHarnessArgs(), "--hitl-timeout", "not-a-duration")

	code, _, errOut := runCLI(t, args, sess)

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage (%d) for invalid --hitl-timeout value", code, cli.ExitUsage)
	}
	if !strings.Contains(errOut, "not-a-duration") || !strings.Contains(errOut, "hitl-timeout") {
		t.Errorf("stderr %q should name the flag and the invalid value", errOut)
	}
	if sess.called {
		t.Error("session.Start should not be called for invalid --hitl-timeout value")
	}
}

func TestHITLTimeoutFlag_ValidatedSameAsTimeout_NonPositiveHandledLikeTimeout(t *testing.T) {
	for _, value := range []string{"0s", "-5m"} {
		t.Run(value, func(t *testing.T) {
			timeoutCode, _, _ := runCLI(t, append(baseHarnessArgs(), "--timeout", value), &scriptedSession{})
			hitlCode, _, _ := runCLI(t, append(baseHarnessArgs(), "--hitl-timeout", value), &scriptedSession{})

			if hitlCode != timeoutCode {
				t.Errorf("--hitl-timeout %s exit code = %d, but --timeout %s gives %d; the rule must be the same", value, hitlCode, value, timeoutCode)
			}
		})
	}
}

func TestTimeoutFlag_InvalidValueStillRejectedWhenHITLTimeoutValid(t *testing.T) {
	sess := &scriptedSession{}
	args := append(baseHarnessArgs(), "--timeout", "bogus", "--hitl-timeout", "4h")

	code, _, _ := runCLI(t, args, sess)

	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
	if sess.called {
		t.Error("session.Start should not be called when --timeout is invalid")
	}
}
