package registry

import "mosaic-deploy/internal/domain"

// BuiltinOptions carries the run context a built-in module needs at construction.
// It is passed to every BuiltinFactory when Discover invokes the registered built-in factories.
//
// Construction shape: a built-in module takes its injection content directory from the ContentDir
// resolver, called with the harness's catalog directory name. The module reads its injection files
// and derives its orchestrator injections version only from the returned directory. Production
// (Discover) supplies a resolver returning <MosaicRoot>/Catalog/HarnessInjections/<harness dir>.
type BuiltinOptions struct {
	// MosaicRoot is the root of the MOSAIC repository. Built-in modules no longer use it to
	// locate their harness content files; ContentDir does that.
	MosaicRoot string
	// ContentDir resolves a built-in harness's injection content directory from its catalog
	// directory name (one of the catalogpaths.HarnessDirName* values). A built-in constructed
	// directly requires it; a nil resolver is a construction error.
	ContentDir func(harnessDirName string) (string, error)
}

// BuiltinFactory constructs a built-in harness module for a run.
// Built-in module packages register a factory by calling Register from their init() function.
// The factory receives BuiltinOptions so it can locate its content files at run time.
//
type BuiltinFactory func(opts BuiltinOptions) (domain.HarnessModule, error)
