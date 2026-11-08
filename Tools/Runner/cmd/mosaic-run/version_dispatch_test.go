package main

import "testing"

// terminalMustNotBeAsked returns a terminal predicate that fails the test when called.
func terminalMustNotBeAsked(t *testing.T) func() bool {
	t.Helper()
	return func() bool {
		t.Error("terminal state was consulted for an invocation that contains --version")
		return true
	}
}

func TestSelectFrontend_LoneVersion_IsVersionEvenOnTerminal(t *testing.T) {
	got := selectFrontend([]string{"--version"}, terminalMustNotBeAsked(t))

	if got != frontendVersion {
		t.Fatalf("selectFrontend([--version]) = %q, want %q", got, frontendVersion)
	}
}

func TestSelectFrontend_VersionCombinedWithOtherArgs_IsUsageErrorRegardlessOfTerminal(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"version before subcommand", []string{"--version", "run"}},
		{"version after subcommand", []string{"run", "--version"}},
		{"version with tui", []string{"--tui", "--version"}},
		{"version with dev", []string{"--dev", "--version"}},
		{"version with value", []string{"--version=1"}},
		{"version twice", []string{"--version", "--version"}},
		{"version as value of value-bearing flag", []string{"run", "--harness", "--version"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectFrontend(tc.args, terminalMustNotBeAsked(t))

			if got != frontendVersionUsage {
				t.Fatalf("selectFrontend(%v) = %q, want %q", tc.args, got, frontendVersionUsage)
			}
		})
	}
}

// Invocations without --version keep their existing routing (regression of the
// unchanged rules, now expressed through the dispatch seam).
func TestSelectFrontend_WithoutVersion_KeepsExistingRouting(t *testing.T) {
	yes := func() bool { return true }
	no := func() bool { return false }
	cases := []struct {
		name     string
		args     []string
		terminal func() bool
		want     frontend
	}{
		{"no args on terminal", nil, yes, frontendTUI},
		{"no args piped", nil, no, frontendCLI},
		{"tui flag piped", []string{"--tui"}, no, frontendTUI},
		{"run subcommand on terminal", []string{"run", "--harness", "fake"}, yes, frontendCLI},
		{"flag-only on terminal", []string{"--allow-version-drift"}, yes, frontendTUI},
		{"flag-only piped", []string{"--allow-version-drift"}, no, frontendCLI},
		{"dev alone on terminal", []string{"--dev"}, yes, frontendTUI},
		{"dev test routing", []string{"--dev", "test"}, yes, frontendDevTest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectFrontend(tc.args, tc.terminal); got != tc.want {
				t.Fatalf("selectFrontend(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
