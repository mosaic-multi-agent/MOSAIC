package main

// Composition-root wiring of the two-level configuration. These tests build the
// real binary, point the user config folder at a temporary folder through the
// environment variables os.UserConfigDir reads, and start it from a temporary
// project folder.
//
// Rules under test:
//   - the global files live in <user config folder>/MOSAIC/LogAnalyzer and are
//     written from comment-only templates on first start, never overwritten;
//   - the global files are found whatever folder the program is started from;
//   - starting the program creates no project file.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// configEnv returns the environment entries that make os.UserConfigDir resolve to
// a folder under base on every supported OS.
func configEnv(base string) []string {
	return []string{
		"APPDATA=" + base,
		"XDG_CONFIG_HOME=" + base,
		"HOME=" + base,
	}
}

// globalDirUnder returns where the program must keep its global files when the
// environment was set up with configEnv(base).
func globalDirUnder(base string) string {
	userCfg := base
	if runtime.GOOS == "darwin" {
		userCfg = filepath.Join(base, "Library", "Application Support")
	}
	return filepath.Join(userCfg, "MOSAIC", "LogAnalyzer")
}

func activeYAMLLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			out = append(out, line)
		}
	}
	return out
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestBinary_FirstStart_WritesBothGlobalFilesFromCommentOnlyTemplates(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()

	code, _, stderr := runBinaryIn(t, bin, project, base, "pricing", "path")

	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %q)", code, stderr)
	}
	for _, name := range []string{"pricing.yaml", "analysis.yaml"} {
		content := readText(t, filepath.Join(globalDirUnder(base), name))
		if !strings.Contains(content, "#") {
			t.Errorf("%s should hold the documented template", name)
		}
		if active := activeYAMLLines(content); len(active) != 0 {
			t.Errorf("%s template must hold comments only, active lines: %v", name, active)
		}
	}
}

func TestBinary_GlobalFilesPresent_AreNeverOverwritten(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()
	dir := globalDirUnder(base)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const pricingYAML = "# mine\n"
	const analysisYAML = "detected_compaction:\n  drop_percent: 40\n"
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte(pricingYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "analysis.yaml"), []byte(analysisYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	runBinaryIn(t, bin, project, base, "settings", "path")

	if got := readText(t, filepath.Join(dir, "pricing.yaml")); got != pricingYAML {
		t.Errorf("pricing.yaml was overwritten: %q", got)
	}
	if got := readText(t, filepath.Join(dir, "analysis.yaml")); got != analysisYAML {
		t.Errorf("analysis.yaml was overwritten: %q", got)
	}
}

func TestBinary_Start_CreatesNoProjectFile(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()

	runBinaryIn(t, bin, project, base, "pricing", "path")
	runBinaryIn(t, bin, project, base, "settings", "path")

	if _, err := os.Stat(filepath.Join(project, "MosaicLogAnalyzer")); !os.IsNotExist(err) {
		t.Errorf("starting the program must not create the project config folder; stat err = %v", err)
	}
}

func TestBinary_PathCommands_PrintBothLevelsUnderTheUserConfigFolder(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()
	dir := globalDirUnder(base)

	_, pricingOut, _ := runBinaryIn(t, bin, project, base, "pricing", "path")
	_, settingsOut, _ := runBinaryIn(t, bin, project, base, "settings", "path")

	wantPricing := "global:  " + filepath.Join(dir, "pricing.yaml") + " (exists)\n" +
		"project: " + filepath.Join(project, "MosaicLogAnalyzer", "config", "pricing.yaml") + " (absent)\n"
	wantSettings := "global:  " + filepath.Join(dir, "analysis.yaml") + " (exists)\n" +
		"project: " + filepath.Join(project, "MosaicLogAnalyzer", "config", "analysis.yaml") + " (absent)\n"
	if pricingOut != wantPricing {
		t.Errorf("pricing path stdout = %q, want %q", pricingOut, wantPricing)
	}
	if settingsOut != wantSettings {
		t.Errorf("settings path stdout = %q, want %q", settingsOut, wantSettings)
	}
}

func TestBinary_GlobalPricing_IsAppliedWhenStartedFromAnyFolder(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()
	logs := filepath.Join(t.TempDir(), "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "pendingusage", "runs"), logs)
	dir := globalDirUnder(base)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const pricingYAML = "models:\n  claude-sonnet-4:\n    input: \"3.00\"\n    cache_read: \"0.30\"\n" +
		"    cache_write_5m: \"3.75\"\n    cache_write_1h: \"6.00\"\n    output: \"15.00\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte(pricingYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runBinaryIn(t, bin, project, base,
		"total", "--format", "json", "--path", logs, "--run", "20261031T110000Z-c001")

	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %q)", code, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("total output is not JSON: %v\n%s", err, stdout)
	}
	money, _ := got["money"].(map[string]any)
	if money["state"] == "unpriced" {
		t.Errorf("the model is priced by the global file, but total reports %v", got)
	}
	if strings.Contains(stdout, `"unpriced_models":["claude-sonnet-4"]`) {
		t.Errorf("claude-sonnet-4 must not be unpriced, got %s", stdout)
	}
}

func TestBinary_ProjectPricing_OverridesGlobalPerModel(t *testing.T) {
	bin := buildBinary(t)
	base, project := t.TempDir(), t.TempDir()
	logs := filepath.Join(t.TempDir(), "logs")
	copyDir(t, filepath.Join("..", "..", "testdata", "pendingusage", "runs"), logs)
	dir := globalDirUnder(base)
	cfgDir := filepath.Join(project, "MosaicLogAnalyzer", "config")
	for _, d := range []string{dir, cfgDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rates := func(input string) string {
		return "models:\n  claude-sonnet-4:\n    input: \"" + input + "\"\n    cache_read: \"0.30\"\n" +
			"    cache_write_5m: \"3.75\"\n    cache_write_1h: \"6.00\"\n    output: \"15.00\"\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "pricing.yaml"), []byte(rates("3.00")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "pricing.yaml"), []byte(rates("30.00")), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"total", "--format", "json", "--path", logs, "--run", "20261031T110000Z-c001"}

	_, withBoth, _ := runBinaryIn(t, bin, project, base, args...)
	if err := os.Remove(filepath.Join(cfgDir, "pricing.yaml")); err != nil {
		t.Fatal(err)
	}
	_, globalOnly, _ := runBinaryIn(t, bin, project, base, args...)

	if strings.Contains(globalOnly, `"state":"unpriced"`) {
		t.Fatalf("the global file alone must price the model, got %s", globalOnly)
	}
	if strings.Contains(withBoth, `"state":"unpriced"`) {
		t.Fatalf("total must be priced with both levels present, got %s", withBoth)
	}
	if withBoth == globalOnly {
		t.Errorf("a project rate for the same model must change the priced total:\n%s", withBoth)
	}
}
