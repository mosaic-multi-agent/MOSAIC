package stack

import (
	"path/filepath"
	"sort"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/catalog/stack/stackrec"
	"mosaic-deploy/internal/catalog/stack/systemfile"
	"mosaic-deploy/internal/domain"
)

// resolved is the immutable composed catalog. Every field is derived from the layers, highest
// layer first, so a key already taken by a higher layer is never replaced.
type resolved struct {
	mosaicRoot string
	layers     []loadedLayer

	workers      []domain.Agent // sorted by Key
	utilities    []domain.Agent // sorted by Key
	standalones  []domain.Agent // sorted by Key
	orch         domain.Agent
	orchScript   domain.Agent
	orchScriptOK bool
	agentIdx     map[string]domain.Agent
	numericIdx   map[string]domain.Agent

	skills   []domain.Skill
	skillIdx map[string]domain.Skill
	hooks    []domain.HookBundle
	hookIdx  map[string]domain.HookBundle

	workflows  []domain.Workflow
	wfIdx      map[string]domain.Workflow
	wfLayer    map[string]catalog.Catalog // layer catalog that supplies each workflow
	categories []domain.WorkflowCategory

	tiers      []domain.TierInfo
	issues     []catalog.Issue
	shadows    []stackrec.Shadow
	sysFile    systemfile.Result
	harnessInj injections.Result

	suppliers map[domain.CatalogItem][]int // layer indexes supplying each item, highest first
	sources   map[string]bool              // absolute paths emitted by winning items
}

// resolve composes the loaded layers (highest first) into one catalog.
func resolve(layers []loadedLayer) *resolved {
	r := &resolved{
		mosaicRoot: layers[0].cat.Root(),
		layers:     layers,
		agentIdx:   make(map[string]domain.Agent),
		skillIdx:   make(map[string]domain.Skill),
		hookIdx:    make(map[string]domain.HookBundle),
		wfIdx:      make(map[string]domain.Workflow),
		wfLayer:    make(map[string]catalog.Catalog),
		sources:    make(map[string]bool),
	}
	r.resolveOrchestrators()
	r.resolveAgents()
	r.resolveSkillsAndHooks()
	r.resolveWorkflows()
	r.issues = attributeIssues(layers)
	r.suppliers = supplierIndexes(layers)
	r.shadows = buildShadows(layers, r.suppliers)
	r.sysFile = systemfile.Resolve(layerInputs(layers))
	r.shadows = append(r.shadows, r.sysFile.Shadows...)
	r.harnessInj = injections.Resolve(layerInputs(layers))
	r.shadows = append(r.shadows, r.harnessInj.Shadows...)
	sortShadows(r.shadows)
	r.tiers = buildTiers(r.agentIdx)
	r.numericIdx = buildNumericIndex(r)
	return r
}

// resolveOrchestrators takes both orchestrators from the highest layer that supplies either of
// them, so the pair never mixes layers.
func (r *resolved) resolveOrchestrators() {
	for _, l := range r.layers {
		orch := l.cat.Orchestrator()
		script, scriptOK := l.cat.OrchestratorScript()
		if orch.Key == "" && !scriptOK {
			continue
		}
		if orch.Key != "" {
			r.orch = orch
			r.agentIdx[orch.Key] = orch
			r.addSource(orch.SourcePath)
		}
		if scriptOK {
			r.orchScript, r.orchScriptOK = script, true
			r.agentIdx[script.Key] = script
			r.addSource(script.SourcePath)
		}
		return
	}
}

// resolveAgents resolves worker, utility and standalone agents by key across all roles.
func (r *resolved) resolveAgents() {
	take := func(a domain.Agent) bool {
		if _, taken := r.agentIdx[a.Key]; taken {
			return false
		}
		r.agentIdx[a.Key] = a
		r.addSource(a.SourcePath)
		return true
	}
	for _, l := range r.layers {
		for _, a := range l.cat.Agents() {
			if take(a) {
				r.workers = append(r.workers, a)
			}
		}
		for _, a := range l.cat.UtilityAgents() {
			if take(a) {
				r.utilities = append(r.utilities, a)
			}
		}
		for _, a := range l.cat.StandaloneAgents() {
			if take(a) {
				r.standalones = append(r.standalones, a)
			}
		}
	}
	for _, list := range [][]domain.Agent{r.workers, r.utilities, r.standalones} {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Key < list[j].Key })
	}
}

