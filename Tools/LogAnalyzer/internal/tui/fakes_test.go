package tui

// fakes_test.go provides minimal fake domain port implementations used by
// navigation tests that require a real *app.Service instance.

import (
	"context"
	"time"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// tuiFakePricingStore is a minimal in-memory PricingStore for TUI tests.
type tuiFakePricingStore struct {
	path string
	// global is the global file ("" = tuiFakeGlobalPricingPath); the Exists fields
	// say which files exist; globalUnavailable makes the global level unusable.
	global            string
	globalExists      bool
	projectExists     bool
	globalUnavailable bool
	// puts lists the level of every save.
	puts []domain.ConfigLevel
	// globalCurrency and projectCurrency are the currencies the files state ("" = none).
	globalCurrency  domain.CurrencyCode
	projectCurrency domain.CurrencyCode
}

// tuiFakeGlobalPricingPath is the global pricing file of tuiFakePricingStore.
const tuiFakeGlobalPricingPath = "/test/global/pricing.yaml"

func (f *tuiFakePricingStore) Path() string { return f.path }

func (f *tuiFakePricingStore) Paths() domain.ConfigPaths {
	global := domain.ConfigFile{Level: domain.LevelGlobal, Path: f.global, Exists: f.globalExists, Currency: f.globalCurrency}
	if global.Path == "" {
		global.Path = tuiFakeGlobalPricingPath
	}
	if f.globalUnavailable {
		global = domain.ConfigFile{Level: domain.LevelGlobal, Unavailable: "user config folder not found"}
	}
	return domain.ConfigPaths{
		Global:  global,
		Project: domain.ConfigFile{Level: domain.LevelProject, Path: f.path, Exists: f.projectExists, Currency: f.projectCurrency},
	}
}

func (f *tuiFakePricingStore) Load(_ context.Context) (domain.PricingTable, []domain.Finding, error) {
	return domain.EmptyPricingTable(), nil, nil
}

func (f *tuiFakePricingStore) Put(_ context.Context, level domain.ConfigLevel, _ domain.ModelPricing) (domain.ConfigFile, error) {
	f.puts = append(f.puts, level)
	file := f.Paths().Project
	if level == domain.LevelGlobal {
		file = f.Paths().Global
	}
	file.Exists = true
	return file, nil
}

// tuiFakeLogSource is a minimal LogSource that always reports SourceNotFound.
type tuiFakeLogSource struct{}

func (f *tuiFakeLogSource) Default(_ string) domain.Source {
	return domain.Source{Kind: domain.SourceNotFound}
}

func (f *tuiFakeLogSource) Classify(_ string) domain.Source {
	return domain.Source{Kind: domain.SourceNotFound}
}

func (f *tuiFakeLogSource) Enumerate(src domain.Source) (domain.Inventory, []domain.Finding) {
	return domain.Inventory{Source: src}, nil
}

// tuiFakeEventReader is a minimal EventReader that delivers no events.
type tuiFakeEventReader struct{}

func (f *tuiFakeEventReader) Read(_ context.Context, _ string, _ func(domain.Event) error) ([]domain.Finding, error) {
	return nil, nil
}

// tuiFakeClock returns a fixed point in time.
type tuiFakeClock struct{}

func (f *tuiFakeClock) Now() time.Time {
	return time.Date(2026, 8, 2, 7, 46, 35, 0, time.UTC)
}

// tuiFakeInteraction is a minimal Interaction that always skips without blocking.
//
// It answers the configuration-level question with levelOption ("" = the project
// option) or skips it when skipLevel is set, and records the question.
type tuiFakeInteraction struct {
	levelOption    string
	skipLevel      bool
	levelQuestions []interaction.ChoiceQuestion
}

func (f *tuiFakeInteraction) SelectOne(_ context.Context, q interaction.ChoiceQuestion) (interaction.ChoiceAnswer, error) {
	if q.ID != app.QuestionConfigLevel {
		return interaction.ChoiceAnswer{Status: interaction.SkippedOne}, nil
	}
	f.levelQuestions = append(f.levelQuestions, q)
	if f.skipLevel {
		return interaction.ChoiceAnswer{Status: interaction.SkippedOne}, nil
	}
	option := f.levelOption
	if option == "" {
		option = "project"
	}
	return interaction.ChoiceAnswer{Status: interaction.Answered, OptionID: option}, nil
}

func (f *tuiFakeInteraction) SelectMany(_ context.Context, _ interaction.ChoiceQuestion) (interaction.MultiChoiceAnswer, error) {
	return interaction.MultiChoiceAnswer{Status: interaction.SkippedOne}, nil
}

func (f *tuiFakeInteraction) AskText(_ context.Context, _ interaction.TextQuestion) (interaction.TextAnswer, error) {
	return interaction.TextAnswer{Status: interaction.SkippedOne}, nil
}

func (f *tuiFakeInteraction) Confirm(_ context.Context, _ interaction.Question) (interaction.ConfirmAnswer, error) {
	return interaction.ConfirmAnswer{Status: interaction.SkippedOne}, nil
}

func (f *tuiFakeInteraction) Notify(_ context.Context, _ interaction.Notice)          {}
func (f *tuiFakeInteraction) Progress(_ context.Context, _ interaction.ProgressEvent) {}

// newServiceWithPricingPath returns an *app.Service whose PricingPath() returns
// pricingPath. Intended for navigation tests that need a non-nil Service
// to call PricingPath() without panicking; not intended for full analysis flows.
func newServiceWithPricingPath(pricingPath string) *app.Service {
	return app.New(app.Deps{
		Source:      &tuiFakeLogSource{},
		Reader:      &tuiFakeEventReader{},
		Pricing:     &tuiFakePricingStore{path: pricingPath},
		Clock:       &tuiFakeClock{},
		Interaction: &tuiFakeInteraction{},
	})
}
