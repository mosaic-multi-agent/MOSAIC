package screens_test

// stack_identities_test.go verifies what the catalog-stack screen shows for each row: the layer
// identities the caller supplies (id, name, version, then the path), the path alone where no
// identity is known, the default row marker, and that the screen text never describes a single
// catalog folder replacing the default or says the protocol or bundle always come from the
// MOSAIC root.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// rowContaining returns the view line holding needle, failing when there is none.
func rowContaining(t *testing.T, view, needle string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no row contains %q; view:\n%s", needle, view)
	return ""
}

// requireInOrder fails unless every part occurs in row, left to right.
func requireInOrder(t *testing.T, row string, parts ...string) {
	t.Helper()
	at := 0
	for _, p := range parts {
		i := strings.Index(row[at:], p)
		if i < 0 {
			t.Errorf("row %q: want %q after position %d, parts in order %v", row, p, at, parts)
			return
		}
		at += i + len(p)
	}
}

func TestStackScreen_FullIdentity_ShowsIdNameVersionThenPath(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	s.SetIdentities(map[string]domain.CatalogIdentity{
		d[0]: {ID: "team-cat", Name: "Team Catalog", Version: "2.1.0"},
	}, domain.CatalogIdentity{})

	requireInOrder(t, rowContaining(t, s.View(), d[0]), "team-cat", "Team Catalog", "2.1.0", d[0])
}

func TestStackScreen_IdOnlyIdentity_ShowsTheIdAndThePath(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	s.SetIdentities(map[string]domain.CatalogIdentity{d[0]: {ID: "stored-id"}}, domain.CatalogIdentity{})

	requireInOrder(t, rowContaining(t, s.View(), d[0]), "stored-id", d[0])
}

func TestStackScreen_NoIdentityEntry_ShowsThePathOnly(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	s.SetIdentities(map[string]domain.CatalogIdentity{d[0]: {ID: "known-id", Name: "Known"}}, domain.CatalogIdentity{})

	row := rowContaining(t, s.View(), d[1])
	if strings.Contains(row, "known-id") || strings.Contains(row, "Known") {
		t.Errorf("a row without an identity entry must show its path only; got %q", row)
	}
}

func TestStackScreen_EntryWithEmptyId_ShowsThePathOnly(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	s.SetIdentities(map[string]domain.CatalogIdentity{d[0]: {Name: "Nameless Id", Version: "9.9.9"}}, domain.CatalogIdentity{})

	row := rowContaining(t, s.View(), d[0])
	if strings.Contains(row, "Nameless Id") || strings.Contains(row, "9.9.9") {
		t.Errorf("an identity without an id is not shown; got %q", row)
	}
}

func TestStackScreen_DefaultRow_AlwaysCarriesTheMarkerAndShowsAKnownIdentity(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	requireInOrder(t, rowContaining(t, s.View(), "(default)"), "(default)")
	s.SetIdentities(nil, domain.CatalogIdentity{ID: "mosaic-default", Name: "MOSAIC Default", Version: "1.4.0"})

	requireInOrder(t, rowContaining(t, s.View(), "(default)"), "mosaic-default", "MOSAIC Default", "1.4.0")
}

func TestStackScreen_DefaultRowWithZeroIdentity_ShowsTheMarkerOnly(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: false})

	s.SetIdentities(nil, domain.CatalogIdentity{Name: "Orphan Name", Version: "3.0.0"})

	row := rowContaining(t, s.View(), "(default)")
	if strings.Contains(row, "Orphan Name") || strings.Contains(row, "3.0.0") {
		t.Errorf("a default identity without an id adds nothing to the row; got %q", row)
	}
}

func TestStackScreen_SetIdentities_ReplacesThePreviousIdentitiesWhole(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})
	s.SetIdentities(map[string]domain.CatalogIdentity{d[0]: {ID: "first-id"}},
		domain.CatalogIdentity{ID: "first-default"})
	if view := s.View(); !strings.Contains(view, "first-id") || !strings.Contains(view, "first-default") {
		t.Fatalf("the first identities must be shown; got:\n%s", view)
	}

	s.SetIdentities(nil, domain.CatalogIdentity{})

	if view := s.View(); strings.Contains(view, "first-id") || strings.Contains(view, "first-default") {
		t.Errorf("the second SetIdentities call must replace the first whole; got:\n%s", view)
	}
}

func TestStackScreen_Reset_KeepsTheIdentitiesUntilTheNextSetIdentities(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d[:1], IncludeDefault: true})
	s.SetIdentities(map[string]domain.CatalogIdentity{d[1]: {ID: "kept-id", Name: "Kept"}}, domain.CatalogIdentity{})

	s.Reset(stackconfig.Description{UserLayers: d[1:], IncludeDefault: true})

	requireInOrder(t, rowContaining(t, s.View(), d[1]), "kept-id", "Kept", d[1])
}

func TestStackScreen_Text_NeverDescribesAReplacingFolderOrMosaicRootSystemFiles(t *testing.T) {
	d := catalogDirs(t, 1)
	starts := []stackconfig.Description{
		{IncludeDefault: true},
		{UserLayers: d, IncludeDefault: true},
		{UserLayers: d, IncludeDefault: false},
	}
	banned := []string{
		"always come from the mosaic root",
		"whole catalog",
		"replaces the default",
		"replace the default",
		"no default skills",
	}
	for _, start := range starts {
		view := strings.ToLower(collapseWhitespace(newTestStackScreen(start).View()))
		if !strings.Contains(view, "(default)") {
			t.Fatalf("view for %+v is not a rendered stack screen; got %q", start, view)
		}
		for _, b := range banned {
			if strings.Contains(view, b) {
				t.Errorf("view for %+v contains %q; the screen must not say that", start, b)
			}
		}
	}
}
