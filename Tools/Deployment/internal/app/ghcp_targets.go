package app

// ghcp_targets.go holds the GHCP target detection and the deploy-flow refusal contract.
// A workspace holds one GHCP target at a time; the two targets share folders and files, so a
// run that would add the second one is refused before anything is asked or written.

import (
	"errors"
	"fmt"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	ghcpCLIHarnessID    = "ghcp-cli"
	ghcpVSCodeHarnessID = "vscode-ghcp"
)

// ErrGhcpOtherTarget is matched when the workspace holds the other GHCP target.
var ErrGhcpOtherTarget = errors.New("workspace already holds the other GHCP target")

// ErrGhcpBothTargets is matched when the workspace holds both GHCP targets.
var ErrGhcpBothTargets = errors.New("workspace holds both GHCP targets")

// GhcpTargetConflictError refuses a run that would put a second GHCP target into a workspace,
// or that runs on a workspace already holding both.
type GhcpTargetConflictError struct {
	Workspace string              // workspace path as the flow resolved it
	Requested domain.HarnessRef   // harness the flow runs for
	Present   []domain.HarnessRef // GHCP targets found, order ghcp-cli, vscode-ghcp; len 1 or 2
}

// Error returns the refusal message.
func (e *GhcpTargetConflictError) Error() string {
	if len(e.Present) >= 2 {
		p0, p1 := e.Present[0], e.Present[1]
		return fmt.Sprintf("workspace %s holds both GHCP targets, \"%s\" (%s) and \"%s\" (%s); "+
			"nothing was changed. Remove one of them by hand (its agent files in .github/agents "+
			"and its hook files in .github/hooks) and run again",
			e.Workspace, p0.DisplayName, p0.ID, p1.DisplayName, p1.ID)
	}
	var p domain.HarnessRef
	if len(e.Present) == 1 {
		p = e.Present[0]
	}
	r := e.Requested
	return fmt.Sprintf("workspace %s already holds the GHCP target \"%s\" (%s); "+
		"a workspace holds one GHCP target at a time, so the run for \"%s\" (%s) was refused "+
		"and nothing was changed. To move the workspace to %s, use transform: "+
		"mosaic-deploy transform --harness %s --target-harness %s --workspace %s",
		e.Workspace, p.DisplayName, p.ID, r.DisplayName, r.ID, r.ID, p.ID, r.ID, e.Workspace)
}

// Is matches ErrGhcpOtherTarget when one target is present and ErrGhcpBothTargets when two are.
func (e *GhcpTargetConflictError) Is(target error) bool {
	switch target {
	case ErrGhcpOtherTarget:
		return len(e.Present) == 1
	case ErrGhcpBothTargets:
		return len(e.Present) == 2
	}
	return false
}

// ghcpTargets is the set of GHCP targets a workspace holds.
type ghcpTargets struct {
	CLI    bool // ghcp-cli is present
	VSCode bool // vscode-ghcp is present
}

// detectGhcpTargets classifies the workspace from its manifest snapshot alone. The manifest
// harness_id is the only signal; agent file names never decide the target.
func detectGhcpTargets(snap manifest.Snapshot) ghcpTargets {
	var found ghcpTargets
	if snap.State != manifest.StatePresent {
		return found
	}
	switch snap.Manifest.HarnessID {
	case ghcpCLIHarnessID:
		found.CLI = true
	case ghcpVSCodeHarnessID:
		found.VSCode = true
	}
	return found
}

// ghcpTargetsOf returns the GHCP targets the workspace holds: the detection over its manifest,
// or the fixed set the test-only constructor installed.
func (s *service) ghcpTargetsOf(workspace string) ghcpTargets {
	if s.ghcpHeld != nil {
		return *s.ghcpHeld
	}
	snap, _ := s.deps.Manifest.Load(workspace)
	return detectGhcpTargets(snap)
}

// overrideGhcpTargets fixes the detected targets for every workspace. Only the test-only
// constructor calls it.
func (s *service) overrideGhcpTargets(held ghcpTargets) {
	s.ghcpHeld = &held
}

// checkGhcpTarget returns nil when the run may proceed, or *GhcpTargetConflictError. It loads
// the manifest through deps.Manifest and writes nothing.
func (s *service) checkGhcpTarget(harnessID, workspace string) error {
	if harnessID != ghcpCLIHarnessID && harnessID != ghcpVSCodeHarnessID {
		return nil
	}
	held := s.ghcpTargetsOf(workspace)

	var ids []string
	if held.CLI {
		ids = append(ids, ghcpCLIHarnessID)
	}
	if held.VSCode {
		ids = append(ids, ghcpVSCodeHarnessID)
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
	return &GhcpTargetConflictError{
		Workspace: workspace,
		Requested: s.ghcpHarnessRef(harnessID),
		Present:   present,
	}
}

// ghcpHarnessRef returns the registry ref for id, or a ref named by its ID when it is not listed.
func (s *service) ghcpHarnessRef(id string) domain.HarnessRef {
	for _, h := range s.deps.Registry.List() {
		if h.ID == id {
			return h
		}
	}
	return domain.HarnessRef{ID: id, DisplayName: id}
}
