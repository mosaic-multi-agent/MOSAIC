// Package stackservicetest provides MockStackService, a no-op implementation of
// app.StackService for embedding in app.Service test doubles. Imported by tests only.
package stackservicetest

import (
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/domain"
)

// MockStackService implements every app.StackService method with a no-op result. The zero
// value is ready to use; doubles embed it by value. A double's own method of the same name
// wins over the embedded one (Go promotion rules).
type MockStackService struct{}

// ReadPersistedStack reports no persisted stack.
func (MockStackService) ReadPersistedStack(workspace string) (stackconfig.Stored, bool, error) {
	return stackconfig.Stored{}, false, nil
}

// ValidatePersistedStack accepts any stack.
func (MockStackService) ValidatePersistedStack(s stackconfig.Stored) error { return nil }

// EncodeStack returns the zero Stored.
func (MockStackService) EncodeStack(d stackconfig.Description) (stackconfig.Stored, error) {
	return stackconfig.Stored{}, nil
}

// FindWorkspace finds no workspace.
func (MockStackService) FindWorkspace(path string) (string, bool) { return "", false }

// ActiveStack reports the zero stack (no layers, empty origin).
func (MockStackService) ActiveStack() stackconfig.ActiveStack { return stackconfig.ActiveStack{} }

// CheckHarnessInjections accepts any harness.
func (MockStackService) CheckHarnessInjections(harnessID string) error { return nil }

// CatalogOf reports every item as not found.
func (MockStackService) CatalogOf(item domain.CatalogItem) (domain.CatalogLayer, bool) {
	return domain.CatalogLayer{}, false
}
