package harness

import (
	"fmt"
	"os"
	"path/filepath"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
)

const (
	orchestratorPlainName = "orchestrator-script.md"
	orchestratorAgentName = "orchestrator-script.agent.md"
)

// DiscoverOrchestrator computes the expected path to the script-mode
// orchestrator agent file from the harness convention and verifies the file
// exists.
//
// The path is either {workDir}/{agentsDir}/orchestrator-script.md or
// {workDir}/{agentsDir}/orchestrator-script.agent.md.
//
// Returns the absolute path on success.
// Returns *domain.RefusalError with Component "harness" if:
//   - harnessID is not a known CLI harness (unknown identity)
//   - neither file exists on disk (workspace not deployed)
//   - both files exist (ambiguous; no file is preferred)
func DiscoverOrchestrator(workDir, harnessID string) (string, error) {
	entry, ok := commonharness.LookupCLIHarness(harnessID)
	if !ok {
		return "", &domain.RefusalError{
			Component: "harness",
			Resource:  harnessID,
			Reason:    fmt.Sprintf("unknown harness %s: cannot determine agents directory", harnessID),
		}
	}

	plainPath := filepath.Join(workDir, entry.AgentsDir, orchestratorPlainName)
	agentPath := filepath.Join(workDir, entry.AgentsDir, orchestratorAgentName)

	plainExists, err := fileExists(plainPath)
	if err != nil {
		return "", accessRefusal(plainPath, err)
	}
	agentExists, err := fileExists(agentPath)
	if err != nil {
		return "", accessRefusal(agentPath, err)
	}

	switch {
	case plainExists && agentExists:
		return "", &domain.RefusalError{
			Component: "harness",
			Resource:  filepath.Join(workDir, entry.AgentsDir),
			Reason: fmt.Sprintf("ambiguous orchestrator-script for harness %s: both %s and %s exist; keep only one",
				harnessID, plainPath, agentPath),
		}
	case plainExists:
		return plainPath, nil
	case agentExists:
		return agentPath, nil
	}

	return "", &domain.RefusalError{
		Component: "harness",
		Resource:  plainPath,
		Reason: fmt.Sprintf("workspace not deployed for harness %s: expected orchestrator-script at %s or %s",
			harnessID, plainPath, agentPath),
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
