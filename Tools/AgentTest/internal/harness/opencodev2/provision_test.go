package opencodev2_test

// Tests for provisioning and deprovisioning. V2 loads only top-level files of
// .opencode/plugins/ as plugins, so the logger variant's entry lands at the
// top level and its helpers in a subfolder. Anything already present in
// either project plugin folder, or any plugin entry in project config, is a
// foreign rewriter and is refused before anything is written.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-common/hookbundle"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func provisionWithBundle(t *testing.T, bundle string) (domain.Sandbox, domain.Provisioning, error) {
	t.Helper()
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.LoggerBundleDir = fixtureBundleDir(t, bundle)
	prov, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)
	return sb, prov, err
}

func pluginsDir(sb domain.Sandbox) string {
	return filepath.Join(sb.SubjectDir, filepath.FromSlash(opencodev2.PluginsRelDir))
}

func writeProjectFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %q: %v", path, err)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestProvision_PlacesLoggerVariantEntryAtTopLevelAndHelpersInSubfolder(t *testing.T) {
	sb, prov, err := provisionWithBundle(t, "fixture-bundle")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	want := map[string]string{
		"mosaic-logger.ts": "logger_entry",
		"lib/host.ts":      "logger_host",
		"lib/shared.ts":    "logger_shared",
	}
	for target, marker := range want {
		full := filepath.Join(pluginsDir(sb), filepath.FromSlash(target))
		data, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("deployed file %q missing: %v", target, err)
			continue
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("deployed file %q = %q, want it to carry %q", target, data, marker)
		}
		if !contains(prov.Files, full) {
			t.Errorf("Provisioning.Files = %v, want it to record %q", prov.Files, full)
		}
	}
	interceptor := filepath.Join(pluginsDir(sb), opencodev2.PluginFileName)
	if !contains(prov.Files, interceptor) {
		t.Errorf("Provisioning.Files = %v, want it to record the interceptor plugin %q", prov.Files, interceptor)
	}
	if len(prov.Files) != len(want)+1 {
		t.Errorf("Provisioning.Files = %v, want exactly the %d logger files plus the interceptor plugin", prov.Files, len(want))
	}
}

func TestProvision_WritesOnlyUnderTheSandboxPluginsDirectory(t *testing.T) {
	sb, prov, err := provisionWithBundle(t, "fixture-bundle")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	err = filepath.WalkDir(sb.SubjectDir, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if !d.IsDir() && !strings.HasPrefix(p, pluginsDir(sb)+string(filepath.Separator)) {
			t.Errorf("file %q written outside %q", p, pluginsDir(sb))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking subject dir: %v", err)
	}
	for _, d := range prov.Dirs {
		if !strings.HasPrefix(d, sb.SubjectDir) {
			t.Errorf("recorded directory %q is outside the subject dir", d)
		}
	}
}

func TestProvision_RecordsCreatedDirectoriesOutermostFirst(t *testing.T) {
	sb, prov, err := provisionWithBundle(t, "fixture-bundle")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	lib := filepath.Join(pluginsDir(sb), "lib")
	if !contains(prov.Dirs, lib) {
		t.Fatalf("Provisioning.Dirs = %v, want it to record the created helper directory %q", prov.Dirs, lib)
	}
	for i, d := range prov.Dirs {
		for _, later := range prov.Dirs[i+1:] {
			if strings.HasPrefix(d, later+string(filepath.Separator)) {
				t.Errorf("Dirs = %v: %q is recorded before its parent %q", prov.Dirs, d, later)
			}
		}
	}
}

func TestProvision_WithoutLoggerBundleSucceedsAndReportsFindings(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	a := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}})

	prov, err := a.Provision(testContext(), minimalProvisionRequest(sb))
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if len(prov.ScopeFindings) != 4 {
		t.Errorf("ScopeFindings has %d entries, want 4", len(prov.ScopeFindings))
	}
}

