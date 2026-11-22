package app

// opencode_targets.go holds the OpenCode one-target rule. A workspace holds one OpenCode target
// at a time (opencode or opencode-v2); both write identical paths under .opencode/, so only the
// manifest entries decide which target a workspace holds, never the paths. A run that would add
// the second target is refused before anything is asked or written.
//
// Schema-1 manifest migration attributes .opencode/ paths to opencode by path. That only
// concerns manifests that predate schema 2 and is left as it is.

import (
	"errors"
	"fmt"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	openCodeHarnessID   = "opencode"
	openCodeV2HarnessID = "opencode-v2"
)

// ErrOpenCodeOtherTarget is matched when the workspace holds the other OpenCode target.
var ErrOpenCodeOtherTarget = errors.New("workspace already holds the other OpenCode target")

// ErrOpenCodeBothTargets is matched when the workspace holds both OpenCode targets.
var ErrOpenCodeBothTargets = errors.New("workspace holds both OpenCode targets")

// OpenCodeTargetConflictError refuses a run that would put a second OpenCode target into a
// workspace, or that runs on a workspace already holding both.
type OpenCodeTargetConflictError struct {
	Workspace string              // workspace path as the flow resolved it
	Requested domain.HarnessRef   // harness the flow runs for (the target, for a transform)
	Present   []domain.HarnessRef // OpenCode targets found, order opencode, opencode-v2; len 1 or 2
}

// Error returns the refusal message.
func (e *OpenCodeTargetConflictError) Error() string {
	if len(e.Present) >= 2 {
		p0, p1 := e.Present[0], e.Present[1]
		return fmt.Sprintf("workspace %s holds both OpenCode targets, \"%s\" (%s) and \"%s\" (%s); "+
			"nothing was changed. Remove one of them by hand (its agent, skill and plugin files "+
			"in .opencode) and run again",
			e.Workspace, p0.DisplayName, p0.ID, p1.DisplayName, p1.ID)
	}
	var p domain.HarnessRef
	if len(e.Present) == 1 {
		p = e.Present[0]
	}
	r := e.Requested
	return fmt.Sprintf("workspace %s already holds the OpenCode target \"%s\" (%s); "+
		"a workspace holds one OpenCode target at a time, so the run for \"%s\" (%s) was refused "+
		"and nothing was changed. To move the workspace to %s, use transform: "+
		"mosaic-deploy transform --harness %s --target-harness %s --workspace %s",
		e.Workspace, p.DisplayName, p.ID, r.DisplayName, r.ID, r.ID, p.ID, r.ID, e.Workspace)
}

// Is matches ErrOpenCodeOtherTarget when one target is present and ErrOpenCodeBothTargets when
// two are.
func (e *OpenCodeTargetConflictError) Is(target error) bool {
	switch target {
	case ErrOpenCodeOtherTarget:
		return len(e.Present) == 1
	case ErrOpenCodeBothTargets:
		return len(e.Present) == 2
	}
	return false
}

// openCodeTargets is the set of OpenCode targets a workspace holds.
type openCodeTargets struct {
	V1 bool // opencode is present
	V2 bool // opencode-v2 is present
}

// detectOpenCodeTargets classifies the workspace from its manifest snapshot alone. The manifest
// harness_id is the only signal; paths never decide the target.
func detectOpenCodeTargets(snap manifest.Snapshot) openCodeTargets {
	var found openCodeTargets
	if snap.State != manifest.StatePresent {
		return found
	}
	for _, e := range snap.Manifest.Entries {
		switch e.Harness {
		case openCodeHarnessID:
			found.V1 = true
		case openCodeV2HarnessID:
			found.V2 = true
		}
	}
	return found
}

// openCodeTargetsOf returns the OpenCode targets the workspace holds: the detection over its
// manifest, or the fixed set the test-only constructor installed.
func (s *service) openCodeTargetsOf(workspace string) openCodeTargets {
	if s.openCodeHeld != nil {
		return *s.openCodeHeld
	}
	snap := loadReconciledManifest(s.deps.Manifest, workspace).Snapshot
	return detectOpenCodeTargets(snap)
}

// overrideOpenCodeTargets fixes the detected targets for every workspace. Only the test-only
// constructor calls it.
func (s *service) overrideOpenCodeTargets(v1, v2 bool) {
	s.openCodeHeld = &openCodeTargets{V1: v1, V2: v2}
}

// checkOpenCodeTarget returns nil when the run may proceed, or *OpenCodeTargetConflictError. It
// loads the manifest through deps.Manifest and writes nothing.
func (s *service) checkOpenCodeTarget(harnessID, workspace string) error {
	if harnessID != openCodeHarnessID && harnessID != openCodeV2HarnessID {
		return nil
	}
	held := s.openCodeTargetsOf(workspace)

	var ids []string
	if held.V1 {
		ids = append(ids, openCodeHarnessID)
	}
	if held.V2 {
		ids = append(ids, openCodeV2HarnessID)
	}
	switch {
	case len(ids) == 0:
		return nil
	case len(ids) == 1 && ids[0] == harnessID:
		return nil
	}
	present := make([]domain.HarnessRef, 0, len(ids))
	for _, id := range ids {
		present = append(present, s.ghcpHarnessRef(id))
	}
	return &OpenCodeTargetConflictError{
		Workspace: workspace,
		Requested: s.ghcpHarnessRef(harnessID),
		Present:   present,
	}
}

// checkTargetPairs applies the one-target rule of both target pairs, GHCP and OpenCode, to a
// deploy-family run; at most one of them concerns the harness.
func (s *service) checkTargetPairs(harnessID, workspace string) error {
	if err := s.checkGhcpTarget(harnessID, workspace); err != nil {
		return err
	}
	return s.checkOpenCodeTarget(harnessID, workspace)
}
