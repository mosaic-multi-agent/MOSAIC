package codexsetup

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheck_HomeUnresolvable_BothChecksIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	c := NewChecker(
		WithGetenv(func(string) string { return "" }),
		WithUserHomeDir(func() (string, error) { return "", errors.New("no home") }),
		WithCaseInsensitive(false),
	)

	res := c.Check(e.workDir)

	if res.Trust != TrustIndeterminate || res.TrustReason == "" {
		t.Errorf("Trust = %q reason %q, want indeterminate with a reason", res.Trust, res.TrustReason)
	}
	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Errorf("ContactUser = %q reason %q, want indeterminate with a reason", res.ContactUser, res.ContactUserReason)
	}
	if res.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty when the location cannot be resolved", res.ConfigPath)
	}
}

func TestCheck_MalformedHomeConfig_BothChecksIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)

	res := e.checkHome(t, "malformed", false)

	if res.Trust != TrustIndeterminate || res.TrustReason == "" {
		t.Errorf("Trust = %q reason %q, want indeterminate with a reason", res.Trust, res.TrustReason)
	}
	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Errorf("ContactUser = %q reason %q, want indeterminate with a reason", res.ContactUser, res.ContactUserReason)
	}
	if res.ConfigPath == "" {
		t.Error("ConfigPath is empty, want the config that could not be parsed")
	}
}

func TestCheck_UnreadableHomeConfig_BothChecksIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	// A directory where config.toml should be: it exists but cannot be read.
	if err := os.Mkdir(filepath.Join(e.codexHome, "config.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := e.checker(false).Check(e.workDir)

	if res.Trust != TrustIndeterminate || res.TrustReason == "" {
		t.Errorf("Trust = %q reason %q, want indeterminate with a reason", res.Trust, res.TrustReason)
	}
	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Errorf("ContactUser = %q reason %q, want indeterminate with a reason", res.ContactUser, res.ContactUserReason)
	}
}

func TestCheck_TrustedWithMalformedProjectConfig_ContactUserIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "malformed")

	res := e.checkHome(t, "trusted_no_contact", false)

	if res.Trust != TrustTrusted {
		t.Errorf("Trust = %q, want trusted (the home config is fine)", res.Trust)
	}
	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Errorf("ContactUser = %q reason %q, want indeterminate with a reason", res.ContactUser, res.ContactUserReason)
	}
	if res.TrustReason != "" {
		t.Errorf("TrustReason = %q, want empty for a decided trust check", res.TrustReason)
	}
}

func TestCheck_TrustedWithUnreadableProjectConfig_ContactUserIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	if err := os.MkdirAll(filepath.Join(e.workDir, ".codex", "config.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := e.checkHome(t, "trusted_no_contact", false)

	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Errorf("ContactUser = %q reason %q, want indeterminate with a reason", res.ContactUser, res.ContactUserReason)
	}
}

func TestCheck_UntrustedWithMalformedProjectConfig_ProjectConfigNotConsulted(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "malformed")

	res := e.checkHome(t, "untrusted_no_contact", false)

	if res.ContactUser != ServerMissing {
		t.Fatalf("ContactUser = %q, want missing: an untrusted project config is never consulted", res.ContactUser)
	}
}

func TestCheck_IndeterminateTrustWithProjectConfig_ContactUserStaysIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "contact_user")

	res := e.checkHome(t, "malformed", false)

	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Fatalf("ContactUser = %q reason %q, want indeterminate: with undecidable trust the project config is not consulted and the result is never a pass",
			res.ContactUser, res.ContactUserReason)
	}
}

func TestCheck_UnreadableHomeWithProjectConfig_ContactUserStaysIndeterminate(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "contact_user")
	if err := os.Mkdir(filepath.Join(e.codexHome, "config.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	res := e.checker(false).Check(e.workDir)

	if res.ContactUser != ServerIndeterminate || res.ContactUserReason == "" {
		t.Fatalf("ContactUser = %q reason %q, want indeterminate", res.ContactUser, res.ContactUserReason)
	}
}

func TestCheck_NeverModifiesAnyConfiguration(t *testing.T) {
	cases := []struct {
		name    string
		fixture string // "" = no home config at all
		project string
	}{
		{"trusted with project config", "trusted_no_contact", "contact_user"},
		{"untrusted", "untrusted_level", ""},
		{"malformed home config", "malformed", ""},
		{"no home config", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)
			if tc.fixture != "" {
				e.installHome(t, tc.fixture)
			}
			if tc.project != "" {
				e.installProject(t, tc.project)
			}
			before := snapshotTree(t, e.root)

			_ = e.checker(false).Check(e.workDir)

			after := snapshotTree(t, e.root)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("Check changed the file tree:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}
