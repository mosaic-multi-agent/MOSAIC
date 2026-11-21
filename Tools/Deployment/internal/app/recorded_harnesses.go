package app

// recorded_harnesses.go declares the service method that reports which harnesses the workspace
// manifest at a path records (StackService.RecordedHarnesses).

import "mosaic-deploy/internal/manifest"

// RecordedHarnesses reports the harness IDs of the raw manifest at workspace, sorted ascending.
// The manifest is not reconciled with the disk and no ancestor directory is consulted. Every
// state other than a present manifest, and any load error, reports nil. It never writes.
func (s *service) RecordedHarnesses(workspace string) []string {
	if s.deps.Manifest == nil {
		return nil
	}
	snap, err := s.deps.Manifest.Load(workspace)
	if err != nil || snap.State != manifest.StatePresent {
		return nil
	}
	return snap.Manifest.HarnessIDs()
}
