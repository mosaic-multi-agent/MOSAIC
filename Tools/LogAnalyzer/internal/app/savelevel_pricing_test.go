package app_test

// Tests for saving prices to a chosen configuration level, through the pricing-gap
// flow and the pricing-edit flow.
//
// Rules under test:
//   - Before any rate is asked, the flow asks which level to save to (all projects
//     or this project); the chosen level decides which file is written.
//   - The notice and the result name the file actually written.
//   - Skipping the level question saves nothing and asks no rate.
//   - When the global folder is unavailable the global option is disabled.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// resolveGap runs the pricing-gap flow over one unpriced model with the
// interaction entering "3.00" for every rate.
func resolveGap(t *testing.T, ia *scriptedInteraction) (domain.Report, *fakePricingStore) {
	t.Helper()
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{""})
	report, err := svc.ResolvePricingGaps(context.Background(), result.Session, result.Report)
	if err != nil {
		t.Fatalf("ResolvePricingGaps: %v", err)
	}
	return report, store
}

func TestResolvePricingGaps_AsksTheLevelBeforeAnyRate(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	resolveGap(t, ia)

	level := firstIndexOf(ia.asked, app.QuestionConfigLevel)
	rate := firstIndexOf(ia.asked, app.QuestionPricingRate)
	if level < 0 {
		t.Fatalf("the flow must ask %q; asked: %v", app.QuestionConfigLevel, ia.asked)
	}
	if rate < 0 || level > rate {
		t.Errorf("the level question (at %d) must come before the first rate question (at %d)", level, rate)
	}
}

func TestResolvePricingGaps_LevelQuestionOffersBothFilesWithGlobalAsDefault(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })

	_, store := resolveGap(t, ia)

	questions := levelQuestions(ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	assertLevelQuestionShape(t, questions[0], store.Paths())
}

func TestResolvePricingGaps_GlobalUnavailable_DisablesTheGlobalOption(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{""})
	store.globalUnavailable = true

	if _, err := svc.ResolvePricingGaps(context.Background(), result.Session, result.Report); err != nil {
		t.Fatalf("ResolvePricingGaps: %v", err)
	}

	questions := levelQuestions(ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	assertGlobalUnavailableShape(t, questions[0])
}

func TestResolvePricingGaps_TheChosenLevelDecidesTheFileWritten(t *testing.T) {
	cases := []struct {
		option string
		level  domain.ConfigLevel
	}{
		{"global", domain.LevelGlobal},
		{"project", domain.LevelProject},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			ia := enteringInteraction(func(string) bool { return true })
			ia.levelOption = c.option

			_, store := resolveGap(t, ia)

			if len(store.putLevels) != 1 || store.putLevels[0] != c.level {
				t.Errorf("saved at levels %v, want exactly [%v]", store.putLevels, c.level)
			}
		})
	}
}

func TestResolvePricingGaps_SaveNoticeNamesTheWrittenFile(t *testing.T) {
	cases := []struct {
		option, written, other string
	}{
		{"global", fakeGlobalPricingPath, "/test/pricing.yaml"},
		{"project", "/test/pricing.yaml", fakeGlobalPricingPath},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			ia := enteringInteraction(func(string) bool { return true })
			ia.levelOption = c.option

			resolveGap(t, ia)

			if !noticeContains(ia.notifyCalls, c.written) {
				t.Errorf("a notice must name the written file %q; notices: %+v", c.written, ia.notifyCalls)
			}
			if noticeContains(ia.notifyCalls, c.other) {
				t.Errorf("no notice may name the file %q that was not written; notices: %+v", c.other, ia.notifyCalls)
			}
		})
	}
}

func TestResolvePricingGaps_GlobalSave_ReportReflectsTheMergedResult(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })
	ia.levelOption = "global"

	report, store := resolveGap(t, ia)
	if len(store.putLevels) != 1 || store.putLevels[0] != domain.LevelGlobal {
		t.Fatalf("saved at levels %v, want [global]", store.putLevels)
	}

	if report.HasUnpricedModels() {
		t.Errorf("the model must be priced after the global save; unpriced: %v", report.UnpricedModels)
	}
}

func TestResolvePricingGaps_SkippedLevel_SavesNothingAndAsksNoRate(t *testing.T) {
	ia := enteringInteraction(func(string) bool { return true })
	ia.skipLevel = true

	report, store := resolveGap(t, ia)

	if len(store.putCalls) != 0 {
		t.Errorf("nothing may be saved after the level question is skipped; saved %d entries", len(store.putCalls))
	}
	if firstIndexOf(ia.asked, app.QuestionPricingRate) >= 0 {
		t.Error("no rate may be asked after the level question is skipped")
	}
	if !report.HasUnpricedModels() {
		t.Error("the model must stay unpriced when the level question is skipped")
	}
}

