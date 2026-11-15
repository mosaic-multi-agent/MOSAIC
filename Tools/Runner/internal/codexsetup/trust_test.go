package codexsetup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck_Trust_Rows(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		want    TrustStatus
	}{
		{"workspace trusted", "trusted_contact", TrustTrusted},
		{"explicit untrusted level", "untrusted_level", TrustUntrusted},
		{"entry without trust level", "entry_without_level", TrustUntrusted},
		{"no projects table", "no_projects", TrustUntrusted},
		{"trust recorded for a different path", "other_path_trusted", TrustUntrusted},
		{"sibling sharing only a string prefix", "sibling_prefix_trusted", TrustUntrusted},
		{"trusted ancestor covers the workspace", "parent_trusted", TrustTrusted},
		{"nearest entry wins over a trusted ancestor", "parent_trusted_workdir_untrusted", TrustUntrusted},
		{"nearest ancestor untrusted beats trusted grandparent", "grandparent_trusted_parent_untrusted", TrustUntrusted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)

			res := e.checkHome(t, tc.fixture, false)

			if res.Trust != tc.want {
				t.Fatalf("Trust = %q, want %q", res.Trust, tc.want)
			}
			if res.TrustReason != "" {
				t.Errorf("TrustReason = %q, want empty for a decided check", res.TrustReason)
			}
		})
	}
}

func TestCheck_Trust_MissingHomeConfigIsUntrusted(t *testing.T) {
	e := newFixtureEnv(t) // CODEX_HOME exists but holds no config.toml

	res := e.checker(false).Check(e.workDir)

	if res.Trust != TrustUntrusted {
		t.Fatalf("Trust = %q, want untrusted when the home config does not exist", res.Trust)
	}
	if want := filepath.Join(e.codexHome, "config.toml"); filepath.Clean(res.ConfigPath) != want {
		t.Errorf("ConfigPath = %q, want %q", res.ConfigPath, want)
	}
}

func TestCheck_Trust_PathSpelling(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		ci      bool
		want    TrustStatus
	}{
		{"upper-case spelling, case-insensitive", "trusted_upper_case", true, TrustTrusted},
		{"upper-case spelling, case-sensitive", "trusted_upper_case", false, TrustUntrusted},
		{"forward slashes, case-insensitive", "trusted_forward_slashes", true, TrustTrusted},
		{"forward slashes, case-sensitive", "trusted_forward_slashes", false, TrustTrusted},
		{"forward slashes and upper case, case-insensitive", "trusted_slash_upper", true, TrustTrusted},
		{"forward slashes and upper case, case-sensitive", "trusted_slash_upper", false, TrustUntrusted},
		{"trailing separator, case-insensitive", "trusted_trailing_separator", true, TrustTrusted},
		{"trailing separator, case-sensitive", "trusted_trailing_separator", false, TrustTrusted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)

			res := e.checkHome(t, tc.fixture, tc.ci)

			if res.Trust != tc.want {
				t.Fatalf("Trust = %q, want %q", res.Trust, tc.want)
			}
		})
	}
}

func TestCheck_Trust_WorkDirSpellingIsNormalisedToo(t *testing.T) {
	e := newFixtureEnv(t)
	e.installHome(t, "trusted_contact")

	res := e.checker(false).Check(e.workDir + string(filepath.Separator))

	if res.Trust != TrustTrusted {
		t.Fatalf("Trust = %q, want trusted for a workDir with a trailing separator", res.Trust)
	}
}

func TestCheck_ConfigPath_HonoursCodexHome(t *testing.T) {
	e := newFixtureEnv(t)
	want := e.installHome(t, "trusted_contact")

	res := e.checker(false).Check(e.workDir)

	if filepath.Clean(res.ConfigPath) != filepath.Clean(want) {
		t.Fatalf("ConfigPath = %q, want %q", res.ConfigPath, want)
	}
	if res.Trust != TrustTrusted {
		t.Errorf("Trust = %q, want trusted from the CODEX_HOME config", res.Trust)
	}
}

func TestCheck_ConfigPath_FallsBackToHomeDotCodex(t *testing.T) {
	e := newFixtureEnv(t)
	dir := filepath.Join(e.userHome, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(e.render(readFixture(t, "homes", "trusted_contact"))), 0o600); err != nil {
		t.Fatal(err)
	}
	// CODEX_HOME is empty: the fallback must be used. A decoy under the
	// CODEX_HOME directory must be ignored.
	e.installHome(t, "untrusted_level")
	c := NewChecker(
		WithGetenv(func(string) string { return "" }),
		WithUserHomeDir(func() (string, error) { return e.userHome, nil }),
		WithCaseInsensitive(false),
	)

	res := c.Check(e.workDir)

	if filepath.Clean(res.ConfigPath) != filepath.Clean(path) {
		t.Fatalf("ConfigPath = %q, want %q", res.ConfigPath, path)
	}
	if res.Trust != TrustTrusted {
		t.Errorf("Trust = %q, want trusted from <home>/.codex/config.toml", res.Trust)
	}
}

func TestCheck_ConfigPath_CodexHomeWinsOverUserHome(t *testing.T) {
	e := newFixtureEnv(t)
	e.installHome(t, "untrusted_level")
	dir := filepath.Join(e.userHome, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	decoy := e.render(readFixture(t, "homes", "trusted_contact"))
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(decoy), 0o600); err != nil {
		t.Fatal(err)
	}

	res := e.checker(false).Check(e.workDir)

	if res.Trust != TrustUntrusted {
		t.Fatalf("Trust = %q, want untrusted: CODEX_HOME must win when non-empty", res.Trust)
	}
}
