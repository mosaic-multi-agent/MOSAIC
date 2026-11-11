package main

// manifest_store_wiring.go builds the one manifest store shared by every flow and by the
// executor, with the harness path knowledge taken from the registry's descriptors.

import (
	"strings"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/manifest"
)

// newManifestStore returns the production manifest store for reg. The store knows which
// directories each harness in reg deploys into, so an old-format manifest is upgraded the same
// way wherever it is loaded.
func newManifestStore(reg registry.Registry) manifest.Store {
	return manifest.NewStore(manifest.WithHarnessPaths(harnessPathsOf(reg)))
}

// harnessPathsOf collects, for every harness of reg that can be resolved, the project
// directories its descriptor declares.
func harnessPathsOf(reg registry.Registry) manifest.HarnessPaths {
	out := manifest.HarnessPaths{}
	for _, ref := range reg.List() {
		mod, err := reg.Resolve(ref.ID)
		if err != nil {
			continue
		}
		if desc := mod.Descriptor(); desc != nil {
			out[ref.ID] = projectDirs(desc.Paths)
		}
		_ = mod.Close()
	}
	return out
}

// projectDirs returns the non-empty project directories of paths in slash form without a
// trailing slash.
func projectDirs(paths domain.PathSpec) []string {
	var dirs []string
	for _, p := range []domain.ScopedPaths{paths.Agents, paths.Skills, paths.Hooks} {
		if !p.Supported || p.Project == "" {
			continue
		}
		dirs = append(dirs, strings.TrimRight(strings.ReplaceAll(p.Project, "\\", "/"), "/"))
	}
	return dirs
}
