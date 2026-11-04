package app

import (
	"context"

	"mosaic-log-analyzer/internal/analysis"
	"mosaic-log-analyzer/internal/domain"
)

// Deps is the injected dependency set. The composition root constructs every
// field; the service constructs no infrastructure of its own.
type Deps struct {
	Source      domain.LogSource
	Reader      domain.EventReader
	Pricing     domain.PricingStore
	Settings    domain.SettingsStore // nil = built-in defaults, no findings, SettingsPath() == ""
	Digests     domain.DigestStore   // nil = no digest support (Analyze is logs-only)
	Clock       domain.Clock
	Interaction domain.Interaction
	WorkDir     string                  // used for default-location probing
	ToolVersion string                  // LogAnalyzer version recorded in exported digests
	Ingester    domain.UsageIngester    // nil = ingest-otel off
	IngestProbe domain.IngestStateProbe // nil = pending usage is never reported
	Alarms      domain.AlarmSource      // nil = adapter alarms are not read
}

// Service is the single use-case layer both frontends drive.
type Service struct {
	deps Deps
}

// New returns a Service wired with the given dependencies.
func New(d Deps) *Service {
	return &Service{deps: d}
}

// Request parameterises an analysis.
type Request struct {
	// ExplicitPath is a flag/argument-supplied logs root or single run folder.
	ExplicitPath string
	// RunID restricts the analysis to one named run; "" analyses everything.
	RunID string
	// ResolvePricingGaps enables the interactive missing-pricing flow. The CLI
	// may set it true safely: its Interaction never blocks and simply skips.
	ResolvePricingGaps bool
}

// Outcome is the coarse result classification frontends branch on.
type Outcome uint8

const (
	OutcomeReport         Outcome = iota // a report is present (possibly with findings)
	OutcomeNoData                        // usable source, nothing in it
	OutcomeSourceNotFound                // no source could be resolved; NOT an error
	OutcomeSourceUnusable                // a path was supplied but is not a logs tree
)

// Result carries everything a frontend needs to render.
type Result struct {
	Outcome Outcome
	Source  domain.Source
	Report  domain.Report // zero value unless Outcome == OutcomeReport
	// Session lets the caller re-price without re-reading logs.
	Session *Session
	// GapConfigPath is the file the pricing-gap flow wrote ("" = it saved nothing).
	GapConfigPath string
}

// PricingPath returns the config file path from the injected pricing store.
// Used by the CLI's "pricing path" subcommand to display the file location.
func (s *Service) PricingPath() string {
	return s.deps.Pricing.Path()
}

// Analyze runs the whole flow: resolve source, load pricing, enumerate, read,
// aggregate, price, optionally resolve pricing gaps, assemble the report.
//
// Binding flow rules:
//  1. Source resolution order: ExplicitPath -> default OrchestrationLogs/ under
//     WorkDir -> prompt via Interaction. A missing source is NEVER a hard failure;
//     a declined, skipped or cancelled prompt yields OutcomeSourceNotFound.
//  2. A usable but empty source yields OutcomeNoData, not an error.
//  3. Every returned Report carries its QualitySummary, so a frontend cannot
//     present partial data as complete.
//  4. An error is returned only for genuine infrastructure failure (a malformed
//     pricing document, or a context cancellation).
func (s *Service) Analyze(ctx context.Context, req Request) (Result, error) {
	// Resolve source following explicit → default → prompt order.
	src, digestFiles, outcome, err := s.resolveWith(ctx, req, s.digestResolver())
	if err != nil {
		return Result{}, err
	}
	if outcome != outcomeContinue {
		return Result{Outcome: outcome, Source: src}, nil
	}

	// Load pricing table. An absent file is not an error; a malformed document is.
	table, pricingFindings, err := s.deps.Pricing.Load(ctx)
	if err != nil {
		return Result{}, err
	}

	// Load analysis settings. An absent file is not an error; an unparseable one is.
	settings, settingsFindings, err := s.loadSettings(ctx)
	if err != nil {
		return Result{}, err
	}

	loaded, err := s.readSource(ctx, src, digestFiles)
	if err != nil {
		return Result{}, err
	}

	// A structurally valid source with no run data is a named no-data outcome.
	if loaded.noData {
		return Result{
			Outcome: OutcomeNoData,
			Source:  src,
		}, nil
	}

	// Findings from infrastructure phases carry into the quality summary.
	carried := make([]domain.Finding, 0, len(pricingFindings)+len(settingsFindings)+len(loaded.findings))
	carried = append(carried, pricingFindings...)
	carried = append(carried, settingsFindings...)
	carried = append(carried, loaded.findings...)
	agg := loaded.aggregate

	report, priceFindings := analysis.Price(agg, table, settings, src)
	usageIngest := s.probeUsageIngest(loaded.inventory)
	applyUsageIngest(&report, usageIngest)
	carried = append(carried, priceFindings...)

	// Attach the complete quality summary, merging findings from every phase.
	report.Quality = domain.NewQualitySummary(carried)

	sess := &Session{
		aggregate:      agg,
		table:          table,
		settings:       settings,
		source:         src,
		exportFindings: loaded.exportFindings,
		usageIngest:    usageIngest,
	}

	result := Result{
		Outcome: OutcomeReport,
		Source:  src,
		Report:  report,
		Session: sess,
	}

	// Optionally run the interactive pricing-gap resolution flow.
	if req.ResolvePricingGaps && report.HasUnpricedModels() {
		updated, written, err := s.resolvePricingGapsWritten(ctx, sess, report)
		if err != nil {
			return Result{}, err
		}
		result.Report = updated
		result.GapConfigPath = written
	}

	return result, nil
}

// SettingsPath returns the settings file path from the injected settings store,
// or "" when none is configured.
func (s *Service) SettingsPath() string {
	if s.deps.Settings == nil {
		return ""
	}
	return s.deps.Settings.Path()
}

// loadSettings loads the analysis settings; a nil store yields the built-in
// defaults and no findings. An error means the settings document is unusable.
func (s *Service) loadSettings(ctx context.Context) (domain.AnalysisSettings, []domain.Finding, error) {
	if s.deps.Settings == nil {
		return domain.DefaultAnalysisSettings(), nil, nil
	}
	return s.deps.Settings.Load(ctx)
}
