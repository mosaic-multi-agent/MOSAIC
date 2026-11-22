package opencodev2_test

// Tests for configuration-scope enumeration and inspection. V2 reads user
// config from two files (opencode.json, opencode.jsonc) and two plugin
// folders (plugins/, plugin/). Only a non-empty "plugins" or "plugin" entry,
// or a non-empty plugin folder, can register a hook that rewrites the
// intercepted call; MCP configuration never counts. Absent, unreadable and
// malformed scopes never read as an ordinary clean result.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

var userScopeNames = []string{"user-config", "user-config-jsonc", "user-plugins", "user-plugin-legacy"}

func scopeNames(scopes []domain.ConfigScope) []string {
	names := make([]string, 0, len(scopes))
	for _, s := range scopes {
		names = append(names, s.Name)
	}
	return names
}

func inspect(t *testing.T, probe opencodev2.ScopeProbe) []domain.ScopeFinding {
	t.Helper()
	findings, err := opencodev2.New(opencodev2.Options{ScopeProbe: probe}).InspectScopes(testContext())
	if err != nil {
		t.Fatalf("InspectScopes returned error %v, want nil", err)
	}
	return findings
}

func findingFor(t *testing.T, findings []domain.ScopeFinding, name string) domain.ScopeFinding {
	t.Helper()
	for _, f := range findings {
		if f.Scope.Name == name {
			return f
		}
	}
	t.Fatalf("no finding for scope %q among %+v", name, findings)
	return domain.ScopeFinding{}
}

func TestConfigScopes_OrderAndFlags(t *testing.T) {
	scopes := opencodev2.New(opencodev2.Options{}).ConfigScopes()

	want := append(append([]string{}, userScopeNames...), "sandbox")
	if !reflect.DeepEqual(scopeNames(scopes), want) {
		t.Fatalf("scope names = %v, want %v (lowest precedence first)", scopeNames(scopes), want)
	}
	for _, s := range scopes[:4] {
		if s.InSandbox || s.Isolatable {
			t.Errorf("scope %q = %+v, want InSandbox false and Isolatable false", s.Name, s)
		}
	}
	sandbox := scopes[4]
	if !sandbox.InSandbox || sandbox.Isolatable {
		t.Errorf("sandbox scope = %+v, want InSandbox true and Isolatable false", sandbox)
	}
	if !strings.HasSuffix(filepath.ToSlash(sandbox.Path), ".opencode/plugins") {
		t.Errorf("sandbox scope path = %q, want it to end in .opencode/plugins", sandbox.Path)
	}
}

func TestConfigScopes_UserPathsFollowXDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	scopes := opencodev2.New(opencodev2.Options{}).ConfigScopes()

	dir := filepath.Join(xdg, "opencode")
	want := map[string]string{
		"user-config":        filepath.Join(dir, "opencode.json"),
		"user-config-jsonc":  filepath.Join(dir, "opencode.jsonc"),
		"user-plugins":       filepath.Join(dir, "plugins"),
		"user-plugin-legacy": filepath.Join(dir, "plugin"),
	}
	for name, w := range want {
		s, ok := scopeByName(scopes, name)
		if !ok {
			t.Errorf("no scope named %q among %v", name, scopeNames(scopes))
			continue
		}
		if s.Path != w {
			t.Errorf("scope %q path = %q, want %q", name, s.Path, w)
		}
	}
}

func scopeByName(scopes []domain.ConfigScope, name string) (domain.ConfigScope, bool) {
	for _, s := range scopes {
		if s.Name == name {
			return s, true
		}
	}
	return domain.ConfigScope{}, false
}

func TestConfigScopes_UserPathsFallBackToHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatalf("unset XDG_CONFIG_HOME: %v", err)
	}

	scopes := opencodev2.New(opencodev2.Options{}).ConfigScopes()

	want := filepath.Join(home, ".config", "opencode", "opencode.json")
	s, ok := scopeByName(scopes, "user-config")
	if !ok {
		t.Fatalf("no user-config scope among %v", scopeNames(scopes))
	}
	if s.Path != want {
		t.Errorf("user-config path = %q, want %q", s.Path, want)
	}
}

func TestInspectScopes_OneFindingPerNonSandboxScopeAndNeverProbesSandbox(t *testing.T) {
	var asked []string
	findings := inspect(t, fixtureScopeProbe{read: &asked})

	var got []string
	for _, f := range findings {
		got = append(got, f.Scope.Name)
		if f.Scope.InSandbox {
			t.Errorf("finding for in-sandbox scope %q, want none", f.Scope.Name)
		}
	}
	if !reflect.DeepEqual(got, userScopeNames) {
		t.Errorf("finding scope names = %v, want %v", got, userScopeNames)
	}
	if !reflect.DeepEqual(asked, userScopeNames) {
		t.Errorf("probe was asked for %v, want exactly %v", asked, userScopeNames)
	}
}

