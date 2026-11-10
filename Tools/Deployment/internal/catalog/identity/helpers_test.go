package identity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// writeIdentityFile creates a temp catalog root holding catalog-identity.yaml with content
// and returns the root.
func writeIdentityFile(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog-identity.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write identity fixture: %v", err)
	}
	return root
}

// withHeader prefixes the required identity fields to an overrides block.
func withHeader(rest string) string {
	return "id: test-cat\nname: Test Catalog\nversion: \"1.0.0\"\n" + rest
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

func item(kind domain.CatalogItemKind, key string) domain.CatalogItem {
	return domain.CatalogItem{Kind: kind, Key: key}
}

func assertItems(t *testing.T, got, want []domain.CatalogItem) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Items() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Items()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
