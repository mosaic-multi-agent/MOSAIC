package artifact

import (
	"path/filepath"

	"mosaic-run/internal/domain"
)

// EvidenceDir returns {parent of the run folder}/RunnerLogs/{run_id} and true
// when IsRunScopedArtifactPath(path); "" and false otherwise.
func EvidenceDir(artifactPath string) (string, bool) {
	if !IsRunScopedArtifactPath(artifactPath) {
		return "", false
	}
	runFolder := filepath.Dir(artifactPath)
	runID, _ := domain.ParseRunFolder(filepath.Base(runFolder))
	return filepath.Join(filepath.Dir(runFolder), "RunnerLogs", runID), true
}