// editWith runs the pricing-edit flow for the one model of the gap scenario.
func editWith(t *testing.T, ia *scriptedInteraction) (app.PricingEditResult, *fakePricingStore) {
	t.Helper()
	res, _, store := editPricing(t, ia, []domain.ProviderID{""})
	return res, store
}

func TestEditPricing_AsksTheLevelAfterTheModelAndBeforeAnyRate(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))

	editWith(t, ia)

	model := firstIndexOf(ia.asked, app.QuestionPricingModel)
	level := firstIndexOf(ia.asked, app.QuestionConfigLevel)
	rate := firstIndexOf(ia.asked, app.QuestionPricingRate)
	if level < 0 || !(model < level && level < rate) {
		t.Errorf("order of model/level/rate questions = %d/%d/%d, want model < level < rate; asked: %v", model, level, rate, ia.asked)
	}
}

func TestEditPricing_LevelQuestionOffersBothFilesWithGlobalAsDefault(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))

	_, store := editWith(t, ia)

	questions := levelQuestions(ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	assertLevelQuestionShape(t, questions[0], store.Paths())
}

func TestEditPricing_TheResultNamesTheWrittenFileAndItsLevel(t *testing.T) {
	cases := []struct {
		option string
		level  domain.ConfigLevel
		path   string
	}{
		{"global", domain.LevelGlobal, fakeGlobalPricingPath},
		{"project", domain.LevelProject, "/test/pricing.yaml"},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			ia := choosingInteraction(string(providerGapModel))
			ia.levelOption = c.option

			res, store := editWith(t, ia)

			if res.Outcome != app.PricingEditSaved {
				t.Fatalf("Outcome = %v, want PricingEditSaved", res.Outcome)
			}
			if res.ConfigPath != c.path || res.ConfigLevel != c.level {
				t.Errorf("ConfigPath/ConfigLevel = %q/%v, want %q/%v", res.ConfigPath, res.ConfigLevel, c.path, c.level)
			}
			if len(store.putLevels) != 1 || store.putLevels[0] != c.level {
				t.Errorf("saved at levels %v, want exactly [%v]", store.putLevels, c.level)
			}
			if !noticeContains(ia.notifyCalls, c.path) {
				t.Errorf("the save notice must name %q; notices: %+v", c.path, ia.notifyCalls)
			}
		})
	}
}

func TestEditPricing_GlobalSave_ReportReflectsTheMergedResult(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))
	ia.levelOption = "global"

	res, store := editWith(t, ia)
	if len(store.putLevels) != 1 || store.putLevels[0] != domain.LevelGlobal {
		t.Fatalf("saved at levels %v, want [global]", store.putLevels)
	}

	if res.Report.HasUnpricedModels() {
		t.Errorf("the model must be priced after the global save; unpriced: %v", res.Report.UnpricedModels)
	}
}

func TestEditPricing_GlobalUnavailable_DisablesTheGlobalOption(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))
	svc, result, store := providerGapAnalysis(t, ia, []domain.ProviderID{""})
	store.globalUnavailable = true

	if _, err := svc.EditPricing(context.Background(), result.Session, result.Report); err != nil {
		t.Fatalf("EditPricing: %v", err)
	}

	questions := levelQuestions(ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	assertGlobalUnavailableShape(t, questions[0])
}

func TestEditPricing_SkippedLevel_SavesNothingAndAsksNoRate(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))
	ia.skipLevel = true

	res, store := editWith(t, ia)

	if res.Outcome == app.PricingEditSaved {
		t.Error("a skipped level question must not end as saved")
	}
	if len(store.putCalls) != 0 {
		t.Errorf("nothing may be saved after the level question is skipped; saved %d entries", len(store.putCalls))
	}
	if firstIndexOf(ia.asked, app.QuestionPricingRate) >= 0 {
		t.Error("no rate may be asked after the level question is skipped")
	}
	if !res.Report.HasUnpricedModels() {
		t.Error("the report must stay unchanged when the level question is skipped")
	}
}

func TestLevelQuestion_PromptAsksAllProjectsOrThisProject(t *testing.T) {
	ia := choosingInteraction(string(providerGapModel))

	editWith(t, ia)

	questions := levelQuestions(ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	prompt := strings.ToLower(questions[0].Prompt)
	if !strings.Contains(prompt, "all projects") || !strings.Contains(prompt, "this project") {
		t.Errorf("prompt %q must ask about all projects or this project", questions[0].Prompt)
	}
}
