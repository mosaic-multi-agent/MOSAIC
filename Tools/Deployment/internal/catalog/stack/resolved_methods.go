package stack

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// Root returns the MOSAIC repository root.
func (r *resolved) Root() string { return r.mosaicRoot }

// CatalogRoot returns the root of the highest layer. This is an interim meaning: a stack has no
// single catalogue root, and the consumers of this method are reworked in later stages.
func (r *resolved) CatalogRoot() string { return r.layers[0].info.Root }

// Layers returns the composed layers, highest precedence first.
func (r *resolved) Layers() []domain.CatalogLayer {
	out := make([]domain.CatalogLayer, len(r.layers))
	for i, l := range r.layers {
		out[i] = l.info
	}
	return out
}

func (r *resolved) Agents() []domain.Agent { return r.workers }

func (r *resolved) Agent(key string) (domain.Agent, bool) {
	a, ok := r.agentIdx[key]
	return a, ok
}

func (r *resolved) Orchestrator() domain.Agent { return r.orch }

func (r *resolved) OrchestratorScript() (domain.Agent, bool) { return r.orchScript, r.orchScriptOK }

func (r *resolved) UtilityAgents() []domain.Agent { return r.utilities }

func (r *resolved) StandaloneAgents() []domain.Agent { return r.standalones }

func (r *resolved) InfrastructureAgents() []domain.Agent {
	var result []domain.Agent
	for _, a := range r.workers {
		if a.Infrastructure != "" {
			result = append(result, a)
		}
	}
	return result
}

func (r *resolved) Skills() []domain.Skill { return r.skills }

func (r *resolved) Skill(key string) (domain.Skill, bool) {
	s, ok := r.skillIdx[key]
	return s, ok
}

func (r *resolved) Hooks() []domain.HookBundle { return r.hooks }

func (r *resolved) Hook(key string) (domain.HookBundle, bool) {
	h, ok := r.hookIdx[key]
	return h, ok
}

func (r *resolved) Workflows() []domain.Workflow { return r.workflows }

func (r *resolved) Workflow(id string) (domain.Workflow, bool) {
	w, ok := r.wfIdx[id]
	return w, ok
}

func (r *resolved) WorkflowCategories() []domain.WorkflowCategory { return r.categories }

func (r *resolved) Tiers() []domain.TierInfo { return r.tiers }

// WorkflowSection reads the section from the layer that supplies the winning workflow.
func (r *resolved) WorkflowSection(id string) ([]byte, error) {
	layer, ok := r.wfLayer[id]
	if !ok {
		return nil, fmt.Errorf("catalog: workflow %q not found", id)
	}
	return layer.WorkflowSection(id)
}

// ReadSource serves only paths emitted by winning items; shadowed items' files are refused.
func (r *resolved) ReadSource(path string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("catalog: ReadSource: %w", err)
	}
	if !r.sources[abs] {
		return nil, fmt.Errorf("catalog: path %q was not emitted by this catalog", path)
	}
	return os.ReadFile(abs)
}

// Issues is the concatenation of the layers' issues, highest layer first, each attributed to the
// catalog of the layer that raised it.
func (r *resolved) Issues() []catalog.Issue { return r.issues }

func (r *resolved) AgentByNumericID(id string) (domain.Agent, bool) {
	if id == "" {
		return domain.Agent{}, false
	}
	a, ok := r.numericIdx[id]
	return a, ok
}
