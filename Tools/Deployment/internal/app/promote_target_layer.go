package app

// promote_target_layer.go decides which catalog layer receives a promoted file and derives
// everything that depends on that layer: the destination path and the category options.
//
// An item whose key the stack resolved is written back to the catalog it was resolved from.
// A new item has no such source, so the user chooses the layer every time: through
// QPromoteTargetCatalog interactively, or through PromoteRequest.TargetCatalogID. When a
// choice is needed and none is made, Promote fails and writes nothing.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
)

// errPromoteTargetRequired reports a new item promoted into a multi-layer stack with no layer
// chosen. Placement is never guessed.
var errPromoteTargetRequired = errors.New("promote needs a target catalog for a new item: pass --target-catalog with one of the active catalog ids")

// promoteTarget returns the root (and catalog id, empty for a plain catalog) of the layer
// that receives the promoted file for agent key. A non-empty req.TargetCatalogID must name an
// active layer even when the item has a resolved source; the resolved source then wins.
func (s *service) promoteTarget(ctx context.Context, req PromoteRequest, key string) (root, catalogID string, err error) {
	lister, isStack := s.deps.Catalog.(catalog.LayerLister)
	if !isStack {
		if req.TargetCatalogID != "" {
			return "", "", fmt.Errorf("target catalog %q: the active catalog has no layers", req.TargetCatalogID)
		}
		return s.deps.Catalog.CatalogRoot(), "", nil
	}
	layers := lister.Layers()
	if req.TargetCatalogID != "" && findPromoteLayer(layers, req.TargetCatalogID) < 0 {
		return "", "", fmt.Errorf("target catalog %q is not an active catalog layer", req.TargetCatalogID)
	}

	if reporter, ok := s.deps.Catalog.(catalog.ProvenanceReporter); ok {
		if layer, known := reporter.Provenance(domain.CatalogItem{Kind: domain.CatalogItemAgent, Key: key}); known {
			return layer.Root, layer.Identity.ID, nil
		}
	}

	idx := 0
	switch {
	case req.TargetCatalogID != "":
		idx = findPromoteLayer(layers, req.TargetCatalogID)
	case len(layers) > 1:
		chosen, askErr := s.askPromoteTargetLayer(ctx, layers, req.FilePath)
		if askErr != nil {
			return "", "", askErr
		}
		idx = chosen
	}
	return layers[idx].Root, layers[idx].Identity.ID, nil
}

// targetRequiredError wraps errPromoteTargetRequired with the catalog ids of the stack in
// effect, highest layer first.
func targetRequiredError(layers []domain.CatalogLayer) error {
	ids := make([]string, 0, len(layers))
	for _, l := range layers {
		ids = append(ids, l.Identity.ID)
	}
	return fmt.Errorf("%w (active catalogs, highest first: %s)", errPromoteTargetRequired, strings.Join(ids, ", "))
}

// findPromoteLayer returns the index of the layer with the given catalog id, or -1.
func findPromoteLayer(layers []domain.CatalogLayer, id string) int {
	for i, l := range layers {
		if l.Identity.ID == id {
			return i
		}
	}
	return -1
}

// askPromoteTargetLayer asks which layer receives a new item and returns its index. Any
// outcome other than an answer naming an active layer is an error.
func (s *service) askPromoteTargetLayer(ctx context.Context, layers []domain.CatalogLayer, srcPath string) (int, error) {
	opts := make([]domain.Option, 0, len(layers))
	for _, l := range layers {
		opts = append(opts, domain.Option{
			ID:          l.Identity.ID,
			Label:       fmt.Sprintf("%s (%s %s)", l.Identity.Name, l.Identity.ID, l.Identity.Version),
			Description: l.Root,
		})
	}
	ans, err := s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
		Question: domain.Question{
			ID:        domain.QPromoteTargetCatalog,
			Subject:   srcPath,
			Title:     "Select the catalog that receives the promoted agent",
			AllowSkip: true,
		},
		Options: opts,
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %w", targetRequiredError(layers), err)
	}
	if ans.Status != domain.Answered {
		return 0, targetRequiredError(layers)
	}
	idx := findPromoteLayer(layers, ans.OptionID)
	if idx < 0 {
		return 0, fmt.Errorf("target catalog %q is not an active catalog layer", ans.OptionID)
	}
	return idx, nil
}

// promoteDestinationPath computes where the generated generic file is written, relative
// to the target layer's catalogue root:
//
//	category == PromoteCategoryUtility → <root>/UtilityAgents/{key}.md
//	otherwise                          → <root>/Subagents/{category}/{key}.md
//
// key is the source file's base name with a trailing ".agent.md" or ".md" removed — the
// same derivation the catalog uses to key an agent, so the promoted file lands under the
// key the user expects.
func promoteDestinationPath(catalogRoot, category, key string) string {
	if category == PromoteCategoryUtility {
		return catalogpaths.UtilityAgentFile(catalogRoot, key)
	}
	return catalogpaths.SubagentFile(catalogRoot, category, key)
}

// askPromoteCategory asks the user to select a target category for the promoted agent, from
// the categories of the target layer. It returns ErrPromoteCategoryRequired when the question
// is cancelled, skipped, or unanswered. Placement is never guessed.
func (s *service) askPromoteCategory(ctx context.Context, srcPath, targetRoot string) (string, error) {
	opts, err := buildPromoteCategoryOptions(targetRoot)
	if err != nil {
		return "", fmt.Errorf("reading promote category options: %w", err)
	}

	ans, err := s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
		Question: domain.Question{
			ID:        domain.QPromoteCategory,
			Subject:   srcPath,
			Title:     "Select a target category for the promoted agent",
			AllowSkip: true,
		},
		Options: opts,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPromoteCategoryRequired, err)
	}
	if ans.Status != domain.Answered {
		return "", ErrPromoteCategoryRequired
	}
	return ans.OptionID, nil
}

// buildPromoteCategoryOptions returns the SelectOne options for QPromoteCategory: one per
// subdirectory of the target root's Subagents/ directory (Option.ID is the directory name),
// plus the PromoteCategoryUtility option. Options are sorted by ID so the presented list
// is deterministic regardless of filesystem ordering.
func buildPromoteCategoryOptions(targetRoot string) ([]domain.Option, error) {
	entries, err := os.ReadDir(catalogpaths.SubagentsDir(targetRoot))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading Subagents/: %w", err)
	}

	var opts []domain.Option
	for _, entry := range entries {
		if entry.IsDir() {
			opts = append(opts, domain.Option{ID: entry.Name(), Label: entry.Name()})
		}
	}
	opts = append(opts, domain.Option{
		ID:    PromoteCategoryUtility,
		Label: "Utility Agents (UtilityAgents/)",
	})
	sort.Slice(opts, func(i, j int) bool { return opts[i].ID < opts[j].ID })
	return opts, nil
}
