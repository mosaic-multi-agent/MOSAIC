package app

import (
	"context"
	"strconv"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/domain"
)

// PricingGap describes one (provider, model) the report could not price.
type PricingGap struct {
	// Key is the unpriced (provider, model); an empty provider is a record
	// that logged none (top-level models section).
	Key domain.PricingKey
	// Model is Key.Model.
	Model domain.ModelID
	// Occurrences is how many actor reports listed this key as unpriced,
	// useful for display ordering.
	Occurrences int
}

// PricingGaps lists the unpriced keys in a report, in the order they appear
// in Report.UnpricedModels (by model id, then provider). An empty slice is
// returned when the report has no unpriced models.
func PricingGaps(r domain.Report) []PricingGap {
	if len(r.UnpricedModels) == 0 {
		return nil
	}

	// Count how many actor reports list each key as unpriced.
	occurrences := make(map[domain.PricingKey]int)
	count := func(actor domain.ActorReport) {
		for _, k := range actor.UnpricedModels {
			occurrences[k]++
		}
	}
	countRun := func(run domain.RunReport) {
		count(run.Orchestrator)
		for _, agent := range run.Agents {
			count(agent)
		}
	}
	for _, run := range r.Runs {
		countRun(run)
	}
	if r.Unattributable != nil {
		countRun(*r.Unattributable)
	}

	gaps := make([]PricingGap, len(r.UnpricedModels))
	for i, key := range r.UnpricedModels {
		gaps[i] = PricingGap{Key: key, Model: key.Model, Occurrences: occurrences[key]}
	}
	return gaps
}

// ResolvePricingGaps offers the user, through the Interaction port, either to
// enter the missing prices or to be shown the config file path.
//
// Binding rules:
//   - Entered prices are persisted AUTOMATICALLY via PricingStore.Put, with no
//     separate confirmation step, and the file actually written is reported back through
//     Interaction.Notify.
//   - Declining, skipping or cancelling is a normal outcome: token totals remain
//     complete and money stays explicitly MoneyUnpriced.
//   - The report is re-priced from the Session, never by re-reading logs.
//   - This function must complete without blocking when given a skipping
//     Interaction implementation.
func (s *Service) ResolvePricingGaps(ctx context.Context, sess *Session, r domain.Report) (domain.Report, error) {
	report, _, err := s.resolvePricingGapsWritten(ctx, sess, r)
	return report, err
}

// resolvePricingGapsWritten is ResolvePricingGaps that also returns the path of
// the last file the flow wrote ("" when it saved nothing).
func (s *Service) resolvePricingGapsWritten(ctx context.Context, sess *Session, r domain.Report) (domain.Report, string, error) {
	gaps := PricingGaps(r)
	if len(gaps) == 0 {
		return r, "", nil
	}

	// Ask the user what to do about unpriced models.
	actionAns, err := s.deps.Interaction.SelectOne(ctx, interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:     QuestionPricingAction,
			Title:  "Missing prices",
			Prompt: "Some models have no pricing entry. What would you like to do?",
		},
		Options: []interaction.Option{
			{ID: PricingActionEnter, Label: "Enter prices now"},
			{ID: PricingActionShowPath, Label: "Show config file path"},
			{ID: PricingActionSkip, Label: "Skip (leave unpriced)"},
		},
	})
	if err != nil {
		return r, "", err
	}

	// Skipped or cancelled - return the original report unchanged (complete tokens,
	// explicit MoneyUnpriced money cells).
	if actionAns.Status != interaction.Answered {
		return r, "", nil
	}

	switch actionAns.OptionID {
	case PricingActionShowPath:
		s.deps.Interaction.Notify(ctx, interaction.Notice{
			Level:   interaction.NoticeInfo,
			Title:   "Pricing config",
			Message: "Edit this file to add model prices: " + s.deps.Pricing.Path(),
		})
		return r, "", nil

	case PricingActionEnter:
		return s.enterGapPrices(ctx, sess, r, gaps)

	default: // PricingActionSkip or any unrecognised option
		return r, "", nil
	}
}

// enterGapPrices asks the level once, then the rates of every gap, saving each
// valid entry at that level. It returns the re-priced report and the file written.
func (s *Service) enterGapPrices(ctx context.Context, sess *Session, r domain.Report, gaps []PricingGap) (domain.Report, string, error) {
	level, ok, err := s.askConfigLevel(ctx, s.deps.Pricing.Paths())
	if err != nil || !ok {
		return r, "", err
	}
	written := ""
	for _, gap := range gaps {
		entry, ok, err := s.collectPricingEntry(ctx, gap.Key, s.fileCurrency(level, sess.Table().Currency()))
		if err != nil {
			return r, written, err
		}
		if !ok {
			continue
		}
		file, err := s.SavePricing(ctx, sess, level, entry)
		if err != nil {
			return r, written, err
		}
		s.notifySaved(ctx, gap.Key, level, file, sess.Table().Currency())
		written = file.Path
	}
	if written == "" {
		return r, "", nil
	}
	// Re-price from the cached aggregate in the session - logs are not re-read.
	repriced := sess.Reprice()
	repriced.Quality = r.Quality // preserve the quality summary from the original analysis
	return repriced, written, nil
}