func TestProvision_ScopeFindingsPointAtRelocatedHomeAndAreNeutralized(t *testing.T) {
	sb, prov, err := provisionWithBundle(t, "fixture-bundle")
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if len(prov.ScopeFindings) != len(userScopeNames) {
		t.Fatalf("ScopeFindings = %+v, want %d findings", prov.ScopeFindings, len(userScopeNames))
	}
	home := filepath.Join(sb.ControlDir, "opencode-v2-home", "opencode")
	for i, f := range prov.ScopeFindings {
		if f.Scope.Name != userScopeNames[i] {
			t.Errorf("finding %d scope = %q, want %q", i, f.Scope.Name, userScopeNames[i])
		}
		if !strings.HasPrefix(f.Scope.Path, home+string(filepath.Separator)) {
			t.Errorf("finding %q path = %q, want it under %q", f.Scope.Name, f.Scope.Path, home)
		}
		if f.Scope.InSandbox || !f.Scope.Isolatable || !f.Neutralized || !f.PreservesSubjectFunction || f.RewritesInput {
			t.Errorf("finding = %+v, want InSandbox false, Isolatable, Neutralized and PreservesSubjectFunction true, no rewrite", f)
		}
	}
}

// --- logger bundle refusals ---

func TestProvision_RefusesBundleFileTargetEscapingPluginsDirectory(t *testing.T) {
	sb, _, err := provisionWithBundle(t, "fixture-bundle-escaping-target")
	if err == nil {
		t.Fatalf("Provision: want an error for a target escaping the plugins directory")
	}
	if !strings.Contains(err.Error(), "escaped-hook.ts") {
		t.Errorf("error = %q, want it to name the escaping target escaped-hook.ts", err.Error())
	}

	escaped := filepath.Clean(filepath.Join(pluginsDir(sb), "..", "..", "..", "..", "escaped-hook.ts"))
	if _, statErr := os.Stat(escaped); statErr == nil {
		t.Errorf("escaping target was written to %q", escaped)
	}
	if entries, rerr := os.ReadDir(pluginsDir(sb)); rerr == nil && len(entries) != 0 {
		t.Errorf("plugins directory holds %d entries after the refusal, want none", len(entries))
	}
}

func TestProvision_RefusesNonEmptyRegistrationNamingHarnessAndCount(t *testing.T) {
	_, _, err := provisionWithBundle(t, "fixture-bundle-nonempty-registration")
	if err == nil {
		t.Fatalf("Provision: want an error for a non-empty registration list")
	}
	if !strings.Contains(err.Error(), "opencode-v2") {
		t.Errorf("error = %q, want it to name opencode-v2", err.Error())
	}
	if !regexp.MustCompile(`\b1\b`).MatchString(err.Error()) || !strings.Contains(err.Error(), "registration") {
		t.Errorf("error = %q, want it to state the registration step count (1)", err.Error())
	}
}

func TestProvision_BundleWithoutV2VariantIsUnsupportedVariant(t *testing.T) {
	_, _, err := provisionWithBundle(t, "fixture-bundle-no-v2")

	if !errors.Is(err, hookbundle.ErrVariantUnsupported) {
		t.Fatalf("error = %v, want one wrapping hookbundle.ErrVariantUnsupported", err)
	}
	if !strings.Contains(err.Error(), "opencode-v2") {
		t.Errorf("error = %q, want it to name opencode-v2", err.Error())
	}
}

func TestProvision_V2VariantMarkedUnsupportedIsUnsupportedVariant(t *testing.T) {
	_, _, err := provisionWithBundle(t, "fixture-bundle-v2-marked-unsupported")

	if !errors.Is(err, hookbundle.ErrVariantUnsupported) {
		t.Fatalf("error = %v, want one wrapping hookbundle.ErrVariantUnsupported", err)
	}
	if !strings.Contains(err.Error(), "opencode-v2") {
		t.Errorf("error = %q, want it to name opencode-v2", err.Error())
	}
}

func TestProvision_MissingBundleDirectoryIsAnError(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	req := minimalProvisionRequest(sb)
	req.LoggerBundleDir = filepath.Join(t.TempDir(), "no-such-bundle")

	_, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)

	if err == nil {
		t.Fatalf("Provision: want an error for a missing bundle directory")
	}
	if !strings.Contains(err.Error(), "no-such-bundle") {
		t.Errorf("error = %q, want it to name the missing bundle path %q", err.Error(), req.LoggerBundleDir)
	}
}

