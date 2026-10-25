package app

import (
	"context"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/domain"
)

// ModelPricingStatus pairs one model appearing in a report with its current
// state in the pricing table.
type ModelPricingStatus struct {
	// Key is the (provider, model) as it appears in the report; an empty
	// provider is a record that logged none.
	Key domain.PricingKey
	// Model is Key.Model.
	Model domain.ModelID
	// Priced reports whether the pricing table has an entry for Key (strict
	// lookup: a top-level entry does not price a provider record).
	Priced bool
	// Pricing holds the current rates. Meaningful only when Priced is true;
	// it is the zero value otherwise.
	Pricing domain.ModelPricing
}

// PricingEditOutcome classifies how a pricing-edit flow ended.
type PricingEditOutcome uint8

const (
	// PricingEditCancelled: no model was selected (skipped, cancelled, empty model
	// id, nil session, or no models in the report). Nothing was asked beyond the
	// selection and nothing was persisted.
	PricingEditCancelled PricingEditOutcome = iota
	// PricingEditNoChange: a model was selected but no valid entry resulted (every
	// rate question skipped, or the collected entry failed validation). Nothing was
	// persisted; any pre-existing price is untouched.
	PricingEditNoChange
	// PricingEditSaved: an entry was persisted and the report was re-priced.
	PricingEditSaved
)

// PricingEditResult is what the pricing-editor entry points return.
type PricingEditResult struct {
	// Outcome classifies the end state.
	Outcome PricingEditOutcome
	// Key is the (provider, model) the user chose. Zero when Outcome is
	// PricingEditCancelled.
	Key domain.PricingKey
	// Model is Key.Model. Empty when Outcome is PricingEditCancelled.
	Model domain.ModelID
	// Report is the re-priced report when Outcome is PricingEditSaved, and the
	// caller's original report otherwise. Never a zero Report.
	Report domain.Report
	// ConfigPath is the pricing config file the entry was written to.
	// Set only when Outcome is PricingEditSaved.
	ConfigPath string
}


// ReportModels returns every distinct (provider, model) that appears anywhere
// in the report, sorted by model id then provider ("" first). An actor report
// that carries no Keys contributes its Models as provider-less keys.
func ReportModels(r domain.Report) []domain.PricingKey {
	seen := make(map[domain.PricingKey]bool)
	collect := func(actor domain.ActorReport) {
		for _, k := range actor.Keys {
			seen[k] = true
		}
		if len(actor.Keys) == 0 {
			for _, m := range actor.Models {
				seen[domain.PricingKey{Model: m}] = true
			}
		}
	}
	collectRun := func(run domain.RunReport) {
		collect(run.Orchestrator)
		for _, agent := range run.Agents {
			collect(agent)
		}
	}
	for _, run := range r.Runs {
		collectRun(run)
	}
	if r.Unattributable != nil {
		collectRun(*r.Unattributable)
	}
	return domain.SortedPricingKeys(seen)
}

// ModelPricingStates pairs each (provider, model) in the report with its
// current state in the supplied pricing table, preserving ReportModels ordering,
// so a frontend can render the list without re-deriving priced/unpriced status.
func ModelPricingStates(r domain.Report, t domain.PricingTable) []ModelPricingStatus {
	keys := ReportModels(r)
	if len(keys) == 0 {
		return nil
	}

	states := make([]ModelPricingStatus, len(keys))
	for i, k := range keys {
		states[i] = ModelPricingStatus{Key: k, Model: k.Model}
		if pricing, ok := t.LookupFor(k.Provider, k.Model); ok {
			states[i].Priced = true
			states[i].Pricing = pricing
		}
	}
	return states
}


// EditModelPricing sets or overwrites the price of one already-chosen
// (provider, model): the entry is saved under key.Provider, or in the top-level
// models section when the key has no provider. An empty key.Model or a nil
// session cancels. The binding rules above apply.
func (s *Service) EditModelPricing(ctx context.Context, sess *Session, r domain.Report, key domain.PricingKey) (PricingEditResult, error) {
	if key.Model == "" || sess == nil {
		return PricingEditResult{Outcome: PricingEditCancelled, Report: r}, nil
	}

	entry, ok, err := s.collectPricingEntry(ctx, key)
	if err != nil {
		return PricingEditResult{}, err
	}
	if !ok {
		return PricingEditResult{Outcome: PricingEditNoChange, Report: r}, nil
	}

	if err := s.SavePricing(ctx, sess, entry); err != nil {
		return PricingEditResult{}, err
	}

	s.deps.Interaction.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeInfo,
		Title:   "Saved",
		Message: "Pricing for " + key.String() + " saved to " + s.deps.Pricing.Path(),
	})

	repriced := sess.Reprice()
	repriced.Quality = r.Quality

	return PricingEditResult{
		Outcome:    PricingEditSaved,
		Key:        key,
		Model:      key.Model,
		Report:     repriced,
		ConfigPath: s.deps.Pricing.Path(),
	}, nil
}

// EditPricing is the frontend-agnostic entry point: it asks which (provider,
// model) to price through the Interaction port (QuestionPricingModel, a
// SelectOne over every key returned by ReportModels; the option id is the key's
// printed form, "model [provider]" or the bare model id without provider),
// then delegates to EditModelPricing.
//
// Binding rules:
//   - A skipped or cancelled selection returns PricingEditCancelled with the
//     report unchanged and nothing persisted.
//   - An unrecognised OptionID is treated as a cancel.
//   - A report with no models asks nothing and returns PricingEditCancelled.
//   - Must complete without blocking under a skipping Interaction.
func (s *Service) EditPricing(ctx context.Context, sess *Session, r domain.Report) (PricingEditResult, error) {
	keys := ReportModels(r)
	if len(keys) == 0 {
		return PricingEditResult{Outcome: PricingEditCancelled, Report: r}, nil
	}

	// Determine the pricing table to use for labelling options (priced/unpriced).
	var table domain.PricingTable
	if sess != nil {
		table = sess.Table()
	} else {
		table = domain.EmptyPricingTable()
	}

	// Build one SelectOne option per key, with a label that distinguishes
	// priced from unpriced (strict lookup) so a user can tell them apart.
	options := make([]interaction.Option, len(keys))
	byID := make(map[string]domain.PricingKey, len(keys))
	for i, k := range keys {
		_, priced := table.LookupFor(k.Provider, k.Model)
		label := k.String()
		if priced {
			label += " (priced)"
		} else {
			label += " (unpriced)"
		}
		options[i] = interaction.Option{ID: k.String(), Label: label}
		byID[k.String()] = k
	}

	ans, err := s.deps.Interaction.SelectOne(ctx, interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:     QuestionPricingModel,
			Title:  "Select model to price",
			Prompt: "Which model would you like to set or overwrite a price for?",
		},
		Options: options,
	})
	if err != nil {
		return PricingEditResult{}, err
	}

	if ans.Status != interaction.Answered {
		return PricingEditResult{Outcome: PricingEditCancelled, Report: r}, nil
	}

	// Reject unrecognised option ids - treat them as a cancel.
	chosen, valid := byID[ans.OptionID]
	if !valid {
		return PricingEditResult{Outcome: PricingEditCancelled, Report: r}, nil
	}

	return s.EditModelPricing(ctx, sess, r, chosen)
}
