package claudecode_test

// Pins the literal values the adapter provisions with, so a wrong mapping from
// the shared descriptor (for example agents directory read as hooks directory)
// is caught here rather than passing unnoticed.

import (
	"testing"

	"mosaic-agent-test/internal/harness/claudecode"
)

func TestLayoutValues_MatchProvisionedClaudeCodeLayout(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"harness id", claudecode.HarnessID, "claude-code"},
		{"agents directory", claudecode.AgentsRelDir, ".claude/agents"},
		{"hooks directory", claudecode.HooksRelDir, ".claude/hooks"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}
