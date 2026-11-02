package cli_test

// Tests for `pricing path` and `settings path` printing both configuration levels.
// They use the real stores on temporary folders, so no real user config folder is touched.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/pricing"
	"mosaic-log-analyzer/internal/settings"
)

// pathsFixture holds a service wired to real stores plus the four file locations.
type pathsFixture struct {
	svc                    *app.Service
	globalPricing, projPri string
	globalSettings, projSe string
}

// newPathsFixture wires real stores; globalDir "" makes the global level unavailable.
func newPathsFixture(t *testing.T, withGlobal bool) pathsFixture {
	t.Helper()
	globalDir, root := "", t.TempDir()
	if withGlobal {
		globalDir = filepath.Join(t.TempDir(), "MOSAIC", "LogAnalyzer")
	}
	return pathsFixture{
		svc: app.New(app.Deps{
			Source:      &fakeLogSource{},
			Reader:      newFakeEventReader(),
			Pricing:     pricing.NewStore(globalDir, root),
			Settings:    settings.NewStore(globalDir, root),
			Clock:       newFakeClock(),
			Interaction: &alwaysSkippingInteraction{},
			WorkDir:     root,
		}),
		globalPricing:  filepath.Join(globalDir, "pricing.yaml"),
		projPri:        filepath.Join(root, "MosaicLogAnalyzer", "config", "pricing.yaml"),
		globalSettings: filepath.Join(globalDir, "analysis.yaml"),
		projSe:         filepath.Join(root, "MosaicLogAnalyzer", "config", "analysis.yaml"),
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runPathCommand(t *testing.T, f pathsFixture, args ...string) (int, string) {
	t.Helper()
	stdout := &bytes.Buffer{}
	code := cli.Execute(cli.Options{Service: f.svc, Stdout: stdout, Stderr: &bytes.Buffer{}}, args)
	return code, stdout.String()
}

func TestPricingPath_PrintsBothLevelsAsAbsentWhenNoFileExists(t *testing.T) {
	f := newPathsFixture(t, true)

	code, out := runPathCommand(t, f, "pricing", "path")

	want := "global:  " + f.globalPricing + " (absent)\nproject: " + f.projPri + " (absent)\n"
	if code != cli.ExitSuccess || out != want {
		t.Errorf("exit %d, stdout %q; want exit 0 and %q", code, out, want)
	}
}

func TestPricingPath_PrintsExistsForLevelsWhoseFileIsPresent(t *testing.T) {
	f := newPathsFixture(t, true)
	touch(t, f.globalPricing)

	_, out := runPathCommand(t, f, "pricing", "path")

	want := "global:  " + f.globalPricing + " (exists)\nproject: " + f.projPri + " (absent)\n"
	if out != want {
		t.Errorf("stdout %q, want %q", out, want)
	}
}

func TestPricingPath_ProjectFilePresent_PrintsProjectExists(t *testing.T) {
	f := newPathsFixture(t, true)
	touch(t, f.projPri)

	_, out := runPathCommand(t, f, "pricing", "path")

	want := "global:  " + f.globalPricing + " (absent)\nproject: " + f.projPri + " (exists)\n"
	if out != want {
		t.Errorf("stdout %q, want %q", out, want)
	}
}

func TestPricingPath_GlobalFolderUnavailable_SaysSoOnTheGlobalLine(t *testing.T) {
	f := newPathsFixture(t, false)

	code, out := runPathCommand(t, f, "pricing", "path")

	want := "global:  (unavailable: user config folder not found)\nproject: " + f.projPri + " (absent)\n"
	if code != cli.ExitSuccess || out != want {
		t.Errorf("exit %d, stdout %q; want exit 0 and %q", code, out, want)
	}
}

func TestPricingPath_NeverCreatesAnyFile(t *testing.T) {
	f := newPathsFixture(t, true)

	runPathCommand(t, f, "pricing", "path")

	for _, p := range []string{f.globalPricing, f.projPri} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("`pricing path` created %s", p)
		}
	}
}

func TestSettingsPath_PrintsBothLevelsWithExistence(t *testing.T) {
	f := newPathsFixture(t, true)
	touch(t, f.projSe)

	code, out := runPathCommand(t, f, "settings", "path")

	want := "global:  " + f.globalSettings + " (absent)\nproject: " + f.projSe + " (exists)\n"
	if code != cli.ExitSuccess || out != want {
		t.Errorf("exit %d, stdout %q; want exit 0 and %q", code, out, want)
	}
}

func TestSettingsPath_GlobalFolderUnavailable_SaysSoOnTheGlobalLine(t *testing.T) {
	f := newPathsFixture(t, false)

	_, out := runPathCommand(t, f, "settings", "path")

	want := "global:  (unavailable: user config folder not found)\nproject: " + f.projSe + " (absent)\n"
	if out != want {
		t.Errorf("stdout %q, want %q", out, want)
	}
}

func TestSettingsPath_DoesNotPrintPricingFilesAndCreatesNothing(t *testing.T) {
	f := newPathsFixture(t, true)

	_, out := runPathCommand(t, f, "settings", "path")

	if bytes.Contains([]byte(out), []byte("pricing.yaml")) {
		t.Errorf("`settings path` must not print pricing files, got %q", out)
	}
	for _, p := range []string{f.globalSettings, f.projSe} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("`settings path` created %s", p)
		}
	}
}
