package app_test

// promote_batch_projectregions_test.go covers the once-per-batch question whether the content of
// project regions is kept or emptied in the promoted generics: when it is asked (only when a
// loaded selected agent has project content at any depth), its default and options, its place
// after the tier question, how the answer applies to every agent of the batch, how a cancel aborts
// the batch, and that the batch result records the answer.

import (
	"bytes"
	"os"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	regionsTopLevelBody = "<Identity type=\"core\">\nYou are an agent.\n" +
		"<IdentityExtension type=\"project\">\nTop-level project text.\n</IdentityExtension>\n</Identity>\n"
	regionsInManagedBody = "<Identity type=\"core\">\nYou are an agent.\n</Identity>\n" +
		"<CommunicationProtocol type=\"managed\">\n" +
		"<NestedExtension type=\"project\">\nNested project text.\n</NestedExtension>\n" +
		"</CommunicationProtocol>\n"
	regionsEmptyOnlyBody = "<Identity type=\"core\">\nYou are an agent.\n" +
		"<IdentityExtension type=\"project\">\n\n</IdentityExtension>\n</Identity>\n"
	regionsNoneBody = "<Identity type=\"core\">\nYou are an agent without project regions.\n</Identity>\n"
)

// batchAgentBody is the bytes of a batch agent file whose body is body.
func batchAgentBody(file, body string) []byte {
	full := batchSrc{File: file}.bytes()
	cut := bytes.Index(full, []byte("<Identity"))
	return append(append([]byte(nil), full[:cut]...), body...)
}

// regionsBatch writes one agent file per (file, body) pair, selects them all and returns the
// deps, the folder and the spy of a batch over them.
func regionsBatch(t *testing.T, bodies map[string]string, order ...string) (app.Deps, string, *batchSpy) {
	t.Helper()
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t)
	for _, file := range order {
		writeBatchFile(t, dir, file, batchAgentBody(file, bodies[file]))
	}
	spy := batchSpyFor(t, ls, order...)
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return ls.batchDeps(t, spy), dir, spy
}

// projectContents returns the trimmed content of every project region of the written generic.
func projectContents(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written generic: %v", err)
	}
	doc, err := docformat.Parse(data)
	if err != nil {
		t.Fatalf("parse written generic: %v", err)
	}
	var out []string
	for _, inj := range doc.Body().Injections() {
		out = append(out, string(bytes.TrimSpace(inj.Content())))
	}
	return out
}

func twoAgents(a, b string) (map[string]string, []string) {
	return map[string]string{"a-agent.md": a, "b-agent.md": b}, []string{"a-agent.md", "b-agent.md"}
}

