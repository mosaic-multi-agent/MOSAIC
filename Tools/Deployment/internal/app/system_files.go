package app

// system_files.go resolves the catalog root the protocol and the deployed-sections bundle are
// loaded from. When the service's catalog is a stack, each file comes whole from the layer the
// stack resolved, and a stack that supplies neither is a run-stopping error that names the file.
// A catalog that is not a stack has a single root, which supplies both files.

import (
	"mosaic-deploy/internal/catalog/stack"
	"mosaic-deploy/internal/catalog/stack/systemfile"
)

// protocolCatalogRoot returns the root of the catalog layer that supplies the protocol.
func (s *service) protocolCatalogRoot() (string, error) {
	return s.systemFileRoot(func(r systemfile.Result) systemfile.Resolution { return r.Protocol })
}

// bundleCatalogRoot returns the root of the catalog layer that supplies the bundle.
func (s *service) bundleCatalogRoot() (string, error) {
	return s.systemFileRoot(func(r systemfile.Result) systemfile.Resolution { return r.Bundle })
}

func (s *service) systemFileRoot(pick func(systemfile.Result) systemfile.Resolution) (string, error) {
	st, ok := s.deps.Catalog.(stack.Catalog)
	if !ok {
		return s.deps.Catalog.CatalogRoot(), nil
	}
	res := pick(st.SystemFiles())
	if err := res.Err(); err != nil {
		return "", err
	}
	return res.Supplier.Root, nil
}
