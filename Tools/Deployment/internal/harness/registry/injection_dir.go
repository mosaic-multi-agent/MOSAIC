package registry

import "mosaic-deploy/internal/domain"

// InjectionDirReporter is implemented by every Registry produced by Discover, DiscoverForStack
// and RebuildForStack. It is optional for other Registry implementations.
type InjectionDirReporter interface {
	// InjectionDirName returns the HarnessInjections directory name (a catalogpaths.HarnessDirName*
	// value, e.g. "OpenCode") that the built-in module with the given harness id passed to its
	// content-directory resolver when it was constructed, whether or not construction succeeded.
	// ok is false for an unknown id, for an id whose entry is descriptor-only or external (also
	// when such an entry overrides a built-in of the same id), and for a built-in that asked for
	// no directory.
	InjectionDirName(harnessID string) (dirName string, ok bool)
}

// InjectionDirName implements InjectionDirReporter.
func (r *registryImpl) InjectionDirName(harnessID string) (string, bool) {
	e, found := r.entries[harnessID]
	if !found || e.ref.Tier != domain.TierBuiltin || e.injectionDir == "" {
		return "", false
	}
	return e.injectionDir, true
}

// recordAskedDir returns opts with its content resolver wrapped to remember the last directory
// name asked for, plus a pointer to that name (empty until the factory asks).
func recordAskedDir(opts BuiltinOptions) (BuiltinOptions, *string) {
	asked := new(string)
	inner := opts.ContentDir
	if inner == nil {
		return opts, asked
	}
	opts.ContentDir = func(harnessDirName string) (string, error) {
		*asked = harnessDirName
		return inner(harnessDirName)
	}
	return opts, asked
}
