package codexsetup

import "testing"

func TestCheck_ContactUser_HomeConfigRows(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		want    ServerStatus
	}{
		{"server configured", "trusted_contact", ServerConfigured},
		{"server explicitly enabled", "contact_explicitly_enabled", ServerConfigured},
		{"configured while the workspace is untrusted", "untrusted_level", ServerConfigured},
		{"only another server configured", "trusted_no_contact", ServerMissing},
		{"server disabled", "contact_disabled", ServerMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)

			res := e.checkHome(t, tc.fixture, false)

			if res.ContactUser != tc.want {
				t.Fatalf("ContactUser = %q, want %q", res.ContactUser, tc.want)
			}
			if res.ContactUserReason != "" {
				t.Errorf("ContactUserReason = %q, want empty for a decided check", res.ContactUserReason)
			}
		})
	}
}

func TestCheck_ContactUser_MissingHomeConfigIsMissing(t *testing.T) {
	e := newFixtureEnv(t)

	res := e.checker(false).Check(e.workDir)

	if res.ContactUser != ServerMissing {
		t.Fatalf("ContactUser = %q, want missing when the home config does not exist", res.ContactUser)
	}
}

func TestCheck_ContactUser_TrustedWorkspaceUsesProjectConfig(t *testing.T) {
	cases := []struct {
		name    string
		project string
		want    ServerStatus
	}{
		{"server in project config", "contact_user", ServerConfigured},
		{"project server disabled", "contact_disabled", ServerMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)
			e.installProject(t, tc.project)

			res := e.checkHome(t, "trusted_no_contact", false)

			if res.ContactUser != tc.want {
				t.Fatalf("ContactUser = %q, want %q", res.ContactUser, tc.want)
			}
		})
	}
}

func TestCheck_ContactUser_HomeAndProjectEitherSourceSuffices(t *testing.T) {
	cases := []struct {
		name    string
		home    string
		project string
		want    ServerStatus
	}{
		{"home disabled, project enabled", "contact_disabled", "contact_user", ServerConfigured},
		{"home enabled, project disabled", "trusted_contact", "contact_disabled", ServerConfigured},
		{"both disabled", "contact_disabled", "contact_disabled", ServerMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newFixtureEnv(t)
			e.installProject(t, tc.project)

			res := e.checkHome(t, tc.home, false)

			if res.Trust != TrustTrusted {
				t.Fatalf("Trust = %q, want trusted (precondition of consulting the project config)", res.Trust)
			}
			if res.ContactUser != tc.want {
				t.Fatalf("ContactUser = %q, want %q", res.ContactUser, tc.want)
			}
		})
	}
}

func TestCheck_ContactUser_TrustedAncestorAppliesProjectConfig(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "contact_user")

	res := e.checkHome(t, "parent_trusted_no_contact", false)

	if res.Trust != TrustTrusted || res.ContactUser != ServerConfigured {
		t.Fatalf("got trust=%q contact=%q, want trusted and configured: ancestor trust covers the project config",
			res.Trust, res.ContactUser)
	}
}

func TestCheck_ContactUser_EntryThatIsNotATableIsNeverConfigured(t *testing.T) {
	e := newFixtureEnv(t)

	res := e.checkHome(t, "contact_not_a_table", false)

	if res.Trust != TrustTrusted {
		t.Fatalf("Trust = %q, want trusted (the entry is well-formed TOML)", res.Trust)
	}
	switch res.ContactUser {
	case ServerMissing:
	case ServerIndeterminate:
		if res.ContactUserReason == "" {
			t.Error("an indeterminate result must carry a reason")
		}
	default:
		t.Fatalf("ContactUser = %q, want missing or indeterminate for a non-table contact-user entry", res.ContactUser)
	}
}

func TestCheck_ContactUser_UntrustedWorkspaceIgnoresProjectConfig(t *testing.T) {
	e := newFixtureEnv(t)
	e.installProject(t, "contact_user")

	res := e.checkHome(t, "untrusted_no_contact", false)

	if res.Trust != TrustUntrusted {
		t.Fatalf("Trust = %q, want untrusted", res.Trust)
	}
	if res.ContactUser != ServerMissing {
		t.Fatalf("ContactUser = %q, want missing: an untrusted project config is not applied", res.ContactUser)
	}
}

func TestCheck_ContactUser_TrustedWithoutProjectConfigStaysMissing(t *testing.T) {
	e := newFixtureEnv(t)

	res := e.checkHome(t, "trusted_no_contact", false)

	if res.Trust != TrustTrusted || res.ContactUser != ServerMissing {
		t.Fatalf("got trust=%q contact=%q, want trusted and missing", res.Trust, res.ContactUser)
	}
}