// resolveSkillsAndHooks replaces each skill and hook bundle as a whole item: only the winning
// layer's files and variants are kept and made readable.
func (r *resolved) resolveSkillsAndHooks() {
	for _, l := range r.layers {
		for _, s := range l.cat.Skills() {
			if _, taken := r.skillIdx[s.Key]; taken {
				continue
			}
			r.skillIdx[s.Key] = s
			r.skills = append(r.skills, s)
			for _, f := range s.Files {
				r.addSource(filepath.Join(s.SourceDir, f))
			}
		}
		for _, h := range l.cat.Hooks() {
			if _, taken := r.hookIdx[h.Key]; taken {
				continue
			}
			r.hookIdx[h.Key] = h
			r.hooks = append(r.hooks, h)
			for _, v := range h.Variants {
				for _, f := range v.Files {
					r.addSource(f.SourcePath)
				}
			}
		}
	}
}

// resolveWorkflows builds the union of workflows with per-id shadowing and groups categories of
// the same name across layers (alphabetical; ids ascending).
func (r *resolved) resolveWorkflows() {
	byCategory := make(map[string][]domain.Workflow)
	for _, l := range r.layers {
		for _, w := range l.cat.Workflows() {
			if _, taken := r.wfIdx[w.ID]; taken {
				continue
			}
			r.wfIdx[w.ID] = w
			r.wfLayer[w.ID] = l.cat
			r.addSource(w.SourcePath)
			byCategory[w.Category] = append(byCategory[w.Category], w)
		}
	}
	names := make([]string, 0, len(byCategory))
	for name := range byCategory {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wfs := byCategory[name]
		sort.Slice(wfs, func(i, j int) bool { return wfs[i].ID < wfs[j].ID })
		r.categories = append(r.categories, domain.WorkflowCategory{Name: name, Workflows: wfs})
		r.workflows = append(r.workflows, wfs...)
	}
}

func (r *resolved) addSource(path string) {
	if abs, err := filepath.Abs(path); err == nil {
		r.sources[abs] = true
	}
}

// buildNumericIndex indexes the resolved agents by numeric id in the order the single-root
// loader uses (workers, utilities, orchestrator); the first agent holding an id keeps it.
func buildNumericIndex(r *resolved) map[string]domain.Agent {
	idx := make(map[string]domain.Agent)
	ordered := make([]domain.Agent, 0, len(r.workers)+len(r.utilities)+1)
	ordered = append(ordered, r.workers...)
	ordered = append(ordered, r.utilities...)
	if r.orch.Key != "" {
		ordered = append(ordered, r.orch)
	}
	for _, a := range ordered {
		if a.NumericID == "" {
			continue
		}
		if _, exists := idx[a.NumericID]; !exists {
			idx[a.NumericID] = a
		}
	}
	return idx
}

// buildTiers groups the resolved agents by the verbatim recommended tier string, as the
// single-root loader does; a shadowed agent does not count toward its tier.
func buildTiers(agentIdx map[string]domain.Agent) []domain.TierInfo {
	type accum struct {
		rationale string
		keys      []string
	}
	byTier := make(map[string]*accum)
	for _, a := range agentIdx {
		tier := string(a.RecommendedTier)
		if tier == "" {
			continue
		}
		if byTier[tier] == nil {
			byTier[tier] = &accum{}
		}
		acc := byTier[tier]
		if acc.rationale == "" && a.TierRationale != "" {
			acc.rationale = a.TierRationale
		}
		acc.keys = append(acc.keys, a.Key)
	}
	result := make([]domain.TierInfo, 0, len(byTier))
	for tier, acc := range byTier {
		sort.Strings(acc.keys)
		result = append(result, domain.TierInfo{Tier: domain.Tier(tier), Rationale: acc.rationale, AgentKeys: acc.keys})
	}
	sort.Slice(result, func(i, j int) bool { return string(result[i].Tier) < string(result[j].Tier) })
	return result
}
