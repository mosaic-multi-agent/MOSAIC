package screens_test

// stack_text_test.go verifies that no screen on the entry path describes a single catalogue
// folder replacing the default catalogue, or says that the protocol or bundle always come
// from the MOSAIC root. Checks use key phrases, not exact wording.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/tui/screens"
)

// staleCatalogPhrases are fragments of the retired single-catalogue-folder model.
var staleCatalogPhrases = []string{
	"catalogfilesformat.md",
	"from the mosaic root",
	"is the whole catalog",
	"no default skills or hooks are added",
}

// flatView lower-cases a view and collapses whitespace so checks do not depend on wrapping.
func flatView(view string) string {
	return strings.ToLower(strings.Join(strings.Fields(view), " "))
}

func TestPathScreens_NoPathKindDescribesASingleCatalogueFolder(t *testing.T) {
	// Arrange: every path kind value that has ever been defined, 0 through 3.
	for kind := 0; kind <= 3; kind++ {
		s := screens.NewWorkspaceScreen(80, 24, plainStyles())
		s.SetPathKind(screens.PathKind(kind))

		// Act
		view := flatView(s.View())

		// Assert
		for _, stale := range staleCatalogPhrases {
			if strings.Contains(view, stale) {
				t.Errorf("path kind %d view contains %q (single-catalogue-folder wording):\n%s", kind, stale, view)
			}
		}
	}
}

func TestStackScreen_View_DoesNotDescribeASingleFolderOrMosaicRootSystemFiles(t *testing.T) {
	// Arrange
	s := screens.NewStackScreen(stackconfig.Description{UserLayers: []string{"/some/catalog"}}, 100, 30, plainStyles())

	// Act
	view := flatView(s.View())

	// Assert
	for _, stale := range append([]string{"replaces the default", "instead of the default"}, staleCatalogPhrases...) {
		if strings.Contains(view, stale) {
			t.Errorf("stack screen view contains %q:\n%s", stale, view)
		}
	}
}
