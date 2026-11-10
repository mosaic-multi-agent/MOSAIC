package app_test

// transform_failure_test.go specifies what the whole-deployment transform writes and how it
// fails: the written agent is the rebuilt target-harness form, a file that cannot be rebuilt is
// reported as failed while the others continue, a write failure reverts the whole run, nothing
// is asked when nothing is written, the catalog is consulted by agent key as well as by id,
// and a cancelled workspace question aborts with nothing written. The write-failure case runs
// outside the GHCP pair; the pair's failures are in transform_ghcp_pair_failure_test.go.

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trBrokenAgent is a well-formed stamped agent, so it is recognised and planned for writing.
// Its rebuild fails because trFailingFrontmatterModule cannot build the target frontmatter for it.
const trBrokenAgent = "---\nmosaic_harness_version: 1.2.0\nmosaic_version: 1.0.0\nname: broken\n" +
	"description: Broken\n---\n\n<Identity type=\"core\">\nYou are broken.\n</Identity>\n"

// trFailingFrontmatterModule is a harness module whose frontmatter planning fails for one agent key.
type trFailingFrontmatterModule struct {
	domain.HarnessModule
	failKey string
}

func (m trFailingFrontmatterModule) Frontmatter(req domain.FrontmatterRequest) (domain.FrontmatterPlan, error) {
	if req.AgentKey == m.failKey {
		return domain.FrontmatterPlan{}, errors.New("cannot plan frontmatter for " + req.AgentKey)
	}
	return m.HarnessModule.Frontmatter(req)
}

// trFailFrontmatterFor makes the harness fail to plan the frontmatter of the agent with the key.
func trFailFrontmatterFor(harnessID, key string) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		reg.modules[harnessID] = trFailingFrontmatterModule{HarnessModule: reg.modules[harnessID], failKey: key}
	}
}

// trSearcherAgent is a GHCP CLI agent whose single `search` tool is ambiguous in reverse.
const trSearcherAgent = "---\nmosaic_harness_version: 1.2.0\nmosaic_version: 1.0.0\nname: searcher\n" +
	"description: Searches\ntools: ['search']\n---\n\n<Identity type=\"core\">\nYou are the searcher.\n</Identity>\n"

func TestTransformDeployment_WrittenAgentIsTheRebuiltTargetForm(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed("subagent", "harness-only")
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	sub := r.read(r.dest("subagent"))
	if trHasKey(sub, "tools") {
		t.Error("the written agent still has the source `tools:` field instead of the target form")
	}
	if !slices.Contains(trFrontmatter(sub), "mode: subagent") || !trHasKey(sub, "permission") {
		t.Errorf("the written agent lacks the target form (mode and permission):\n%s", sub)
	}
	for _, want := range []string{"mosaic_id: 55", "mosaic_version: 2.3.0"} {
		if !slices.Contains(trFrontmatter(sub), want) {
			t.Errorf("the written agent does not carry %q:\n%s", want, sub)
		}
	}
	if slices.Contains(trFrontmatter(sub), "mosaic_harness_version: 1.2.0") || !trHasKey(sub, "mosaic_harness_version") {
		t.Errorf("the written agent does not carry the target harness version stamp:\n%s", sub)
	}
	trAssertModel(t, r, "subagent", "pre/answered")
	if !strings.Contains(sub, "You are the **CodeReviewer** agent.") {
		t.Errorf("the body was not carried:\n%s", sub)
	}
	if only := r.read(r.dest("harness-only")); trHasKey(only, "mosaic_version") {
		t.Errorf("a missing mosaic_version was invented:\n%s", only)
	}
}

func TestTransformDeployment_AgentThatCannotBeRebuilt_IsFailedAndTheOthersContinue(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen, trFailFrontmatterFor(hOpen, "broken"))
	r.seed(trAgentFixtures...)
	r.put(hClaude, "broken", trBrokenAgent)

	res := r.mustRun(r.request())

	f := trFile(t, res, "broken")
	if f.Status != app.StatusFailed || f.Reason == "" {
		t.Errorf("broken: status %q reason %q, want failed with a reason", f.Status, f.Reason)
	}
	if r.exists(r.dest("broken")) {
		t.Error("the failed agent was written")
	}
	if res.Failed != 1 || res.Transformed != len(trAgentFixtures) {
		t.Errorf("Failed=%d Transformed=%d, want 1 and %d", res.Failed, res.Transformed, len(trAgentFixtures))
	}
	for _, key := range trAgentFixtures {
		if !r.exists(r.dest(key)) {
			t.Errorf("agent %q was not written although only another file failed", key)
		}
	}
}

