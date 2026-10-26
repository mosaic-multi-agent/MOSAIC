package cli_test

// The anti-loop guard can only be switched off in the TUI: the CLI registers no
// flag that does it. This is regression coverage for a rule the CLI must keep.

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"mosaic-run/internal/cli"
)

// guardWords are the words a flag name or help text would use to switch the
// anti-loop guard off.
var guardWords = []string{"guard", "anti-loop", "antiloop", "loop-guard", "waive", "waiver"}

func TestRunFlags_NoFlagSwitchesTheAntiLoopGuardOff(t *testing.T) {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	cli.RegisterRunFlags(fs)

	fs.VisitAll(func(f *pflag.Flag) {
		text := strings.ToLower(f.Name + " " + f.Usage)
		for _, w := range guardWords {
			if strings.Contains(text, w) {
				t.Errorf("flag --%s (%q) mentions %q: the CLI must have no way to switch the anti-loop guard off", f.Name, f.Usage, w)
			}
		}
	})
}

func TestFlagSpecs_NoEntryNamesTheAntiLoopGuard(t *testing.T) {
	for _, spec := range append(cli.RunFlagSpecs(), cli.TestFlagSpecs()...) {
		name := strings.ToLower(spec.Name)
		for _, w := range guardWords {
			if strings.Contains(name, w) {
				t.Errorf("flag spec %s mentions %q: the CLI must have no way to switch the anti-loop guard off", spec.Name, w)
			}
		}
	}
}
