package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/domain"
)

const orchestratorStem = "orchestrator-script"

// DiscoverOrchestrator computes the expected path to the script-mode
// orchestrator agent file from the harness convention and verifies the file
// exists.
//
// The path is {workDir}/{agentsDir}/orchestrator-script{ext} for an extension
// the harness's agent files accept (for Markdown harnesses .md or .agent.md).
//
// Returns the absolute path on success.
// Returns *domain.RefusalError with Component "harness" if:
//   - harnessID has no agent format data (unknown identity)
//   - no candidate file exists on disk (workspace not deployed)
//   - more than one candidate exists (ambiguous; no file is preferred)
func DiscoverOrchestrator(workDir, harnessID string) (string, error) {
	format, ok := agentfile.For(harnessID)
	if !ok {
		return "", &domain.RefusalError{
			Component: "harness",
			Resource:  harnessID,
			Reason:    fmt.Sprintf("unknown harness %s: cannot determine agents directory", harnessID),
		}
	}

	// Shortest extension first, so a Markdown harness lists the plain name
	// before the .agent.md name.
	exts := format.Extensions()
	candidates := make([]string, 0, len(exts))
	for i := len(exts) - 1; i >= 0; i-- {
		candidates = append(candidates, filepath.Join(workDir, format.AgentsDir, orchestratorStem+exts[i]))
	}

	var existing []string
	for _, candidate := range candidates {
		exists, err := fileExists(candidate)
		if err != nil {
			return "", accessRefusal(candidate, err)
		}
		if exists {
			existing = append(existing, candidate)
		}
	}

	switch {
	case len(existing) > 1:
		return "", &domain.RefusalError{
			Component: "harness",
			Resource:  filepath.Join(workDir, format.AgentsDir),
			Reason: fmt.Sprintf("ambiguous orchestrator-script for harness %s: %s exist; keep only one",
				harnessID, strings.Join(existing, " and ")),
		}
	case len(existing) == 1:
		return existing[0], nil
	}

	return "", &domain.RefusalError{
		Component: "harness",
		Resource:  candidates[0],
		Reason: fmt.Sprintf("workspace not deployed for harness %s: expected orchestrator-script at %s",
			harnessID, strings.Join(candidates, " or ")),
	}
}

// fileExists reports whether path exists; a non-nil error means the path could
// not be inspected for a reason other than absence.
func fileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func accessRefusal(path string, err error) *domain.RefusalError {
	return &domain.RefusalError{
		Component: "harness",
		Resource:  path,
		Reason:    fmt.Sprintf("cannot access expected orchestrator-script: %v", err),
	}
}
