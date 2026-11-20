package app

// promote_batch_discovery.go finds the promotable agents of an agents folder and asks which of
// them to promote. Promotable means what single-agent promote accepts: the scan rules of
// scanHarnessOnlyAgents, applied without excluding keys the catalog already holds.

import (
	"context"
	"fmt"
	"os"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// promotableAgent is one promotable file of the agents folder.
type promotableAgent struct {
	// file is the file name inside the folder.
	file string
	// path is the absolute path of the file.
	path string
	// key is the agent key derived from the file name.
	key string
}

// checkPromoteAgentsDir validates the agents folder of a batch request.
func checkPromoteAgentsDir(dir string) error {
	if dir == "" {
		return ErrPromoteAgentsDirRequired
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrPromoteAgentsDirInvalid, dir)
	}
	return nil
}

// discoverPromotableAgents lists the promotable agents of dir sorted by file name: files with
// the harness agent extension, not MOSAIC orchestrator files, and eligible as harness-only
// agents. The scan is non-recursive and never writes. Keys the catalog already holds are listed
// too.
func discoverPromotableAgents(dir string, desc *domain.HarnessDescriptor) []promotableAgent {
	found, _ := scanHarnessOnlyAgents("", dir, nil, desc)
	out := make([]promotableAgent, 0, len(found))
	for _, a := range found {
		out = append(out, promotableAgent{file: a.FileName, path: a.TargetPath, key: a.Key})
	}
	return out
}

// promoteLayerLabel is the label of a catalog layer in questions and results.
func promoteLayerLabel(l domain.CatalogLayer) string {
	if l.Default {
		return fmt.Sprintf("MOSAIC default catalog (%s)", l.Identity.ID)
	}
	return fmt.Sprintf("%s (%s)", l.Identity.Name, l.Identity.ID)
}

// plainCatalogLabel names the catalog in a question when it has no layers.
const plainCatalogLabel = "the active catalog"

// promoteCatalogLayers returns the layers of the service's catalog, highest first; nil for a
// catalog that is not a stack.
func (s *service) promoteCatalogLayers() []domain.CatalogLayer {
	if lister, ok := s.deps.Catalog.(catalog.LayerLister); ok {
		return lister.Layers()
	}
	return nil
}

// layerLabelOfPath names the layer that holds the file at path.
func (s *service) layerLabelOfPath(path string) string {
	for _, l := range s.promoteCatalogLayers() {
		if pathInside(l.Root, path) {
			return promoteLayerLabel(l)
		}
	}
	return plainCatalogLabel
}

// stackAgentHolders maps each agent key the catalog resolves to the label of the layer that
// supplies it.
func (s *service) stackAgentHolders() map[string]string {
	holders := make(map[string]string)
	for _, a := range promoteCatalogAgents(s.deps.Catalog) {
		if _, seen := holders[a.Key]; !seen {
			holders[a.Key] = s.layerLabelOfPath(a.SourcePath)
		}
	}
	return holders
}

// askPromoteAgents asks which of the agents to promote and returns the chosen ones in option
// order. A selection that is not an answer naming at least one offered agent ends with an error
// wrapping ErrSelectionCancelled; an interaction error is returned wrapped.
func (s *service) askPromoteAgents(ctx context.Context, dir string, agents []promotableAgent) ([]promotableAgent, error) {
	holders := s.stackAgentHolders()
	opts := make([]domain.Option, 0, len(agents))
	for _, a := range agents {
		opt := domain.Option{ID: a.file, Label: a.key, Description: a.file}
		if holder, exists := holders[a.key]; exists {
			opt.Hint = "already in " + holder
		}
		opts = append(opts, opt)
	}
	q := domain.ChoiceQuestion{
		Question: domain.Question{
			ID:      domain.QPromoteAgents,
			Subject: dir,
			Title:   "Select the agents to promote",
		},
		Options: opts,
	}
	ans, err := s.deps.Interaction.SelectMany(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("asking which agents to promote: %w", err)
	}
	if ans.Status != domain.Answered {
		return nil, fmt.Errorf("%w: %s", ErrSelectionCancelled, q.ID)
	}
	picked := make(map[string]bool, len(ans.OptionIDs))
	for _, id := range ans.OptionIDs {
		picked[id] = true
	}
	var chosen []promotableAgent
	for _, a := range agents {
		if picked[a.file] {
			chosen = append(chosen, a)
		}
	}
	if len(chosen) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrSelectionCancelled, q.ID)
	}
	return chosen, nil
}