func TestInspectScopes_PluginEntryInConfigRewritesInput(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		doc   string
		key   string
	}{
		{"plugin key in json", "user-config", "user-config-with-plugin.json", `"plugin"`},
		{"plugins key in jsonc with comments and trailing comma", "user-config-jsonc", "user-config-with-plugins.jsonc", `"plugins"`},
		{"plugins key in the json file too", "user-config", "user-config-with-plugins.jsonc", `"plugins"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := findingFor(t, inspect(t, fixtureScopeProbe{docs: map[string][]byte{c.scope: readScopeFixture(t, c.doc)}}), c.scope)

			if !f.RewritesInput {
				t.Fatalf("finding = %+v, want RewritesInput", f)
			}
			if f.Neutralized || !f.PreservesSubjectFunction {
				t.Errorf("finding = %+v, want Neutralized false and PreservesSubjectFunction true", f)
			}
			if !strings.Contains(f.Detail, c.scope) || !strings.Contains(f.Detail, c.key) {
				t.Errorf("Detail = %q, want it to name the scope %q and the key %s", f.Detail, c.scope, c.key)
			}
		})
	}
}

func TestInspectScopes_ConfigWithEmptyPluginKeysAndMCPServersIsClean(t *testing.T) {
	doc := readScopeFixture(t, "user-config-clean.json")

	for _, scope := range []string{"user-config", "user-config-jsonc"} {
		f := findingFor(t, inspect(t, fixtureScopeProbe{docs: map[string][]byte{scope: doc}}), scope)

		if f.RewritesInput {
			t.Errorf("%s: finding = %+v, want no rewrite for empty plugin keys and an MCP server entry", scope, f)
		}
		if f.Neutralized || !f.PreservesSubjectFunction {
			t.Errorf("%s: finding = %+v, want Neutralized false and PreservesSubjectFunction true", scope, f)
		}
	}
}

func TestInspectScopes_MalformedOrNonObjectConfigIsReportedMalformed(t *testing.T) {
	for _, name := range []string{"user-config-malformed.json", "user-config-not-object.json"} {
		t.Run(name, func(t *testing.T) {
			probe := fixtureScopeProbe{docs: map[string][]byte{"user-config": readScopeFixture(t, name)}}

			f := findingFor(t, inspect(t, probe), "user-config")

			if !f.RewritesInput {
				t.Errorf("finding = %+v, want RewritesInput for a document that cannot be judged", f)
			}
			if !strings.Contains(f.Detail, "malformed") {
				t.Errorf("Detail = %q, want it to say the document is malformed", f.Detail)
			}
		})
	}
}

func TestInspectScopes_PluginFolderListingDecidesRewrite(t *testing.T) {
	for _, scope := range []string{"user-plugins", "user-plugin-legacy"} {
		t.Run(scope, func(t *testing.T) {
			full := findingFor(t, inspect(t, fixtureScopeProbe{docs: map[string][]byte{scope: []byte("third-party.ts\n")}}), scope)
			if !full.RewritesInput || !strings.Contains(full.Detail, scope) {
				t.Errorf("non-empty listing: finding = %+v, want a rewrite whose Detail names %q", full, scope)
			}

			for _, blank := range []string{"", "  \n"} {
				empty := findingFor(t, inspect(t, fixtureScopeProbe{docs: map[string][]byte{scope: []byte(blank)}}), scope)
				if empty.RewritesInput {
					t.Errorf("blank listing %q: finding = %+v, want no rewrite", blank, empty)
				}
			}
		})
	}
}

func TestInspectScopes_AbsentScopeIsTreatedAsAbsentNotClean(t *testing.T) {
	f := findingFor(t, inspect(t, fixtureScopeProbe{}), "user-config")

	if f.RewritesInput || f.Neutralized || !f.PreservesSubjectFunction {
		t.Errorf("finding = %+v, want no rewrite, Neutralized false (not isolatable) and PreservesSubjectFunction true", f)
	}
	if !strings.Contains(f.Detail, "not found") {
		t.Errorf("Detail = %q, want it to say the scope was not found", f.Detail)
	}
}

func TestInspectScopes_UnreadableScopeIsTreatedAsUnresolved(t *testing.T) {
	probe := fixtureScopeProbe{errs: map[string]error{"user-plugins": errors.New("access denied")}}

	f := findingFor(t, inspect(t, probe), "user-plugins")

	if !f.RewritesInput {
		t.Errorf("finding = %+v, want RewritesInput for an unreadable scope", f)
	}
	if !strings.Contains(f.Detail, "could not be read") || !strings.Contains(f.Detail, "access denied") {
		t.Errorf("Detail = %q, want it to say the scope could not be read and carry the cause", f.Detail)
	}
}

// --- the real, read-only probe ---

func TestInspectScopes_RealProbeReadsRelocatedHomeAndNeverWrites(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	dir := filepath.Join(xdg, "opencode")
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "third-party.ts"), []byte("//"), 0o644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), readScopeFixture(t, "user-config-clean.json"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	before := listDir(t, xdg)

	findings := inspect(t, nil)

	if f := findingFor(t, findings, "user-plugins"); !f.RewritesInput {
		t.Errorf("user-plugins finding = %+v, want a rewrite for a folder holding a plugin", f)
	}
	if f := findingFor(t, findings, "user-config"); f.RewritesInput {
		t.Errorf("user-config finding = %+v, want clean", f)
	}
	for _, absent := range []string{"user-config-jsonc", "user-plugin-legacy"} {
		f := findingFor(t, findings, absent)
		if f.RewritesInput || !strings.Contains(f.Detail, "not found") {
			t.Errorf("%s finding = %+v, want absent (no rewrite, not found)", absent, f)
		}
	}
	if after := listDir(t, xdg); !reflect.DeepEqual(before, after) {
		t.Errorf("inspection changed the config home from %v to %v", before, after)
	}
}
