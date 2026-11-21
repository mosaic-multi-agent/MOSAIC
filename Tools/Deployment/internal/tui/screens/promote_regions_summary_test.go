package screens_test

// promote_regions_summary_test.go verifies what the promote summaries say about project and
// custom regions: the batch summary states the keep-or-empty answer when the question was asked
// and lists the removed custom regions by name under each promoted agent; the single promote
// summary lists the removed custom regions by name.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/tui/screens"
)

func regionsBatchResult(choice app.ProjectRegionChoice) app.PromoteBatchResult {
	return app.PromoteBatchResult{
		HarnessID:      "h",
		AgentsDir:      "/agents",
		TargetLayer:    batchTargetLayer,
		ProjectRegions: choice,
		Agents: []app.PromoteBatchAgentResult{
			{
				SourcePath: "/agents/alpha.md", Key: "alpha", Status: app.PromoteBatchPromoted,
				Result: app.PromoteResult{
					DestinationPath: "/team/Subagents/Dev/alpha.md", NumericID: "41", Key: "alpha",
					RemovedCustomRegions: []string{"AlphaNote", "AlphaExtra"},
				},
			},
			{
				SourcePath: "/agents/beta.md", Key: "beta", Status: app.PromoteBatchPromoted,
				Result: app.PromoteResult{
					DestinationPath: "/team/Subagents/Ops/beta.md", NumericID: "7", Key: "beta",
					RemovedCustomRegions: []string{"BetaNote"},
				},
			},
			{
				SourcePath: "/agents/gamma.md", Key: "gamma", Status: app.PromoteBatchPromoted,
				Result: app.PromoteResult{DestinationPath: "/team/Subagents/Ops/gamma.md", NumericID: "8", Key: "gamma"},
			},
		},
	}
}

func TestPromoteBatchSummary_StatesTheKeepAnswer(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(regionsBatchResult(app.ProjectRegionsKeep), 120, 40).View())

	if !strings.Contains(view, "Project region content: kept") {
		t.Errorf("view does not state that project region content was kept:\n%s", view)
	}
	if strings.Contains(view, "emptied") {
		t.Errorf("view says emptied for a keep answer:\n%s", view)
	}
}

func TestPromoteBatchSummary_StatesTheEmptyAnswer(t *testing.T) {
	view := collapseWhitespace(newBatchSummary(regionsBatchResult(app.ProjectRegionsEmpty), 120, 40).View())

	if !strings.Contains(view, "Project region content: emptied") {
		t.Errorf("view does not state that project region content was emptied:\n%s", view)
	}
}

func TestPromoteBatchSummary_AnswerLineFollowsTheTargetLayer(t *testing.T) {
	view := newBatchSummary(regionsBatchResult(app.ProjectRegionsEmpty), 120, 40).View()

	layerAt := strings.Index(view, "Target layer:")
	answerAt := strings.Index(view, "Project region content:")
	countsAt := strings.Index(view, "promoted, ")
	if layerAt < 0 || answerAt < layerAt || (countsAt >= 0 && answerAt > countsAt) {
		t.Errorf("answer line at %d, target layer at %d, counts at %d; want it directly after the target layer:\n%s",
			answerAt, layerAt, countsAt, view)
	}
}

func TestPromoteBatchSummary_NoAnswerLineWhenTheQuestionWasNotAsked(t *testing.T) {
	view := newBatchSummary(regionsBatchResult(""), 120, 40).View()

	if strings.Contains(view, "Project region content") {
		t.Errorf("view states an answer although none was given:\n%s", view)
	}
}

func TestPromoteBatchSummary_ListsRemovedCustomRegionsByNameUnderEachPromotedAgent(t *testing.T) {
	view := newBatchSummary(regionsBatchResult(app.ProjectRegionsKeep), 120, 40).View()

	for _, name := range []string{"AlphaNote", "AlphaExtra", "BetaNote"} {
		if !strings.Contains(view, "removed custom region: "+name) {
			t.Errorf("view lacks the line %q:\n%s", "removed custom region: "+name, view)
		}
	}
	alpha, beta, gamma := strings.Index(view, "alpha ("), strings.Index(view, "beta ("), strings.Index(view, "gamma (")
	alphaNote, betaNote := strings.Index(view, "AlphaNote"), strings.Index(view, "BetaNote")
	if !(alpha < alphaNote && alphaNote < beta && beta < betaNote && betaNote < gamma) {
		t.Errorf("removed custom regions are not listed under their own agent:\n%s", view)
	}
	if strings.Index(view, "AlphaNote") > strings.Index(view, "AlphaExtra") {
		t.Errorf("removed custom regions are not in the given order:\n%s", view)
	}
}

func TestPromoteBatchSummary_NoRemovedRegionLineForAnAgentWithNone(t *testing.T) {
	r := regionsBatchResult(app.ProjectRegionsKeep)
	r.Agents = r.Agents[2:] // gamma only

	view := newBatchSummary(r, 120, 40).View()

	if strings.Contains(view, "removed custom region") {
		t.Errorf("view lists a removed custom region for an agent that had none:\n%s", view)
	}
}

func TestPromoteSummaryScreen_ListsRemovedCustomRegionsByName(t *testing.T) {
	result := basicPromoteResult()
	result.RemovedCustomRegions = []string{"FirstNote", "SecondNote"}
	s := screens.NewPromoteSummaryScreen(result, 100, 40, plainStyles())

	view := s.View()

	first, second := strings.Index(view, "Removed custom region: FirstNote"), strings.Index(view, "Removed custom region: SecondNote")
	if first < 0 || second < first {
		t.Errorf("view does not list both removed custom regions in order:\n%s", view)
	}
}

func TestPromoteSummaryScreen_NoRemovedRegionLineWhenNoneWereRemoved(t *testing.T) {
	s := screens.NewPromoteSummaryScreen(basicPromoteResult(), 100, 40, plainStyles())

	if view := s.View(); strings.Contains(view, "Removed custom region") {
		t.Errorf("view lists a removed custom region although none was removed:\n%s", view)
	}
}
