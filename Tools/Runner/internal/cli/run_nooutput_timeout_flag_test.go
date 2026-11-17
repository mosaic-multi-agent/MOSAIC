package cli_test

// Tests for the --no-output-timeout flag: the no-output watchdog duration of
// opencode-v2 invocations without human_in_the_loop. Default 10m; 0 disables;
// unparseable or negative values are usage errors.

import (
	"strings"
	"testing"

	"mosaic-run/internal/cli"
	"mosaic-run/internal/domain"
)

func TestNoOutputTimeoutFlag_DefaultsToTenMinutes(t *testing.T) {
	f := registeredRunFlag(t, "no-output-timeout")

	if f.DefValue != "10m" {
		t.Errorf("--no-output-timeout default = %q, want %q", f.DefValue, "10m")
	}
	if got := domain.DefaultNoOutputTimeout.String(); got != "10m0s" {
		t.Errorf("DefaultNoOutputTimeout = %s, want 10m0s", got)
	}
}

func TestNoOutputTimeoutFlag_HelpStatesScopeDefaultAndDisabling(t *testing.T) {
	usage := registeredRunFlag(t, "no-output-timeout").Usage

	for _, want := range []string{"opencode-v2", "human_in_the_loop", "default 10m", "0 disables"} {
		if !strings.Contains(usage, want) {
			t.Errorf("--no-output-timeout help %q does not mention %q", usage, want)
		}
	}
}

func TestNoOutputTimeoutFlag_IsDeclaredValueBearing(t *testing.T) {
	var spec *cli.FlagSpec
	for _, s := range cli.RunFlagSpecs() {
		if s.Name == "--no-output-timeout" {
			s := s
			spec = &s
		}
	}
	if spec == nil {
		t.Fatal("RunFlagSpecs() has no entry for --no-output-timeout")
	}
	if !spec.TakesValue {
		t.Error("--no-output-timeout must be value-bearing so pre-scans skip its value token")
	}
	found := false
	for _, n := range cli.ValueBearingFlagNames() {
		if n == "--no-output-timeout" {
			found = true
		}
	}
	if !found {
		t.Error("ValueBearingFlagNames() does not contain --no-output-timeout")
	}
}

func TestNoOutputTimeoutFlag_AcceptedValues(t *testing.T) {
	for _, value := range []string{"90s", "30m", "0", "0s"} {
		t.Run(value, func(t *testing.T) {
			sess := &scriptedSession{outcome: domain.RunOutcome{Status: domain.RunCompleted}}
			args := append(baseHarnessArgs(), "--no-output-timeout", value)

			code, _, errOut := runCLIWithStore(t, args, &spyStore{}, sess)

			if code != cli.ExitSuccess {
				t.Errorf("exit code = %d, want ExitSuccess (%d); stderr: %q", code, cli.ExitSuccess, errOut)
			}
			if !sess.called {
				t.Error("session.Start was not called for a valid --no-output-timeout value")
			}
		})
	}
}

func TestNoOutputTimeoutFlag_RejectedValuesAreUsageErrors(t *testing.T) {
	for _, value := range []string{"not-a-duration", "-5m", "-1s"} {
		t.Run(value, func(t *testing.T) {
			sess := &scriptedSession{}
			args := append(baseHarnessArgs(), "--no-output-timeout", value)

			code, _, errOut := runCLI(t, args, sess)

			if code != cli.ExitUsage {
				t.Errorf("exit code = %d, want ExitUsage (%d)", code, cli.ExitUsage)
			}
			if !strings.Contains(errOut, value) || !strings.Contains(errOut, "no-output-timeout") {
				t.Errorf("stderr %q should name the flag and the rejected value", errOut)
			}
			if sess.called {
				t.Error("session.Start should not be called for an invalid --no-output-timeout value")
			}
		})
	}
}
