package main

// Tests for the version request: how an invocation is recognised, which
// frontend it selects, and the single line it prints.
//
// Chosen behaviour for combinations (common CLI convention): a "--version"
// flag token anywhere in the arguments wins over every other command and
// flag, except that (a) the interceptor route keeps absolute priority when
// the first argument is "intercept", and (b) a "--version" token that is the
// value of a value-consuming flag, or that carries an "=value", is not a
// version request.

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// failIfConsulted is a TerminalCheck that reports both streams as terminals
// and fails the test when called, proving the decision did not depend on it.
func failIfConsulted(t *testing.T) TerminalCheck {
	t.Helper()
	return func() (bool, bool) {
		t.Error("TerminalCheck was consulted; a version request must not depend on terminal state")
		return true, true
	}
}

func TestVersionRequested_LoneVersionFlag_IsTrue(t *testing.T) {
	if !versionRequested([]string{"--version"}) {
		t.Error(`versionRequested(["--version"]) = false, want true`)
	}
}

func TestVersionRequested_Combinations(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"flag after command", []string{"run", "suite.yaml", "--version"}, true},
		{"flag before command", []string{"--version", "run", "suite.yaml"}, true},
		{"flag with help", []string{"--help", "--version"}, true},
		{"flag with boolean flag", []string{"--tui", "--version"}, true},
		{"value of a value-consuming flag", []string{"run", "suite.yaml", "--harness", "--version"}, false},
		{"equals form", []string{"--version=1"}, false},
		{"no version token", []string{"run", "suite.yaml"}, false},
		{"empty", nil, false},
		{"similar token", []string{"--versions"}, false},
		{"version as a positional", []string{"version"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionRequested(tc.args); got != tc.want {
				t.Errorf("versionRequested(%q) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestSelectFrontend_LoneVersion_RoutesToVersionWithoutConsultingTerminal(t *testing.T) {
	got := selectFrontend([]string{"--version"}, failIfConsulted(t))

	if got != FrontendVersion {
		t.Errorf("selectFrontend([--version], terminals) = %q, want %q", got, FrontendVersion)
	}
}

func TestSelectFrontend_LoneVersion_RoutesToVersionWhenNotTerminals(t *testing.T) {
	got := selectFrontend([]string{"--version"}, neverTerminal)

	if got != FrontendVersion {
		t.Errorf("selectFrontend([--version], neverTerminal) = %q, want %q", got, FrontendVersion)
	}
}

func TestSelectFrontend_VersionCombinedWithOthers_RoutesToVersion(t *testing.T) {
	for _, args := range [][]string{
		{"run", "suite.yaml", "--version"},
		{"--version", "run", "suite.yaml"},
		{"--tui", "--version"},
	} {
		if got := selectFrontend(args, alwaysTerminal); got != FrontendVersion {
			t.Errorf("selectFrontend(%q, ...) = %q, want %q", args, got, FrontendVersion)
		}
	}
}

func TestSelectFrontend_HelpWithVersion_RoutesToVersion(t *testing.T) {
	got := selectFrontend([]string{"--help", "--version"}, alwaysTerminal)

	if got != FrontendVersion {
		t.Errorf("selectFrontend([--help --version], ...) = %q, want %q", got, FrontendVersion)
	}
}

func TestSelectFrontend_InterceptWithVersionAfterIt_StillRoutesToInterceptor(t *testing.T) {
	for _, args := range [][]string{
		{InterceptorSubcommand, "--version"},
		{InterceptorSubcommand, "--control-dir", "C:/sandbox/control", "--version"},
	} {
		if got := selectFrontend(args, failIfConsulted(t)); got != FrontendInterceptor {
			t.Errorf("selectFrontend(%q, ...) = %q, want %q", args, got, FrontendInterceptor)
		}
	}
}

func TestSelectFrontend_VersionAsValueOfFlag_FollowsExistingRules(t *testing.T) {
	got := selectFrontend([]string{"run", "suite.yaml", "--harness", "--version"}, alwaysTerminal)

	if got != FrontendCLI {
		t.Errorf("selectFrontend(--harness --version value form) = %q, want %q", got, FrontendCLI)
	}
}

func TestRunVersion_WritesExactlyOneLineAndReturnsZero(t *testing.T) {
	var out bytes.Buffer

	code := runVersion(&out)

	if code != 0 {
		t.Errorf("runVersion exit code = %d, want 0", code)
	}
	want := "mosaic-agent-test " + ToolVersion + "\n"
	if out.String() != want {
		t.Errorf("runVersion output = %q, want %q", out.String(), want)
	}
}

func TestRunVersion_LineUsesFixedToolNameWithoutExeSuffix(t *testing.T) {
	var out bytes.Buffer

	runVersion(&out)

	if strings.Contains(out.String(), ".exe") {
		t.Errorf("runVersion output %q contains .exe; the tool name is fixed", out.String())
	}
	if !strings.HasPrefix(out.String(), "mosaic-agent-test ") {
		t.Errorf("runVersion output %q does not start with the fixed tool name", out.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestRunVersion_WriteFailure_ReturnsNonZero(t *testing.T) {
	if code := runVersion(failingWriter{}); code == 0 {
		t.Error("runVersion with a failing writer returned 0, want a failure exit code")
	}
}

func TestToolVersion_IsBumped(t *testing.T) {
	if ToolVersion != "1.2.1" {
		t.Errorf("ToolVersion = %q, want %q", ToolVersion, "1.2.1")
	}
}
