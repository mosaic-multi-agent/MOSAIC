package app_test

// transform_codex_source_tools_test.go specifies the tool handling of a Transform from the
// Codex harness, whose agent files record no tool list. Chosen precedence: a catalog entry
// that declares tools wins (the target gets the tools Deploy would write for that agent, no
// warning); without a usable catalog entry a minimal read-only grant is applied and the run
// warns once, listing the affected destination files. Markdown sources never get the grant.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// cxToolsAgentKey is the agent key shared by the catalog stub and the Codex source file.
const cxToolsAgentKey = "code-reviewer"

// cxMinimalGrantTools are the Claude Code tool names the minimal read-only grant renders to.
var cxMinimalGrantTools = []string{"Glob", "Grep", "Read"}

// cxCatalogTools are the Claude Code tool names of cxCatalogAgent's generic list.
var cxCatalogTools = []string{"Bash", "Read", "Write"}

// cxCatalogAgent is a catalog entry for cxToolsAgentKey that declares a generic tool list
// distinct from the minimal grant.
func cxCatalogAgent() domain.Agent {
	return domain.Agent{
		Key:   cxToolsAgentKey,
		Role:  domain.RoleWorker,
		Tools: []string{"file_read", "file_write", "terminal"},
	}
}

// withCatalogAgents replaces the run's catalog with a stub holding the given agents.
func withCatalogAgents(agents ...domain.Agent) func(*app.Deps) {
	return func(d *app.Deps) {
		cat := newMinimalCatalog()
		cat.agents = agents
		d.Catalog = cat
	}
}

// cxToolsRun wires a Codex to Claude Code run with the given catalog agents.
func cxToolsRun(t *testing.T, agents ...domain.Agent) *trRun {
	t.Helper()
	return trNew(t, nil, hCodex, hClaude, withCodexTarget(t), withCatalogAgents(agents...))
}

func TestTransformFromCodex_Run_WithoutCatalogEntryAppliesMinimalReadOnlyGrant(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, cxToolsAgentKey)
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	got := rbToolNames(t, r.read(r.dest(cxToolsAgentKey)), "tools")
	if !equalStrings(got, cxMinimalGrantTools) {
		t.Errorf("written tools = %v, want the minimal read-only grant %v", got, cxMinimalGrantTools)
	}
	if !equalStrings(sortedCopy(f.Tools...), cxMinimalGrantTools) {
		t.Errorf("outcome tools = %v, want %v", f.Tools, cxMinimalGrantTools)
	}
}

func TestTransformFromCodex_Run_WithoutCatalogEntryWarnsOnceNamingTheDestination(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 {
		t.Fatalf("minimal-grant notes = %d, want exactly 1: %+v", len(notes), res.Notes)
	}
	if len(notes[0].Paths) != 1 || notes[0].Paths[0] != r.dest(cxToolsAgentKey) {
		t.Errorf("note paths = %v, want [%s]", notes[0].Paths, r.dest(cxToolsAgentKey))
	}
	if notes[0].Message == "" {
		t.Errorf("the minimal-grant note must carry a warning message")
	}
}

func TestTransformFromCodex_Run_WithCatalogToolsUsesCatalogToolsAndDoesNotWarn(t *testing.T) {
	r := cxToolsRun(t, cxCatalogAgent())
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	f := trFile(t, res, cxToolsAgentKey)
	if f.Status != app.StatusTransformed {
		t.Fatalf("status = %q (reason %q), want transformed", f.Status, f.Reason)
	}
	got := rbToolNames(t, r.read(r.dest(cxToolsAgentKey)), "tools")
	if !equalStrings(got, cxCatalogTools) {
		t.Errorf("written tools = %v, want the catalog agent's tools %v", got, cxCatalogTools)
	}
	if notes := trNotes(res, app.NoteMinimalToolGrant); len(notes) != 0 {
		t.Errorf("a catalog-tools agent must not trigger the minimal-grant warning: %+v", notes)
	}
}

