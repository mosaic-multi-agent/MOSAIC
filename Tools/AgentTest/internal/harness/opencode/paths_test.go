package opencode_test

// Pins the literal values the adapter provisions with, so a wrong mapping from
// the shared descriptor (notably the descriptor's hooks directory serving as
// the plugins directory) is caught here rather than passing unnoticed.

import (
	"testing"

	"mosaic-agent-test/internal/harness/opencode"
)

func TestLayoutValues_MatchProvisionedOpenCodeLayout(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"harness id", opencode.HarnessID, "opencode"},
		{"agents directory", opencode.AgentsRelDir, ".opencode/agents"},
		{"plugins directory", opencode.PluginsRelDir, ".opencode/plugins"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}
