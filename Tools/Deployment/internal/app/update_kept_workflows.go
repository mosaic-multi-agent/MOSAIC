package app

// update_kept_workflows.go reads the workflow layout of each deployed orchestrator-role file
// for the update flow and classifies its workflows: ids the catalog supplies are refreshed
// as usual, all others are kept as deployed, byte for byte, in their own file.

import (
	"fmt"
	"path/filepath"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// deployedOrchestratorFile names an orchestrator-role file the update flow found on disk.
type deployedOrchestratorFile struct {
	TargetPath string
	Present    bool
}

// keptWorkflows is what the update flow keeps in the deployed orchestrator-role files.
type keptWorkflows struct {
	byPath map[string]DeployedPathWorkflows
	ids    []string // kept ids of all files, deduplicated, in first-seen order
}

// customIDSet returns the kept ids as a membership set.
func (k keptWorkflows) customIDSet() map[string]bool {
	set := make(map[string]bool, len(k.ids))
	for _, id := range k.ids {
		set[id] = true
	}
	return set
}

// readKeptWorkflows reads the workflow blocks of each present file and keeps those whose id the
// catalog does not supply. A file whose workflows cannot be read is an error naming its path.
func readKeptWorkflows(workspace string, src *domain.HarnessDescriptor, cat catalog.Catalog, files []deployedOrchestratorFile) (keptWorkflows, error) {
	result := keptWorkflows{byPath: map[string]DeployedPathWorkflows{}}
	seen := map[string]bool{}
	for _, f := range files {
		if !f.Present || f.TargetPath == "" {
			continue
		}
		read := readDeployedArtifact(filepath.Join(workspace, f.TargetPath), domain.ArtifactAgent, src)
		if read.ReadErr != nil {
			return keptWorkflows{}, fmt.Errorf("%s: %w", f.TargetPath, read.ReadErr)
		}
		if read.DecodeErr != nil {
			return keptWorkflows{}, fmt.Errorf("%s: %w", f.TargetPath, read.DecodeErr)
		}
		blocks, err := transform.KeptWorkflowBlocks(read.Canonical)
		if err != nil {
			return keptWorkflows{}, fmt.Errorf("%s: %w", f.TargetPath, err)
		}
		layout := DeployedPathWorkflows{}
		for _, b := range blocks {
			if b.ID == "" {
				continue
			}
			layout.Order = append(layout.Order, b.ID)
			if _, supplied := cat.Workflow(b.ID); supplied {
				continue
			}
			b.Kept = true
			layout.Kept = append(layout.Kept, b)
			if !seen[b.ID] {
				seen[b.ID] = true
				result.ids = append(result.ids, b.ID)
			}
		}
		if len(layout.Kept) > 0 {
			result.byPath[f.TargetPath] = layout
		}
	}
	return result, nil
}
