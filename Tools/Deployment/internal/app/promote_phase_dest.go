package app

// promote_phase_dest.go is the destination phase of promoting one agent: choose the target
// layer and category, find an agent with the same key already in that layer, refuse or accept
// its replacement, and decide the numeric id of the new file.
//
// An agent with the same key in the target layer is found by key, not by path, so a different
// category cannot produce a second file with the key. A replacement keeps the replaced agent's
// numeric id and key; the old file is removed after the new one is written when it lives at a
// different path.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// promoteExisting is an agent with the promoted key that already lives in the target layer.
type promoteExisting struct {
	// path is the existing agent's file.
	path string
	// numericID is the existing agent's id; empty when only the file is known.
	numericID string
}

// promoteDestination is where a promoted agent is written.
type promoteDestination struct {
	// targetRoot is the catalog root of the target layer.
	targetRoot string
	// targetCatalogID is the target layer's catalog id; empty for a plain catalog.
	targetCatalogID string
	// category is the chosen category, or PromoteCategoryUtility.
	category string
	// path is the file the new agent is written to.
	path string
	// existing is the agent being replaced; nil when the key is new to the layer.
	existing *promoteExisting
}

// resolvePromoteDestination chooses the target layer and category for key, finds the existing
// agent with that key in the layer, and refuses to go on when one exists and overwrite is not
// granted. A non-empty category suppresses the category question.
func (s *service) resolvePromoteDestination(
	ctx context.Context, req PromoteRequest, key, category string,
) (promoteDestination, error) {
	// The layer is chosen before the category because the category options come from its
	// Subagents/ directory.
	targetRoot, targetCatalogID, err := s.promoteTarget(ctx, req, key)
	if err != nil {
		return promoteDestination{}, err
	}

	if category == "" {
		category, err = s.askPromoteCategory(ctx, req.FilePath, targetRoot)
		if err != nil {
			return promoteDestination{}, err
		}
	}

	// A category with ".." segments or path separators could escape the catalogue root; it is
	// rejected before any directory is created.
	if strings.Contains(category, "..") || strings.ContainsAny(category, "/\\") {
		return promoteDestination{}, fmt.Errorf("invalid category %q: category must be a single directory name without path traversal or separators", category)
	}

	dest := promoteDestination{
		targetRoot:      targetRoot,
		targetCatalogID: targetCatalogID,
		category:        category,
		path:            promoteDestinationPath(targetRoot, category, key),
	}
	dest.existing = s.findPromoteExisting(targetRoot, key, dest.path)

	// Promote never silently replaces a generic agent; the caller must set Overwrite.
	if dest.existing != nil && !req.Overwrite {
		return promoteDestination{}, fmt.Errorf("%w: %s", ErrPromoteDestinationExists, dest.existing.path)
	}
	return dest, nil
}

// findPromoteExisting returns the agent with key that lives under targetRoot, or nil. The
// catalog is searched by key; a file already at destPath that the catalog does not know (it is
// not reloaded after writes) counts as existing too, without a numeric id.
func (s *service) findPromoteExisting(targetRoot, key, destPath string) *promoteExisting {
	for _, a := range promoteCatalogAgents(s.deps.Catalog) {
		if a.Key == key && pathInside(targetRoot, a.SourcePath) {
			return &promoteExisting{path: a.SourcePath, numericID: a.NumericID}
		}
	}
	if _, err := os.Stat(destPath); err == nil {
		return &promoteExisting{path: destPath}
	}
	return nil
}

// promoteCatalogAgents returns every agent the catalog holds that can live in a layer:
// subagents, standalone agents and utility agents.
func promoteCatalogAgents(c catalog.Catalog) []domain.Agent {
	all := append([]domain.Agent{}, c.Agents()...)
	all = append(all, c.StandaloneAgents()...)
	return append(all, c.UtilityAgents()...)
}

// pathInside reports whether path is root or lies below it.
func pathInside(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// promoteNumericIDFor returns the id for the file written to dest: the replaced agent's id
// when there is one, otherwise one greater than the largest id in the catalog.
func promoteNumericIDFor(dest promoteDestination, c catalog.Catalog) string {
	if dest.existing != nil && dest.existing.numericID != "" {
		return dest.existing.numericID
	}
	return nextNumericID(c)
}

// nextNumericID returns the numeric agent id to assign to a newly promoted agent: one
// greater than the largest numeric id currently declared by any agent in the catalog,
// rendered in decimal. When the catalog declares no parseable numeric id, it returns "1".
//
// Non-numeric and empty id values are ignored rather than treated as errors. The result
// is deterministic for a given catalog state, which is what makes the write assertable in
// a test.
func nextNumericID(c catalog.Catalog) string {
	max := 0
	all := make([]domain.Agent, 0)
	all = append(all, c.Agents()...)
	all = append(all, c.UtilityAgents()...)
	orch := c.Orchestrator()
	if orch.Key != "" {
		all = append(all, orch)
	}
	for _, a := range all {
		if a.NumericID == "" {
			continue
		}
		n, err := strconv.Atoi(a.NumericID)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return strconv.Itoa(max + 1)
}

// sameFile reports whether a and b are the same file on disk, independent of how each path
// is spelled.
func sameFile(a, b string) bool {
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}

// writePromotedAgent writes the generated bytes to dest.path, creating the directory, and then
// removes the replaced agent's file when it differs from the new one. When the removal fails
// the new file stays, and the error names both files.
func writePromotedAgent(dest promoteDestination, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest.path), 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	if err := os.WriteFile(dest.path, content, 0o644); err != nil {
		return fmt.Errorf("writing promoted agent file: %w", err)
	}
	if dest.existing == nil || sameFile(dest.existing.path, dest.path) {
		return nil
	}
	if err := os.Remove(dest.existing.path); err != nil {
		return fmt.Errorf("removing replaced agent %s after writing %s: %w", dest.existing.path, dest.path, err)
	}
	return nil
}