func TestProvision_RefusesTwoFilesWithTheSameTarget(t *testing.T) {
	sb, _, err := provisionWithBundle(t, "fixture-bundle-duplicate-target")
	if err == nil {
		t.Fatalf("Provision: want an error for two files with the same target")
	}
	if !strings.Contains(err.Error(), "mosaic-logger.ts") {
		t.Errorf("error = %q, want it to name the shared target", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(pluginsDir(sb), "mosaic-logger.ts")); statErr == nil {
		t.Errorf("a file was written despite the conflicting targets")
	}
}

// --- foreign plugin refusal ---

func TestProvision_RefusesForeignPluginsAndLeavesThemUntouched(t *testing.T) {
	cases := []struct {
		name    string
		relPath string
		content string
	}{
		{"entry in plugins folder", ".opencode/plugins/someone-elses.ts", "// not ours"},
		{"entry in legacy plugin folder", ".opencode/plugin/legacy.ts", "// not ours"},
		{"plugin entry in root opencode.json", "opencode.json", `{"plugin": ["x"]}`},
		{"plugins entry in root opencode.jsonc", "opencode.jsonc", "{\n // c\n \"plugins\": [\"x\",],\n}"},
		{"plugin entry in .opencode/opencode.json", ".opencode/opencode.json", `{"plugin": ["x"]}`},
		{"plugins entry in .opencode/opencode.jsonc", ".opencode/opencode.jsonc", `{"plugins": ["x"]}`},
		{"malformed project config", "opencode.json", "{ not json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb := newSandbox(t, t.TempDir())
			foreign := filepath.Join(sb.SubjectDir, filepath.FromSlash(c.relPath))
			writeProjectFile(t, foreign, c.content)
			req := minimalProvisionRequest(sb)
			req.LoggerBundleDir = fixtureBundleDir(t, "fixture-bundle")

			_, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)

			if !errors.Is(err, opencodev2.ErrMultipleRewriters) {
				t.Fatalf("error = %v, want one wrapping ErrMultipleRewriters", err)
			}
			if !strings.Contains(err.Error(), filepath.Base(foreign)) {
				t.Errorf("error = %q, want it to name %q", err.Error(), filepath.Base(foreign))
			}
			if data, rerr := os.ReadFile(foreign); rerr != nil || string(data) != c.content {
				t.Errorf("foreign file changed or vanished: %q, %v", data, rerr)
			}
			if _, statErr := os.Stat(filepath.Join(pluginsDir(sb), "mosaic-logger.ts")); statErr == nil {
				t.Errorf("logger files were written despite the refusal")
			}
		})
	}
}

func TestProvision_ProjectConfigWithoutPluginEntriesIsAccepted(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	writeProjectFile(t, filepath.Join(sb.SubjectDir, "opencode.json"), string(readScopeFixture(t, "user-config-clean.json")))
	req := minimalProvisionRequest(sb)
	req.LoggerBundleDir = fixtureBundleDir(t, "fixture-bundle")

	_, err := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}}).Provision(testContext(), req)

	if err != nil {
		t.Errorf("Provision: %v, want success for project config with empty plugin keys and MCP servers", err)
	}
}

// --- deprovision ---

func TestDeprovision_RemovesExactlyWhatWasRecordedAndIsIdempotent(t *testing.T) {
	sb := newSandbox(t, t.TempDir())
	keep := filepath.Join(sb.SubjectDir, "keep.txt")
	writeProjectFile(t, keep, "mine")
	req := minimalProvisionRequest(sb)
	req.LoggerBundleDir = fixtureBundleDir(t, "fixture-bundle")
	a := opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}})
	prov, err := a.Provision(testContext(), req)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if len(prov.Files) == 0 {
		t.Fatalf("Provision recorded no files, nothing to remove")
	}

	if err := a.Deprovision(testContext(), prov); err != nil {
		t.Fatalf("Deprovision: %v", err)
	}
	if err := a.Deprovision(testContext(), prov); err != nil {
		t.Errorf("Deprovision (second call): %v, want nil", err)
	}

	for _, p := range append(append([]string{}, prov.Files...), prov.Dirs...) {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("recorded path %q still exists (stat error %v)", p, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("unrecorded file removed: %v", err)
	}
	if _, err := os.Stat(sb.SubjectDir); err != nil {
		t.Errorf("subject dir removed: %v", err)
	}
}
