package app

import (
	"context"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/domain"
)

// Option ids of QuestionConfigLevel.
const (
	configLevelGlobalOption  = "global"
	configLevelProjectOption = "project"
)

// askConfigLevel asks whether a save goes to the global file ("All projects") or
// the project file ("This project"), showing both files. When the global level is
// unavailable its option is disabled and the project option is the default.
// ok is false when the question was skipped or cancelled: nothing is saved then.
func (s *Service) askConfigLevel(ctx context.Context, paths domain.ConfigPaths) (domain.ConfigLevel, bool, error) {
	global := interaction.Option{
		ID:          configLevelGlobalOption,
		Label:       "All projects",
		Description: paths.Global.Path,
	}
	defaultOption := configLevelGlobalOption
	if paths.Global.Unavailable != "" {
		global.Disabled = true
		global.DisabledReason = paths.Global.Unavailable
		defaultOption = configLevelProjectOption
	}
	project := interaction.Option{
		ID:          configLevelProjectOption,
		Label:       "This project",
		Description: paths.Project.Path,
	}

	ans, err := s.deps.Interaction.SelectOne(ctx, interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:              QuestionConfigLevel,
			Title:           "Where to save",
			Prompt:          "Save for all projects or for this project only?",
			DefaultOptionID: defaultOption,
			AllowSkip:       true,
		},
		Options: []interaction.Option{global, project},
	})
	if err != nil {
		return domain.LevelBuiltIn, false, err
	}
	if ans.Status != interaction.Answered {
		return domain.LevelBuiltIn, false, nil
	}
	switch ans.OptionID {
	case configLevelGlobalOption:
		if global.Disabled {
			return domain.LevelBuiltIn, false, nil
		}
		return domain.LevelGlobal, true, nil
	case configLevelProjectOption:
		return domain.LevelProject, true, nil
	default:
		return domain.LevelBuiltIn, false, nil
	}
}

// fileCurrency is the currency the pricing file of level states, else effective.
func (s *Service) fileCurrency(level domain.ConfigLevel, effective domain.CurrencyCode) domain.CurrencyCode {
	paths := s.deps.Pricing.Paths()
	file := paths.Project
	if level == domain.LevelGlobal {
		file = paths.Global
	}
	if file.Currency != "" {
		return file.Currency
	}
	return effective
}

// notifySaved reports a successful price save, naming the file actually written
// and its currency. A global save that the project file's different currency
// ignores says so.
func (s *Service) notifySaved(ctx context.Context, key domain.PricingKey, level domain.ConfigLevel, file domain.ConfigFile, effective domain.CurrencyCode) {
	msg := "Pricing for " + key.String() + " saved to " + file.Path +
		" (rates in " + string(s.fileCurrency(level, effective)) + ")"
	if level == domain.LevelGlobal {
		paths := s.deps.Pricing.Paths()
		if g, p := paths.Global.Currency, paths.Project.Currency; g != "" && p != "" && g != p {
			msg += "; ignored while the project file's currency is " + string(p)
		}
	}
	s.deps.Interaction.Notify(ctx, interaction.Notice{
		Level:   interaction.NoticeInfo,
		Title:   "Saved",
		Message: msg,
	})
}
