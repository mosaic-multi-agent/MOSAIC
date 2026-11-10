package registry

// stack_discover.go builds and rebuilds the built-in harness modules from the catalog stack in
// effect for a run.
//
// Rebuild mechanism: the stack's injections result decides, per harness, which layer's
// HarnessInjections directory a built-in reads. DiscoverForStack runs Discover with a content
// directory resolver backed by that result and remembers the options. When the stack changes
// (the TUI catalogue reload), Rebuild re-runs only the built-in factories against the new
// stack's result and returns a new Registry; descriptor-only and external entries are carried
// over unchanged. No state is shared between the old and the new registry, so a service
// holding the old one keeps its own stack's content.
//
// A harness whose injections no layer supplies still gets a listed entry that is not usable;
// resolving it returns an error that wraps the *injections.NotSuppliedError naming the item.

import (
	"errors"
	"fmt"
	"sort"

	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/injectionfile"
)

// Rebuilder is implemented by every Registry produced by Discover.
type Rebuilder interface {
	// RebuildForStack returns a registry whose built-in modules read their injection content
	// from the layers res resolved.
	RebuildForStack(res injections.Result) (Registry, error)
}

// StackContentDirs returns a content-directory resolver backed by a stack's injections result.
// A harness no layer supplies yields a *injections.NotSuppliedError.
func StackContentDirs(res injections.Result) func(harnessDirName string) (string, error) {
	return func(harnessDirName string) (string, error) {
		r := res.Lookup(harnessDirName)
		if err := r.Err(); err != nil {
			return "", err
		}
		return r.Dir, nil
	}
}

// DiscoverForStack is Discover with the built-in injection content taken from res. Any
// BuiltinContentDir already set in opts is replaced. Every Registry from Discover implements Rebuilder.
func DiscoverForStack(opts Options, res injections.Result) (Registry, error) {
	opts.BuiltinContentDir = StackContentDirs(res)
	opts.injectionLookup = res.Lookup
	return Discover(opts)
}

// RebuildForStack implements Rebuilder.
func (r *registryImpl) RebuildForStack(res injections.Result) (Registry, error) {
	if r.opts == nil {
		return nil, fmt.Errorf("registry carries no discovery options")
	}
	opts := *r.opts
	opts.BuiltinContentDir = StackContentDirs(res)
	opts.injectionLookup = res.Lookup
	builtinMu.RLock()
	factories := make(map[string]BuiltinFactory, len(builtinFactories))
	for id, f := range builtinFactories {
		factories[id] = f
	}
	builtinMu.RUnlock()

	builtinOpts := BuiltinOptions{MosaicRoot: opts.MosaicRoot, ContentDir: opts.BuiltinContentDir}
	entries := make(map[string]harnessEntry, len(r.entries))
	for id, e := range r.entries {
		entries[id] = e
	}
	for id, factory := range factories {
		if old, ok := entries[id]; ok && old.ref.Tier == domain.TierBuiltin {
			entries[id] = constructBuiltin(id, factory, builtinOpts, opts)
		}
	}
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	refs := make([]domain.HarnessRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, entries[id].ref)
	}
	return &registryImpl{allowExternal: r.allowExternal, entries: entries, refs: refs, opts: &opts}, nil
}

// constructBuiltin runs one built-in factory and applies the ToolMappings then the ModelCatalog
// hook to the module. A factory failure yields a listed entry that is not usable and keeps the
// cause for Resolve.
func constructBuiltin(id string, factory BuiltinFactory, builtinOpts BuiltinOptions, opts Options) harnessEntry {
	builtinOpts, askedDir := recordAskedDir(builtinOpts)
	mod, err := factory(builtinOpts)
	if err != nil {
		err = attributeInjectionLoadError(err, *askedDir, opts)
		return harnessEntry{
			ref: domain.HarnessRef{
				ID:             id,
				Tier:           domain.TierBuiltin,
				Usable:         false,
				UnusableReason: fmt.Sprintf("failed to initialise built-in: %v", err),
			},
			err:          err,
			injectionDir: *askedDir,
		}
	}
	// The hooks wrap the module with descriptor copies so the original descriptor is not mutated.
	wrapped := applyToolMappingsHook(id, mod, opts.ToolMappings)
	wrapped = applyModelCatalogHook(id, wrapped, opts.ModelCatalog)
	return harnessEntry{ref: wrapped.Ref(), mod: wrapped, injectionDir: *askedDir}
}

// attributeInjectionLoadError wraps a factory error carrying an injection file load failure with
// the layer supplying the harness directory the built-in asked for. Without a stack-backed
// resolver, a layer-less or unrelated error, err is returned unchanged.
func attributeInjectionLoadError(err error, askedDir string, opts Options) error {
	var loadErr *injectionfile.LoadError
	if opts.injectionLookup == nil || askedDir == "" || !errors.As(err, &loadErr) {
		return err
	}
	res := opts.injectionLookup(askedDir)
	if !res.Supplied {
		return err
	}
	return &InjectionLoadError{Resolution: res, Err: err}
}

// unusableError is the Resolve error for an entry with no module. A built-in construction
// failure is wrapped so errors.Is and errors.As reach its cause.
func (e harnessEntry) unusableError(id string) error {
	if e.err != nil {
		return fmt.Errorf("harness %q is not usable: failed to initialise built-in: %w", id, e.err)
	}
	return fmt.Errorf("harness %q is not usable: %s", id, e.ref.UnusableReason)
}
