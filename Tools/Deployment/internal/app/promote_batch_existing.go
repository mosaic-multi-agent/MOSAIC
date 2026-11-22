package app

// promote_batch_existing.go handles an agent of a batch whose key already exists in the stack:
// finding the existing agent, asking whether to replace it, and what a replacement takes over.

import (
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// batchExisting is the agent with the promoted key that the stack already holds.
type batchExisting struct {
	// path is the existing file when it lives in the batch layer; empty when another layer
	// holds it, which a replacement never modifies.
	path string
	// numericID is the existing agent's id; empty when it could not be read.
	numericID string
	// layer is the label of the layer that holds the agent.
	layer string
}

// findBatchExisting returns the agent with key the stack holds, or nil. The batch layer's own
// agent wins over the agent another layer supplies, because a replacement takes its place.
func (r *promoteBatchRun) findBatchExisting(key string) *batchExisting {
	if path := findAgentFileInRoot(r.layer.root, key); path != "" {
		label := r.layer.label
		if label == "" {
			label = plainCatalogLabel
		}
		return &batchExisting{path: path, numericID: readAgentNumericID(path), layer: label}
	}
	for _, a := range promoteCatalogAgents(r.s.deps.Catalog) {
		if a.Key == key {
			return &batchExisting{numericID: a.NumericID, layer: r.s.layerLabelOfPath(a.SourcePath)}
		}
	}
	return nil
}

// findAgentFileInRoot returns the generic agent file for key under a catalog root (a subagent,
// a standalone agent or a utility agent), or "".
func findAgentFileInRoot(root, key string) string {
	if root == "" {
		return ""
	}
	if p := catalogpaths.UtilityAgentFile(root, key); isRegularFile(p) {
		return p
	}
	for _, dir := range []string{catalogpaths.SubagentsDir(root), catalogpaths.StandaloneAgentsDir(root)} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if p := filepath.Join(dir, entry.Name(), key+".md"); isRegularFile(p) {
				return p
			}
		}
	}
	return ""
}

// isRegularFile reports whether path is a regular file.
func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// readAgentNumericID reads the id of the generic agent file at path; empty when unreadable.
func readAgentNumericID(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	doc, err := ParseGenericSource(b)
	if err != nil {
		return ""
	}
	if v, ok := doc.Frontmatter().Get("id"); ok && v.Kind == domain.KindScalar {
		return v.Scalar
	}
	return ""
}

// askReplaceExisting asks whether to replace the existing agent. It reports replace true only
// for the explicit replace option; every other answer keeps the existing agent. The second
// result is false when the batch was aborted.
func (r *promoteBatchRun) askReplaceExisting(key string, ex *batchExisting) (replace, ok bool) {
	ans, ok := r.selectOne(domain.ChoiceQuestion{
		Question: domain.Question{
			ID:              domain.QPromoteExistingAgent,
			Subject:         key,
			Title:           "Agent " + key + " already exists in " + ex.layer,
			DefaultOptionID: PromoteExistingSkip,
		},
		Options: []domain.Option{
			{ID: PromoteExistingSkip, Label: "Skip this agent"},
			{ID: PromoteExistingReplace, Label: "Replace the existing agent in " + ex.layer},
		},
	})
	if !ok {
		return false, false
	}
	return ans.Status == domain.Answered && ans.OptionID == PromoteExistingReplace, true
}