func TestPromoteBatch_ProjectRegionsQuestion_AskedOnceForSeveralAgentsWithProjectContent(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsInManagedBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)

	mustRunBatch(t, deps, dir)

	if n := len(spy.asked(domain.QPromoteProjectRegions)); n != 1 {
		t.Errorf("project regions question asked %d times, want once per batch", n)
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_AskedForContentNestedInAManagedRegion(t *testing.T) {
	bodies, order := twoAgents(regionsInManagedBody, regionsNoneBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)

	mustRunBatch(t, deps, dir)

	if n := len(spy.asked(domain.QPromoteProjectRegions)); n != 1 {
		t.Errorf("project regions question asked %d times, want once for a project region nested in a managed region", n)
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_NotAskedWhenNoAgentHasProjectContent(t *testing.T) {
	bodies, order := twoAgents(regionsEmptyOnlyBody, regionsNoneBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)

	res := mustRunBatch(t, deps, dir)

	if n := len(spy.asked(domain.QPromoteProjectRegions)); n != 0 {
		t.Errorf("project regions question asked %d times, want none when no project region has content", n)
	}
	if res.ProjectRegions != "" {
		t.Errorf("ProjectRegions = %q, want empty when the question was not asked", res.ProjectRegions)
	}
	requirePromoted(t, res, "a-agent.md")
	requirePromoted(t, res, "b-agent.md")
}

// settledBatch is a batch over x.agent.md (body firstBody) and x.md (body dupBody). Both files
// have the key x, so x.md is skipped as a duplicate before any agent source is loaded, while
// x.agent.md is loaded and promoted.
func settledBatch(t *testing.T, firstBody, dupBody string) (app.Deps, string, *batchSpy) {
	t.Helper()
	ls := newBatchLayers(t, "only")
	dir := writeBatchFolder(t)
	writeBatchFile(t, dir, "x.agent.md", batchAgentBody("x.agent.md", firstBody))
	writeBatchFile(t, dir, "x.md", batchAgentBody("x.md", dupBody))
	spy := batchSpyFor(t, ls, "x.agent.md", "x.md")
	spy.oneByID[domain.QPromoteCategory] = chosen("Audit")
	return ls.batchDeps(t, spy), dir, spy
}

func TestPromoteBatch_ProjectRegionsQuestion_NotAskedWhenOnlyAnAgentSettledBeforeLoadHasProjectContent(t *testing.T) {
	deps, dir, spy := settledBatch(t, regionsNoneBody, regionsTopLevelBody)

	res := mustRunBatch(t, deps, dir)

	if a := agentResult(t, res, "x.md"); a.Status != app.PromoteBatchSkipped {
		t.Fatalf("x.md status = %q, want skipped as the duplicate (the fixture must settle it before load)", a.Status)
	}
	requirePromoted(t, res, "x.agent.md")
	if n := len(spy.asked(domain.QPromoteProjectRegions)); n != 0 {
		t.Errorf("project regions question asked %d times, want none: the only agent with project content was skipped", n)
	}
	if res.ProjectRegions != "" {
		t.Errorf("ProjectRegions = %q, want empty when the question was not asked", res.ProjectRegions)
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_AskedForALoadedAgentWhenTheSettledOneHasNone(t *testing.T) {
	deps, dir, spy := settledBatch(t, regionsTopLevelBody, regionsNoneBody)

	res := mustRunBatch(t, deps, dir)

	if a := agentResult(t, res, "x.md"); a.Status != app.PromoteBatchSkipped {
		t.Fatalf("x.md status = %q, want skipped as the duplicate", a.Status)
	}
	requirePromoted(t, res, "x.agent.md")
	if n := len(spy.asked(domain.QPromoteProjectRegions)); n != 1 {
		t.Errorf("project regions question asked %d times, want once for the loaded agent with project content", n)
	}
}

func TestPromoteBatch_ProjectRegionsEmpty_EmptiesEveryProjectRegionOfAnAgentWithSeveral(t *testing.T) {
	const manyBody = "<Identity type=\"core\">\nYou are an agent.\n" +
		"<FirstExtension type=\"project\">\nFirst text.\n</FirstExtension>\n" +
		"<SecondExtension type=\"project\">\nSecond text.\n</SecondExtension>\n</Identity>\n" +
		"<CommunicationProtocol type=\"managed\">\n" +
		"<NestedExtension type=\"project\">\nNested text.\n</NestedExtension>\n" +
		"</CommunicationProtocol>\n"
	bodies, order := twoAgents(manyBody, regionsNoneBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)
	spy.oneByID[domain.QPromoteProjectRegions] = chosen("empty")

	res := mustRunBatch(t, deps, dir)

	got := projectContents(t, requirePromoted(t, res, "a-agent.md").Result.DestinationPath)
	if len(got) != 3 {
		t.Fatalf("project regions = %q, want all three kept as regions", got)
	}
	for _, content := range got {
		if content != "" {
			t.Errorf("project regions = %q, want every region emptied", got)
			break
		}
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_DefaultsToKeepAndOffersKeepThenEmpty(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsNoneBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)

	mustRunBatch(t, deps, dir)

	asks := spy.asked(domain.QPromoteProjectRegions)
	if len(asks) != 1 {
		t.Fatalf("project regions question asked %d times, want 1", len(asks))
	}
	q := asks[0].Choice
	if q.DefaultOptionID != string(app.ProjectRegionsKeep) {
		t.Errorf("DefaultOptionID = %q, want %q", q.DefaultOptionID, app.ProjectRegionsKeep)
	}
	if len(q.Options) != 2 || q.Options[0].ID != string(app.ProjectRegionsKeep) || q.Options[1].ID != string(app.ProjectRegionsEmpty) {
		t.Errorf("options = %+v, want keep then empty", q.Options)
	}
	if q.AllowSkip {
		t.Error("the question allows skipping, want a plain two-option choice")
	}
	if asks[0].Subject != dir {
		t.Errorf("Subject = %q, want the agents folder %q", asks[0].Subject, dir)
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_IsAskedAfterTheTierQuestion(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsTopLevelBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)

	mustRunBatch(t, deps, dir)

	tierAt, regionsAt := -1, -1
	for i, a := range spy.asks {
		if a.ID == domain.QPromoteRecommendedTier {
			tierAt = i
		}
		if a.ID == domain.QPromoteProjectRegions {
			regionsAt = i
		}
	}
	if tierAt < 0 || regionsAt < 0 || regionsAt < tierAt {
		t.Errorf("tier question at %d, project regions question at %d; want both asked, the latter after the former", tierAt, regionsAt)
	}
	if categoryAt := indexOfAsk(spy, domain.QPromoteCategory); categoryAt >= 0 && regionsAt > categoryAt {
		t.Error("project regions question was asked after an agent was processed")
	}
}

func indexOfAsk(spy *batchSpy, id domain.QuestionID) int {
	for i, a := range spy.asks {
		if a.ID == id {
			return i
		}
	}
	return -1
}

func TestPromoteBatch_ProjectRegionsKeep_AppliesToEveryAgentAndIsRecorded(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsInManagedBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)
	spy.oneByID[domain.QPromoteProjectRegions] = chosen("keep")

	res := mustRunBatch(t, deps, dir)

	a, b := requirePromoted(t, res, "a-agent.md"), requirePromoted(t, res, "b-agent.md")
	if got := projectContents(t, a.Result.DestinationPath); len(got) != 1 || got[0] != "Top-level project text." {
		t.Errorf("first agent project contents = %q, want its text kept", got)
	}
	if got := projectContents(t, b.Result.DestinationPath); len(got) != 1 || got[0] != "Nested project text." {
		t.Errorf("second agent project contents = %q, want its nested text kept", got)
	}
	if res.ProjectRegions != app.ProjectRegionsKeep {
		t.Errorf("ProjectRegions = %q, want %q", res.ProjectRegions, app.ProjectRegionsKeep)
	}
}

func TestPromoteBatch_ProjectRegionsEmpty_AppliesToEveryAgentAndIsRecorded(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsInManagedBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)
	spy.oneByID[domain.QPromoteProjectRegions] = chosen("empty")

	res := mustRunBatch(t, deps, dir)

	for _, file := range order {
		got := projectContents(t, requirePromoted(t, res, file).Result.DestinationPath)
		if len(got) != 1 || got[0] != "" {
			t.Errorf("%s project contents = %q, want one emptied region", file, got)
		}
	}
	if res.ProjectRegions != app.ProjectRegionsEmpty {
		t.Errorf("ProjectRegions = %q, want %q", res.ProjectRegions, app.ProjectRegionsEmpty)
	}
}

func TestPromoteBatch_ProjectRegionsEmpty_AlsoEmptiesAnAgentWithoutProjectContent(t *testing.T) {
	// The second agent holds one whitespace-only region; the first carries the content that
	// makes the question appear. The answer is batch-wide, so the second agent is generated
	// with it too and keeps its single (empty) region.
	bodies, order := twoAgents(regionsTopLevelBody, regionsEmptyOnlyBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)
	spy.oneByID[domain.QPromoteProjectRegions] = chosen("empty")

	res := mustRunBatch(t, deps, dir)

	if got := projectContents(t, requirePromoted(t, res, "a-agent.md").Result.DestinationPath); len(got) != 1 || got[0] != "" {
		t.Errorf("first agent project contents = %q, want emptied", got)
	}
	if got := projectContents(t, requirePromoted(t, res, "b-agent.md").Result.DestinationPath); len(got) != 1 || got[0] != "" {
		t.Errorf("second agent project contents = %q, want its empty region", got)
	}
}

func TestPromoteBatch_ProjectRegionsAnswer_SkippedOrUnknownMeansKeep(t *testing.T) {
	answers := map[string]domain.ChoiceAnswer{
		"skipped one": status(domain.SkippedOne),
		"skipped all": status(domain.SkippedAll),
		"unknown id":  chosen("something-else"),
	}
	for name, ans := range answers {
		t.Run(name, func(t *testing.T) {
			bodies, order := twoAgents(regionsTopLevelBody, regionsNoneBody)
			deps, dir, spy := regionsBatch(t, bodies, order...)
			spy.oneByID[domain.QPromoteProjectRegions] = ans

			res := mustRunBatch(t, deps, dir)

			got := projectContents(t, requirePromoted(t, res, "a-agent.md").Result.DestinationPath)
			if len(got) != 1 || got[0] != "Top-level project text." {
				t.Errorf("project contents = %q, want the text kept", got)
			}
			if res.ProjectRegions != app.ProjectRegionsKeep {
				t.Errorf("ProjectRegions = %q, want %q", res.ProjectRegions, app.ProjectRegionsKeep)
			}
		})
	}
}

func TestPromoteBatch_ProjectRegionsQuestion_CancelAbortsBeforeAnyAgentIsPromoted(t *testing.T) {
	bodies, order := twoAgents(regionsTopLevelBody, regionsTopLevelBody)
	deps, dir, spy := regionsBatch(t, bodies, order...)
	spy.oneByID[domain.QPromoteProjectRegions] = status(domain.Cancelled)

	res := mustRunBatch(t, deps, dir)

	if !res.Aborted {
		t.Error("Aborted = false, want true after the question was cancelled")
	}
	if res.ProjectRegions != "" {
		t.Errorf("ProjectRegions = %q, want empty on abort", res.ProjectRegions)
	}
	for _, file := range order {
		if a := agentResult(t, res, file); a.Status != app.PromoteBatchNotProcessed {
			t.Errorf("%s status = %q, want not-processed", file, a.Status)
		}
	}
	if n := len(spy.asked(domain.QPromoteCategory)); n != 0 {
		t.Errorf("category asked %d times after the abort, want none", n)
	}
}
