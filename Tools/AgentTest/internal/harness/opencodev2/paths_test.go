package opencodev2_test

// Pins the literal identity and layout values the adapter provisions with, so
// a wrong mapping from the shared descriptor is caught here.

import (
	"testing"

	"mosaic-agent-test/internal/harness/opencodev2"
)

func TestLayoutValues_MatchProvisionedOpenCodeV2Layout(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"harness id", opencodev2.HarnessID, "opencode-v2"},
		{"plugins directory", opencodev2.PluginsRelDir, ".opencode/plugins"},
		{"interceptor file name", opencodev2.PluginFileName, "mosaic-interceptor.ts"},
		{"adapter id", opencodev2.New(opencodev2.Options{}).ID(), "opencode-v2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
			}
		})
	}
}