func TestTransformFromCodex_Run_CatalogEntryWithoutToolsFallsBackToMinimalGrant(t *testing.T) {
	entry := cxCatalogAgent()
	entry.Tools = nil
	r := cxToolsRun(t, entry)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	got := rbToolNames(t, r.read(r.dest(cxToolsAgentKey)), "tools")
	if !equalStrings(got, cxMinimalGrantTools) {
		t.Errorf("written tools = %v, want the minimal grant %v", got, cxMinimalGrantTools)
	}
	if notes := trNotes(res, app.NoteMinimalToolGrant); len(notes) != 1 {
		t.Errorf("minimal-grant notes = %d, want 1", len(notes))
	}
}

func TestTransformFromCodex_Run_WarningListsOnlyAgentsThatGotTheMinimalGrant(t *testing.T) {
	r := cxToolsRun(t, cxCatalogAgent())
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))
	cxPut(t, r, "helper.toml", codexSourceAgent(cxOpts{name: "helper", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 || len(notes[0].Paths) != 1 || notes[0].Paths[0] != r.dest("helper") {
		t.Fatalf("minimal-grant note = %+v, want one note listing only %s", notes, r.dest("helper"))
	}
	if got := rbToolNames(t, r.read(r.dest(cxToolsAgentKey)), "tools"); !equalStrings(got, cxCatalogTools) {
		t.Errorf("catalog agent tools = %v, want %v", got, cxCatalogTools)
	}
}

func TestTransformFromMarkdown_Run_NeverAppliesMinimalGrantWarning(t *testing.T) {
	r := trNew(t, nil, hClaude, hOpen, withCatalogAgents())
	r.seed("subagent")

	res := r.mustRun(r.request())

	if notes := trNotes(res, app.NoteMinimalToolGrant); len(notes) != 0 {
		t.Errorf("a Markdown source records its tools; no minimal-grant note expected: %+v", notes)
	}
}

func TestRebuildFromCodex_WithoutCatalogEntryAppliesMinimalGrant(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey}))

	res, out := rbRebuild(t, in)

	if !res.MinimalToolGrant {
		t.Errorf("MinimalToolGrant = false, want true for a source that records no tools")
	}
	if got := rbToolNames(t, out, "tools"); !equalStrings(got, cxMinimalGrantTools) {
		t.Errorf("output tools = %v, want %v", got, cxMinimalGrantTools)
	}
	if !equalStrings(sortedCopy(res.Tools...), cxMinimalGrantTools) {
		t.Errorf("result Tools = %v, want %v", res.Tools, cxMinimalGrantTools)
	}
}

func TestRebuildFromCodex_CatalogToolsWinOverMinimalGrant(t *testing.T) {
	in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey}))
	in.Catalog = MockCatalogAgentLookup{ByKey: map[string]domain.Agent{cxToolsAgentKey: cxCatalogAgent()}}

	res, out := rbRebuild(t, in)

	if res.MinimalToolGrant {
		t.Errorf("MinimalToolGrant = true, want false when the catalog declares tools")
	}
	if got := rbToolNames(t, out, "tools"); !equalStrings(got, cxCatalogTools) {
		t.Errorf("output tools = %v, want the catalog tools %v", got, cxCatalogTools)
	}
}

func TestRebuildFromMarkdown_NeverSetsMinimalToolGrant(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, _ := rbRebuild(t, rbInput(t, p, false))

			if res.MinimalToolGrant {
				t.Errorf("MinimalToolGrant = true for a source with recoverable tools")
			}
		})
	}
}

func TestInspectFromCodex_ToolsReflectCatalogOrMinimalGrant(t *testing.T) {
	t.Run("catalog tools", func(t *testing.T) {
		in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey}))
		in.Catalog = MockCatalogAgentLookup{ByKey: map[string]domain.Agent{cxToolsAgentKey: cxCatalogAgent()}}

		insp, err := app.InspectDeployedAgent(in)

		if err != nil {
			t.Fatalf("InspectDeployedAgent: %v", err)
		}
		if !equalStrings(sortedCopy(insp.Converted.Generic...), sortedCopy(cxCatalogAgent().Tools...)) {
			t.Errorf("Converted.Generic = %v, want the catalog list %v", insp.Converted.Generic, cxCatalogAgent().Tools)
		}
	})
	t.Run("catalog generic the target cannot map is reported unmapped", func(t *testing.T) {
		entry := cxCatalogAgent()
		entry.Tools = append(entry.Tools, "no_such_generic_tool")
		in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey}))
		in.Catalog = MockCatalogAgentLookup{ByKey: map[string]domain.Agent{cxToolsAgentKey: entry}}

		insp, err := app.InspectDeployedAgent(in)

		if err != nil {
			t.Fatalf("InspectDeployedAgent: %v", err)
		}
		if !containsString(insp.Converted.Unmapped, "no_such_generic_tool") {
			t.Errorf("Converted.Unmapped = %v, want the unmappable catalog generic reported", insp.Converted.Unmapped)
		}
	})
	// Regression/support only (already true without the grant); not evidence for the new behavior.
	t.Run("minimal grant asks nothing (regression)", func(t *testing.T) {
		in := cxRebuildInput(t, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey}))

		insp, err := app.InspectDeployedAgent(in)

		if err != nil {
			t.Fatalf("InspectDeployedAgent: %v", err)
		}
		if len(insp.Converted.Unmapped) != 0 {
			t.Errorf("Converted.Unmapped = %v, want none under the minimal grant", insp.Converted.Unmapped)
		}
	})
}

