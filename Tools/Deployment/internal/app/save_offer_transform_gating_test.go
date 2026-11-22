package app_test

// save_offer_transform_gating_test.go specifies where the transform makes no save offer (target
// harnesses whose configuration entries cannot stop the question) and that a saved answer takes
// effect for a later transform on the same service.

import (
	"errors"
	"maps"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/catalog/stack/injections"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/registry"
)

// trTierModule reports another provision tier than its wrapped module.
type trTierModule struct {
	domain.HarnessModule
	tier domain.ProvisionTier
}

func (m trTierModule) Ref() domain.HarnessRef {
	ref := m.HarnessModule.Ref()
	ref.Tier = m.tier
	return ref
}

// trTargetShape makes the target harness declare the tier and, unless noForms, MCP name forms.
func trTargetShape(tier domain.ProvisionTier, noForms bool) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		base := reg.modules[hOpen]
		desc := *base.Descriptor()
		desc.Tools.MCP = domain.MCPNameForms{}
		if !noForms {
			desc.Tools.MCP = domain.MCPNameForms{ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}"}
		}
		reg.modules[hOpen] = trTierModule{HarnessModule: trModelCatalogModule{HarnessModule: base, desc: desc}, tier: tier}
	}
}

func TestSaveOffer_Transform_OfferOnlyForTargetsWhoseConfigEntriesApply(t *testing.T) {
	cases := []struct {
		name    string
		tier    domain.ProvisionTier
		noForms bool
		offer   bool
	}{
		{"builtin with MCP forms", domain.TierBuiltin, false, true},
		{"descriptor with MCP forms", domain.TierDescriptor, false, true},
		{"external", domain.TierExternal, false, false},
		{"builtin without MCP forms", domain.TierBuiltin, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newTrOffer(t, listFlowAnswer, offerSetup{}, trTargetShape(c.tier, c.noForms))

			r.mustRun(r.request())

			if got := len(r.offer.offers) == 1; got != c.offer {
				t.Errorf("offer made = %v (%d offers), want %v", got, len(r.offer.offers), c.offer)
			}
			if got := len(r.user.puts) == 1; got != c.offer {
				t.Errorf("entry saved = %v (%d puts), want %v", got, len(r.user.puts), c.offer)
			}
		})
	}
}

// trRebuildRegistry is a registry that implements registry.Rebuilder: Rebuild returns a registry
// from rebuilt(), which models a registry built again from the saved configuration.
type trRebuildRegistry struct {
	registry.Registry
	state   *rebuildState
	rebuilt func() registry.Registry
}

func (r *trRebuildRegistry) RebuildForStack(injections.Result) (registry.Registry, error) {
	return r, nil
}

func (r *trRebuildRegistry) Rebuild() (registry.Registry, error) {
	r.state.rebuilds++
	if r.state.err != nil {
		return nil, r.state.err
	}
	return &trRebuildRegistry{Registry: r.rebuilt(), state: r.state, rebuilt: r.rebuilt}, nil
}

// trRebuildable makes the registry rebuildable: the rebuilt registry maps the generic on the target
// (as a saved tool_destinations entry does), the first one does not.
func trRebuildable(t *testing.T, state *rebuildState) func(*app.Deps) {
	return func(d *app.Deps) {
		base := d.Registry.(*stubRegistry)
		d.Registry = &trRebuildRegistry{Registry: base, state: state, rebuilt: func() registry.Registry {
			fresh := &stubRegistry{list: base.list, modules: maps.Clone(base.modules)}
			fresh.modules[hOpen] = rbModule(t, hOpen)
			return fresh
		}}
	}
}

func TestSaveOffer_Transform_SameProcess_SavedAnswerIsNotAskedAgainByTheSameService(t *testing.T) {
	state := &rebuildState{}
	r := newTrOffer(t, listFlowAnswer, offerSetup{}, trRebuildable(t, state))
	r.mustRun(r.request())
	again := r.request()
	again.Overwrite = true // the destination of the first run exists

	r.mustRun(again)

	if got := r.spy.Stub.CountAsked(domain.QCustomTool, trUnmappedGeneric); got != 1 {
		t.Errorf("the custom tool question was asked %d times over two transforms, want once", got)
	}
	if len(r.offer.offers) != 1 {
		t.Errorf("offers = %d, want one: a saved generic is not asked, so not offered again", len(r.offer.offers))
	}
	if state.rebuilds != 1 {
		t.Errorf("registry rebuilt %d times, want once after the save", state.rebuilds)
	}
}

func TestSaveOffer_Transform_SameProcess_FailedRebuildIsAWarningAndTheTransformSucceeds(t *testing.T) {
	state := &rebuildState{err: errors.New("rebuild backend exploded")}
	r := newTrOffer(t, listFlowAnswer, offerSetup{}, trRebuildable(t, state))

	r.mustRun(r.request())

	found := false
	for _, n := range noticesAt(r.spy.Stub.Notices(), domain.NoticeWarning) {
		if noticeMentions(n, "rebuild backend exploded") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning names the rebuild error: %+v", r.spy.Stub.Notices())
	}
}

func TestSaveOffer_Transform_NoOfferWhenTheQuestionIsSuppressed(t *testing.T) {
	r := newTrOffer(t, listFlowAnswer, offerSetup{})
	req := r.request()
	req.SkipAll = map[domain.QuestionID]bool{domain.QCustomTool: true}

	r.mustRun(req)

	if len(r.offer.offers) != 0 || len(r.user.puts) != 0 {
		t.Errorf("offers=%d puts=%d, want none when the custom tool question is suppressed", len(r.offer.offers), len(r.user.puts))
	}
}
