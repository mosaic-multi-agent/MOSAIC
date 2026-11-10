package registry

import (
	"errors"
	"fmt"
	"path/filepath"

	"mosaic-deploy/internal/catalog/catalogpaths"
)

// MosaicRootContentDirs returns a content-directory resolver that maps a harness catalog
// directory name to <mosaicRoot>/Catalog/HarnessInjections/<harness dir>.
func MosaicRootContentDirs(mosaicRoot string) func(harnessDirName string) (string, error) {
	return func(harnessDirName string) (string, error) {
		return filepath.Join(mosaicRoot, filepath.FromSlash(catalogpaths.MosaicRelHarnessInjectionsDir), harnessDirName), nil
	}
}

// builtinContentDirs returns the resolver handed to built-in factories: the BuiltinContentDir
// override when set, otherwise the MOSAIC-root layout.
func (o Options) builtinContentDirs() func(harnessDirName string) (string, error) {
	if o.BuiltinContentDir != nil {
		return o.BuiltinContentDir
	}
	return MosaicRootContentDirs(o.MosaicRoot)
}

// ResolveContentDir returns the injection content directory for the harness with the given
// catalog directory name. A nil ContentDir resolver is an error, and a resolver error is
// wrapped so callers can match it with errors.Is.
func (o BuiltinOptions) ResolveContentDir(harnessDirName string) (string, error) {
	if o.ContentDir == nil {
		return "", errors.New("no content directory resolver supplied")
	}
	dir, err := o.ContentDir(harnessDirName)
	if err != nil {
		return "", fmt.Errorf("resolve content directory for %q: %w", harnessDirName, err)
	}
	return dir, nil
}