func TestTransformFromCodex_Run_CatalogToolsPlaceholderTakesPlaceholderNotMinimalGrant(t *testing.T) {
	orch := domain.Agent{Key: "orchestrator", Role: domain.RoleOrchestrator, ToolsPlaceholder: "{tool-permissions}"}
	r := cxToolsRun(t, orch)
	cxPut(t, r, "orchestrator.toml", codexSourceAgent(cxOpts{name: "orchestrator", model: cxSourceModelOrch}))
	cxPut(t, r, "helper.toml", codexSourceAgent(cxOpts{name: "helper", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 || len(notes[0].Paths) != 1 || notes[0].Paths[0] != r.dest("helper") {
		t.Fatalf("minimal-grant note = %+v, want one note listing only %s (placeholder entry is not a grant)", notes, r.dest("helper"))
	}
}

func TestTransformFromCodex_Run_WarningListsEveryGrantedAgentInPlanOrder(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "alpha.toml", codexSourceAgent(cxOpts{name: "alpha", model: cxSourceModelSub}))
	cxPut(t, r, "bravo.toml", codexSourceAgent(cxOpts{name: "bravo", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 {
		t.Fatalf("minimal-grant notes = %d, want exactly 1: %+v", len(notes), res.Notes)
	}
	want := []string{r.dest("alpha"), r.dest("bravo")}
	if !equalStrings(notes[0].Paths, want) {
		t.Errorf("note paths = %v, want %v in plan order", notes[0].Paths, want)
	}
}

func TestTransformFromCodex_Run_FailedAgentIsNotListedInTheGrantWarning(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "broken.toml", "model = \"a\"\nmodel = \"b\"\n")
	cxPut(t, r, "good.toml", codexSourceAgent(cxOpts{name: "good", model: cxSourceModelSub}))

	res := r.mustRun(cxRequest(r))

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 || len(notes[0].Paths) != 1 || notes[0].Paths[0] != r.dest("good") {
		t.Errorf("minimal-grant note = %+v, want one note listing only %s", notes, r.dest("good"))
	}
}

func TestTransformFromCodex_Run_NoteIsNotSetWhenOnlyAgentFails(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "broken.toml", "model = \"a\"\nmodel = \"b\"\n")

	res := r.mustRun(cxRequest(r))

	if notes := trNotes(res, app.NoteMinimalToolGrant); len(notes) != 0 {
		t.Errorf("a failed agent got no grant; no note expected: %+v", notes)
	}
}

func TestTransformFromCodex_Run_MinimalGrantNoteIsReportedUnderDryRun(t *testing.T) {
	r := cxToolsRun(t)
	cxPut(t, r, "code-reviewer.toml", codexSourceAgent(cxOpts{name: cxToolsAgentKey, model: cxSourceModelSub}))
	req := cxRequest(r)
	req.DryRun = true

	res := r.mustRun(req)

	notes := trNotes(res, app.NoteMinimalToolGrant)
	if len(notes) != 1 || len(notes[0].Paths) != 1 || notes[0].Paths[0] != r.dest(cxToolsAgentKey) {
		t.Errorf("dry run minimal-grant note = %+v, want one note for %s", notes, r.dest(cxToolsAgentKey))
	}
	if r.exists(r.dest(cxToolsAgentKey)) {
		t.Errorf("a dry run must not write the target")
	}
}