func TestTransformDeployment_OutsideTheGhcpPair_WriteFailure_RevertsEveryWrittenFile(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed(trAgentFixtures...)
	// A non-empty folder where the last planned agent (utility) must be written cannot be overwritten.
	trWrite(t, r.ws, r.dest("utility")+"/keep.txt", "blocker")
	before := r.snapshot()
	req := r.request()
	req.Overwrite = true

	res, err := r.run(req)

	if !errors.Is(err, app.ErrRunReverted) {
		t.Fatalf("error = %v, want ErrRunReverted", err)
	}
	if res.Cancelled || len(res.Files) != 0 {
		t.Errorf("Cancelled=%v Files=%d, want false and none", res.Cancelled, len(res.Files))
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("the reverted run left a trace: %s", diff)
	}
	for _, key := range trAgentFixtures {
		if key != "utility" && r.exists(r.dest(key)) {
			t.Errorf("agent %q is still written after the revert", key)
		}
	}
}

func TestTransformDeployment_NothingToWrite_AsksNoModelQuestionAndNoReview(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen)
	r.seed(trAgentFixtures...)
	for _, key := range trAgentFixtures {
		r.put(hOpen, key, "existing destination")
	}
	req := r.request()
	req.AutoConfirmPlan = false

	res := r.mustRun(req)

	if len(r.spy.choices) != 0 || len(r.spy.reviews) != 0 {
		t.Errorf("asked %d choice questions and %d reviews although no file is written", len(r.spy.choices), len(r.spy.reviews))
	}
	if res.Transformed != 0 {
		t.Errorf("Transformed = %d, want 0", res.Transformed)
	}
}

func TestTransformDeployment_CatalogEntryByAgentKey_DecidesTheRoleOfAnAgentWithoutAnID(t *testing.T) {
	cat := newMinimalCatalog()
	cat.agents = []domain.Agent{{Key: "harness-only", Role: domain.RoleUtility}}
	with := trNew(t, nil, hClaude, hOpen, func(d *app.Deps) { d.Catalog = cat })
	with.seed("harness-only")
	without := trNew(t, nil, hClaude, hOpen)
	without.seed("harness-only")

	gotWith := trFile(t, with.mustRun(with.request()), "harness-only").Role
	gotWithout := trFile(t, without.mustRun(without.request()), "harness-only").Role

	if gotWith != domain.RoleUtility {
		t.Errorf("role with a catalog entry by key = %q, want %q", gotWith, domain.RoleUtility)
	}
	if gotWithout == domain.RoleUtility {
		t.Errorf("role without a catalog entry = %q, want the default", gotWithout)
	}
}

func TestTransformDeployment_CatalogToolsResolveAnAmbiguousReverseToolMapping(t *testing.T) {
	cat := newMinimalCatalog()
	cat.agents = []domain.Agent{{Key: "searcher", Role: domain.RoleSubagent, Tools: []string{"content_search"}}}
	with := trNew(t, nil, hCLI, hClaude, func(d *app.Deps) { d.Catalog = cat })
	with.put(hCLI, "searcher", trSearcherAgent)
	without := trNew(t, nil, hCLI, hClaude)
	without.put(hCLI, "searcher", trSearcherAgent)

	with.mustRun(with.request())
	without.mustRun(without.request())

	if got := rbToolNames(t, with.read(with.dest("searcher")), "tools"); !slices.Equal(got, []string{"Grep"}) {
		t.Errorf("tools with the catalog entry listing content_search = %v, want [Grep]", got)
	}
	if got := rbToolNames(t, without.read(without.dest("searcher")), "tools"); !slices.Equal(got, []string{"Glob"}) {
		t.Errorf("tools without a catalog entry = %v, want the first reverse candidate [Glob]", got)
	}
}

func TestTransformDeployment_CancelledWorkspaceQuestion_ReportsCancelledAndWritesNothing(t *testing.T) {
	script := interactiontest.NewBuilder().AnswerCancelledText(domain.QWorkspace, "").Build()
	r := trNew(t, script, hClaude, hOpen)
	trSeedAll(r)
	before := r.snapshot()
	req := r.request()
	req.Workspace = ""

	res, err := r.run(req)

	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("error = %v, want ErrSelectionCancelled", err)
	}
	if !res.Cancelled {
		t.Error("Cancelled = false, want true")
	}
	if diff := trSameTree(before, r.snapshot()); diff != "" {
		t.Errorf("cancelled run left a trace: %s", diff)
	}
}
