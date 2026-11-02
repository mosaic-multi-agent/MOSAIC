package app_test

// Shared fixtures for the provider-aware pricing-gap and pricing-edit tests.
//
// The scenario logs one model, "shared-model", used by one agent per entry of
// the supplied provider list ("" = a record that logged no provider). Every
// agent carries a single invocation_end with the same token usage.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

const providerGapModel = domain.ModelID("shared-model")

// providerGapAnalysis analyses the scenario and returns the service (wired with
// the given interaction), the analysis result and the pricing store, which is
// preloaded with the given entries.
func providerGapAnalysis(
	t *testing.T,
	ia domain.Interaction,
	providers []domain.ProviderID,
	preloaded ...domain.ModelPricing,
) (*app.Service, app.Result, *fakePricingStore) {
	t.Helper()
	return providerGapAnalysisWith(t, ia, providers, nil, preloaded...)
}

// providerGapAnalysisWith is providerGapAnalysis with a setup hook that configures
// the pricing store (currencies, table) before the analysis loads it.
func providerGapAnalysisWith(
	t *testing.T,
	ia domain.Interaction,
	providers []domain.ProviderID,
	setup func(*fakePricingStore),
	preloaded ...domain.ModelPricing,
) (*app.Service, app.Result, *fakePricingStore) {
	t.Helper()

	src := domain.Source{Kind: domain.SourceLogsRoot, Path: "/test-logs"}
	runDir := "/test-logs/20260101T000000Z-abcd"
	reader := newFakeEventReader()
	reader.addEvents(appTestOrchFile, appTestRunEndEvent())

	var agents []domain.AgentEntry
	for i, p := range providers {
		id := domain.AgentInstanceID(fmt.Sprintf("GapAgent#%d", i+1))
		file := fmt.Sprintf("%s/%s/03_events.jsonl", runDir, id)
		ev := appTestInvEndEvent(id, providerGapModel, domain.TokenUsage{
			Input:  domain.Tokens(1_000_000),
			Output: domain.Tokens(500_000),
		})
		ev.InvocationEnd.Provider = p
		reader.addEvents(file, ev)
		agents = append(agents, domain.AgentEntry{
			Dir: runDir + "/" + string(id), InstanceHint: id, EventFile: file,
		})
	}

	logSource := &fakeLogSource{
		classifyFunc: func(_ string) domain.Source { return src },
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			return domain.Inventory{Source: s, Runs: []domain.RunEntry{{
				Run: appTestRunRef, Dir: runDir, OrchestratorFile: appTestOrchFile, Agents: agents,
			}}}, nil
		},
	}

	store := newFakePricingStore()
	if setup != nil {
		setup(store)
	}
	for _, e := range preloaded {
		store.table = store.table.With(e)
	}
	deps := app.Deps{
		Source: logSource, Reader: reader, Pricing: store, Clock: newFakeClock(),
		Interaction: &alwaysSkippingInteraction{}, WorkDir: "/workdir",
	}

	result, err := app.New(deps).Analyze(context.Background(), app.Request{ExplicitPath: "/test-logs"})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Outcome != app.OutcomeReport {
		t.Fatalf("Outcome = %v, want OutcomeReport", result.Outcome)
	}
	deps.Interaction = ia
	return app.New(deps), result, store
}

// providerPricing is a flat price for the scenario model under a provider.
func providerPricing(provider domain.ProviderID) domain.ModelPricing {
	p := flatPricing(providerGapModel)
	p.Provider = provider
	return p
}

// enteringInteraction answers the gap flow with "enter prices". Rate questions
// are answered "3.00" when accept(subject) is true and skipped otherwise; the
// threshold question is answered empty.
func enteringInteraction(accept func(subject string) bool) *scriptedInteraction {
	return &scriptedInteraction{
		onSelectOne: func(q interaction.ChoiceQuestion) interaction.ChoiceAnswer {
			if q.ID == app.QuestionPricingAction {
				return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: app.PricingActionEnter}
			}
			return interaction.ChoiceAnswer{Status: interaction.SkippedOne}
		},
		onAskText: func(q interaction.TextQuestion) interaction.TextAnswer {
			if q.ID == app.QuestionPricingRate && accept(q.Subject) {
				return interaction.TextAnswer{Status: interaction.Answered, Text: "3.00"}
			}
			if q.ID == app.QuestionPricingThreshold {
				return interaction.TextAnswer{Status: interaction.Answered, Text: ""}
			}
			return interaction.TextAnswer{Status: interaction.SkippedOne}
		},
	}
}

// subjectOf reports whether a rate/threshold subject belongs to the gap whose
// printed key is key ("model" or "model [provider]").
func subjectOf(subject, key string) bool {
	return subject == key || strings.HasPrefix(subject, key+":")
}

// unpricedLabels prints the report-wide unpriced list.
func unpricedLabels(r domain.Report) []string {
	out := make([]string, 0, len(r.UnpricedModels))
	for _, k := range r.UnpricedModels {
		out = append(out, fmt.Sprint(k))
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// putProviders lists the provider of every entry handed to Put, in order.
func putProviders(s *fakePricingStore) []domain.ProviderID {
	out := make([]domain.ProviderID, 0, len(s.putCalls))
	for _, e := range s.putCalls {
		out = append(out, e.Provider)
	}
	return out
}