// collectPricingEntry asks the user for pricing rates for a single (provider,
// model) key via
// the Interaction port, naming the currency cur of the rates: the five standard rates, then an optional threshold that
// opens a long-context block with its own five rates. Returns (entry, true, nil)
// when the answers form a valid ModelPricing; returns (zero, false, nil) when the
// user skipped all standard rate questions or the resulting entry fails validation.
func (s *Service) collectPricingEntry(ctx context.Context, key domain.PricingKey, cur domain.CurrencyCode) (domain.ModelPricing, bool, error) {
	entry := domain.ModelPricing{Provider: key.Provider, Model: key.Model}

	answered, err := s.askRateSet(ctx, key, cur, "", &entry.Standard)
	if err != nil {
		return domain.ModelPricing{}, false, err
	}

	// Ask for the optional long-context threshold.
	thAns, err := s.deps.Interaction.AskText(ctx, interaction.TextQuestion{
		Question: interaction.Question{
			ID:      QuestionPricingThreshold,
			Subject: key.String(),
			Title:   "Long-context threshold",
			Prompt:  "Enter the token count above which long-context rates apply (empty = no long-context tier):",
		},
	})
	if err != nil {
		return domain.ModelPricing{}, false, err
	}
	if thAns.Status == interaction.Answered && thAns.Text != "" {
		threshold, parseErr := strconv.ParseInt(thAns.Text, 10, 64)
		if parseErr == nil && threshold > 0 {
			entry.LongContext.Threshold = threshold
			entry.HasLongContext = true
			if _, err := s.askRateSet(ctx, key, cur, LongContextFieldPrefix, &entry.LongContext.Rates); err != nil {
				return domain.ModelPricing{}, false, err
			}
		}
	}

	if answered == 0 {
		return domain.ModelPricing{}, false, nil
	}

	// Only persist entries that satisfy the domain validation rules.
	if err := entry.Validate(); err != nil {
		return domain.ModelPricing{}, false, nil
	}

	return entry, true, nil
}

// askRateSet asks the five rate questions of one rate set, storing each answered
// rate into dst, and returns how many were answered. The prefix is empty for the
// standard set and LongContextFieldPrefix for the long-context set.
func (s *Service) askRateSet(ctx context.Context, key domain.PricingKey, cur domain.CurrencyCode, prefix string, dst *domain.RateSet) (int, error) {
	fields := []struct {
		name string
		set  *domain.Rate
	}{
		{RateFieldInput, &dst.Input},
		{RateFieldCacheRead, &dst.CacheRead},
		{RateFieldCacheWrite5m, &dst.CacheWrite5m},
		{RateFieldCacheWrite1h, &dst.CacheWrite1h},
		{RateFieldOutput, &dst.Output},
	}

	answered := 0
	for _, f := range fields {
		name := prefix + f.name
		ans, err := s.deps.Interaction.AskText(ctx, interaction.TextQuestion{
			Question: interaction.Question{
				ID:      QuestionPricingRate,
				Subject: key.String() + ":" + name,
				Title:   name,
				Prompt:  string(cur) + " per million tokens for " + key.String() + " " + name + ":",
			},
			Validate: func(v string) error {
				_, verr := domain.ParseRate(v)
				return verr
			},
		})
		if err != nil {
			return answered, err
		}
		if ans.Status != interaction.Answered {
			continue
		}
		r, err := domain.ParseRate(ans.Text)
		if err != nil {
			continue
		}
		*f.set = r
		answered++
	}
	return answered, nil
}

// SavePricing persists one entry at level via the PricingStore and folds it into
// the session's pricing table so a subsequent Reprice call reflects the new entry
// without reloading the file. A global entry never replaces a project entry for
// the same key, matching the merged table a reload would produce. It returns the
// file written.
func (s *Service) SavePricing(ctx context.Context, sess *Session, level domain.ConfigLevel, entry domain.ModelPricing) (domain.ConfigFile, error) {
	file, err := s.deps.Pricing.Put(ctx, level, entry)
	if err != nil {
		return domain.ConfigFile{}, err
	}
	entry.Source = level
	if existing, found := sess.table.LookupFor(entry.Provider, entry.Model); !(found && existing.Source == domain.LevelProject && level == domain.LevelGlobal) {
		sess.table = sess.table.With(entry)
	}
	return file, nil
}
