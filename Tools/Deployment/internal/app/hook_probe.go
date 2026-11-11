package app

// hook_probe.go holds the hook file probes. A hook bundle deploys as files directly in the
// harness hooks directory (flat, or nested under lib/ for some harnesses), and each file is
// probed on its own: its presence, the hash of its real bytes and its Deploy-owned record. A
// hook file carries no version marker, so the manifest entry is the only version source.

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

// probeHookFile reports the on-disk state of one deployed hook file at the workspace-relative
// targetPath: absent when it is not a readable regular file, otherwise present with the hash of
// its bytes and its Deploy-owned record (the whole file). It never returns an error; any I/O
// failure yields absent.
func probeHookFile(workspace, targetPath string) domain.DeployedArtifactState {
	full := filepath.Join(workspace, filepath.FromSlash(targetPath))
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		return domain.DeployedArtifactState{}
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return domain.DeployedArtifactState{}
	}
	state := domain.DeployedArtifactState{Present: true, ContentHash: manifest.Hash(data)}
	if owned, oerr := transform.OwnedContentOf(domain.ArtifactHook, data); oerr == nil {
		state.Owned = &owned
	}
	return state
}

// probeHookPlanPresence reports whether the hook bundle described by hp is deployed: Present is
// true when at least one hp.Files entry exists as a regular file at
// <workspace>/<hp.TargetDir>/<TargetName>. An unsupported plan, a plan without files, and a
// plan whose files are all missing are absent.
func probeHookPlanPresence(workspace string, hp domain.HookPlan) domain.DeployedArtifactState {
	if !hp.Supported {
		return domain.DeployedArtifactState{}
	}
	for _, f := range hp.Files {
		info, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(hp.TargetDir), filepath.FromSlash(f.TargetName)))
		if err == nil && info.Mode().IsRegular() {
			return domain.DeployedArtifactState{Present: true}
		}
	}
	return domain.DeployedArtifactState{}
}
