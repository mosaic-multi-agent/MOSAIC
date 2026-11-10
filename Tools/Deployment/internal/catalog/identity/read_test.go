package identity_test

import (
	"errors"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog/identity"
)

// The version scalar is kept as its source text, never re-formatted as a number.
func TestRead_VersionKeepsSourceText(t *testing.T) {
	cases := []struct{ name, yamlValue, want string }{
		{"unquoted one point zero", "1.0", "1.0"},
		{"unquoted trailing zero", "1.10", "1.10"},
		{"quoted semver", `"0.7.0"`, "0.7.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeIdentityFile(t, "id: test-cat\nname: Test\nversion: "+tc.yamlValue+"\n")

			got, err := identity.Read(root)

			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if got.Version != tc.want {
				t.Fatalf("Version = %q, want %q", got.Version, tc.want)
			}
		})
	}
}

func TestRead_ValidIdentityYieldsFields(t *testing.T) {
	root := writeIdentityFile(t, "id: my-cat.v2_x\nname: My Catalog\nversion: \"2.3.4\"\n")

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.ID != "my-cat.v2_x" || got.Name != "My Catalog" || got.Version != "2.3.4" {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if n := len(got.Overrides.Items()); n != 0 {
		t.Fatalf("expected no declared overrides, got %d", n)
	}
}

func TestRead_UnknownTopLevelKeyIsIgnored(t *testing.T) {
	root := writeIdentityFile(t, withHeader("future_key: whatever\n"))

	got, err := identity.Read(root)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.ID != "test-cat" {
		t.Fatalf("ID = %q", got.ID)
	}
}

func TestRead_MissingIdentityFile(t *testing.T) {
	root := t.TempDir()

	_, err := identity.Read(root)

	if !errors.Is(err, identity.ErrIdentityMissing) {
		t.Fatalf("expected ErrIdentityMissing, got %v", err)
	}
	assertContains(t, err.Error(), root)
	assertContains(t, err.Error(), "catalog-identity.yaml")
}

func TestRead_MissingRootDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := identity.Read(root)

	if !errors.Is(err, identity.ErrIdentityMissing) {
		t.Fatalf("expected ErrIdentityMissing, got %v", err)
	}
	assertContains(t, err.Error(), root)
}

func TestRead_RequiredFieldMissingOrEmpty(t *testing.T) {
	full := map[string]string{"id": "test-cat", "name": "Test", "version": `"1.0"`}
	variants := map[string]func(m map[string]string, f string){
		"absent": func(m map[string]string, f string) { delete(m, f) },
		"null":   func(m map[string]string, f string) { m[f] = "~" },
		"empty":  func(m map[string]string, f string) { m[f] = `""` },
	}
	for _, field := range []string{"id", "name", "version"} {
		for vname, mutate := range variants {
			t.Run(field+" "+vname, func(t *testing.T) {
				m := map[string]string{}
				for k, v := range full {
					m[k] = v
				}
				mutate(m, field)
				root := writeIdentityFile(t, renderFields(m))

				_, err := identity.Read(root)

				if !errors.Is(err, identity.ErrIdentityFieldMissing) {
					t.Fatalf("expected ErrIdentityFieldMissing, got %v", err)
				}
				assertContains(t, err.Error(), field)
				assertContains(t, err.Error(), root)
			})
		}
	}
}

func TestRead_InvalidValues(t *testing.T) {
	cases := map[string]string{
		"id uppercase":        "id: Bad\nname: T\nversion: \"1\"\n",
		"id leading dash":     "id: -bad\nname: T\nversion: \"1\"\n",
		"id with space":       "id: \"a b\"\nname: T\nversion: \"1\"\n",
		"version is a list":   "id: a\nname: T\nversion: [1, 2]\n",
		"name is a map":       "id: a\nname: {x: y}\nversion: \"1\"\n",
		"malformed yaml":      "id: a\n  name: : [unclosed\n",
		"top level not a map": "- just\n- a list\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeIdentityFile(t, content)

			_, err := identity.Read(root)

			if !errors.Is(err, identity.ErrIdentityInvalid) {
				t.Fatalf("expected ErrIdentityInvalid, got %v", err)
			}
			assertContains(t, err.Error(), root)
		})
	}
}

func renderFields(m map[string]string) string {
	s := ""
	for _, k := range []string{"id", "name", "version"} {
		if v, ok := m[k]; ok {
			s += k + ": " + v + "\n"
		}
	}
	return s
}
