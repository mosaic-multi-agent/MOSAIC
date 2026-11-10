package app

// stack_service.go declares the stack service interface: the catalog-stack operations a
// frontend uses, embedded in Service, and the helper that puts the stack of a write run into
// the executor request.

import (
	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// StackService exposes catalog-stack configuration to frontends. Every further stack-related
// service method goes on this interface, never on a second one, and its no-op form goes on
// stackservicetest.MockStackService.
type StackService interface {
	// ReadPersistedStack returns the stack persisted in the manifest of workspace; ok is false
	// when none is persisted.
	ReadPersistedStack(workspace string) (stackconfig.Stored, bool, error)
	// ValidatePersistedStack checks that every layer of s still resolves (uses Deps.MosaicRoot).
	ValidatePersistedStack(s stackconfig.Stored) error
	// EncodeStack turns a stack description into the persisted value.
	EncodeStack(d stackconfig.Description) (stackconfig.Stored, error)
	// FindWorkspace returns the workspace holding path, if any.
	FindWorkspace(path string) (string, bool)
	// ActiveStack returns the layers of the catalog in effect (highest first) and the origin of
	// the stack. The origin is never empty.
	ActiveStack() stackconfig.ActiveStack
	// CheckHarnessInjections returns exactly the error of the registry's Resolve for harnessID,
	// nil when Resolve succeeds. A built-in whose injections no layer of the active stack
	// supplies yields an error wrapping *injections.NotSuppliedError.
	CheckHarnessInjections(harnessID string) error
	// CatalogOf returns the layer that supplies item, for the provenance label of selection
	// screens. ok is false for a catalog that is not a stack and for an item the catalog does
	// not contain. Both orchestrator agent keys answer with the layer that supplies the pair.
	CatalogOf(item domain.CatalogItem) (domain.CatalogLayer, bool)
}

func (s *service) ReadPersistedStack(workspace string) (stackconfig.Stored, bool, error) {
	return stackconfig.Read(workspace)
}

func (s *service) ValidatePersistedStack(st stackconfig.Stored) error {
	return stackconfig.Validate(st, s.deps.MosaicRoot)
}

func (s *service) EncodeStack(d stackconfig.Description) (stackconfig.Stored, error) {
	return stackconfig.Encode(d)
}

func (s *service) FindWorkspace(path string) (string, bool) { return stackconfig.FindWorkspace(path) }

// supplyStack puts the encoded stack into req when the service was built from stack input, so
// the executor's manifest save replaces the persisted stack. Without stack input the request
// supplies none and the persisted stack is preserved.
func (s *service) supplyStack(req *deploy.ExecRequest) error {
	if s.effectiveStackOrigin() != stackconfig.OriginStackInput {
		return nil
	}
	stored, err := stackconfig.Encode(s.deps.StackDescription)
	if err != nil {
		return err
	}
	req.ManifestMeta.CatalogStack = stored.Layers
	return nil
}
