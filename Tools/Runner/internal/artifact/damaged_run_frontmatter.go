package artifact

import (
	"errors"
	"os"
	"path/filepath"

	"mosaic-run/internal/domain"
)

// lastGoodFileName is the name of the last-good copy in the evidence directory.
const lastGoodFileName = "Orchestration.last-good.md"

// DamagedRunFrontmatter returns the recorded frontmatter values of the run
// whose Orchestration.md is at artifactPath, given readErr, the error reading
// that file returned. It returns false unless readErr's chain holds an
// *domain.ArtifactDamageError. Sources, in order:
//  1. the damage error's own Frontmatter, when non-nil;
//  2. otherwise the frontmatter of Orchestration.last-good.md in
//     EvidenceDir(artifactPath), when that copy is usable;
//  3. otherwise zero and false.
//
// The copy is usable when it exists and reads (ParseAt) either without error
// or with an *domain.ArtifactDamageError whose Frontmatter is non-nil, and its
// recorded run_id equals the run id of artifactPath's run folder. A copy
// refused as pre-3.0 layout or for its run identity, or unreadable, is not
// usable. Neither file is written.
func DamagedRunFrontmatter(artifactPath string, readErr error) (domain.ArtifactState, bool) {
	var damage *domain.ArtifactDamageError
	if !errors.As(readErr, &damage) {
		return domain.ArtifactState{}, false
	}
	if damage.Frontmatter != nil {
		return *damage.Frontmatter, true
	}
	return lastGoodFrontmatter(artifactPath)
}

// lastGoodFrontmatter returns the frontmatter values of the run's last-good
// copy when the copy is usable for the run at artifactPath.
func lastGoodFrontmatter(artifactPath string) (domain.ArtifactState, bool) {
	dir, ok := EvidenceDir(artifactPath)
	if !ok {
		return domain.ArtifactState{}, false
	}
	copyPath := filepath.Join(dir, lastGoodFileName)
	data, err := os.ReadFile(copyPath)
	if err != nil {
		return domain.ArtifactState{}, false
	}
	state, err := ParseAt(copyPath, data)
	if err != nil {
		var copyDamage *domain.ArtifactDamageError
		if !errors.As(err, &copyDamage) || copyDamage.Frontmatter == nil {
			return domain.ArtifactState{}, false
		}
		state = *copyDamage.Frontmatter
	}
	runID, _ := domain.ParseRunFolder(filepath.Base(filepath.Dir(artifactPath)))
	if state.RunID != runID {
		return domain.ArtifactState{}, false
	}
	return state, true
}
