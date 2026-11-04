package app_test

// rebuild_role_test.go covers the role each deployed agent is rebuilt with, in its four-step
// order: (a) the source's mosaic_role stamp, (b) the file name orchestrator or
// orchestrator-script, (c) the catalog entry found by mosaic_id and otherwise by agent key,
// (d) subagent. The role is observed on the result and through the role-dependent
// user-invocable field of the GHCP target; a mosaic_role stamp is never written for a source
// that had none. The catalog's generic tool list resolves ambiguous reverse tool mappings.

import (
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// roleCase rebuilds Claude Code to VS Code GHCP, the one GHCP pair role tests need.
var roleCase = rbPairs[2]

// withoutRoleStamp drops the mosaic_role line of the fully stamped fixture.
var withoutRoleStamp = withSourceEdit(removeLine("mosaic_role:"))

// invocableByRole is the target's user-invocable value for each role.
var invocableByRole = map[domain.AgentRole]string{
	domain.RoleSubagent:     "false",
	domain.RoleOrchestrator: "true",
	domain.RoleUtility:      "true",
	domain.RoleStandalone:   "true",
}

func assertRole(t *testing.T, res app.RebuildResult, out string, want domain.AgentRole) {
	t.Helper()
	if res.Role != want {
		t.Errorf("Role: want %q, got %q", want, res.Role)
	}
	if got, ok := rbScalar(t, out, "user-invocable"); !ok || got != invocableByRole[want] {
		t.Errorf("user-invocable for role %q: want %q, got %q (present=%v)", want, invocableByRole[want], got, ok)
	}
}

func TestRebuildRole_StampWinsOverEverything(t *testing.T) {
	cat := MockCatalogAgentLookup{ByID: map[string]domain.Agent{"55": {Key: "code-reviewer", Role: domain.RoleOrchestrator}}}
	cases := map[string]struct {
		edit func(string) string
		want domain.AgentRole
	}{
		"utility stamp":    {replaceText("mosaic_role: subagent", "mosaic_role: utility"), domain.RoleUtility},
		"standalone stamp": {replaceText("mosaic_role: subagent", "mosaic_role: standalone"), domain.RoleStandalone},
		"subagent stamp against orchestrator file name and catalog": {nil, domain.RoleSubagent},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			opts := []rbOpt{withFileName("orchestrator.md"), withCatalogLookup(cat)}
			if c.edit != nil {
				opts = append(opts, withSourceEdit(c.edit))
			}
			in := rbInput(t, roleCase, false, opts...)

			res, out := rbRebuild(t, in)

			assertRole(t, res, out, c.want)
		})
	}
}

func TestRebuildRole_UnparseableStampFallsThroughToNextRule(t *testing.T) {
	in := rbInput(t, roleCase, false, withSourceEdit(replaceText("mosaic_role: subagent", "mosaic_role: worker")),
		withFileName("orchestrator.md"))

	res, out := rbRebuild(t, in)

	assertRole(t, res, out, domain.RoleOrchestrator)
}

func TestRebuildRole_OrchestratorFileNamesGiveOrchestratorRole(t *testing.T) {
	for _, name := range []string{"orchestrator.md", "orchestrator.agent.md", "orchestrator-script.md", "orchestrator-script.agent.md", "Orchestrator.MD"} {
		t.Run(name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, roleCase, true, withFileName(name)))

			assertRole(t, res, out, domain.RoleOrchestrator)
		})
	}
}

func TestRebuildRole_OrchestratorFileNameWinsOverCatalog(t *testing.T) {
	cat := MockCatalogAgentLookup{ByKey: map[string]domain.Agent{"orchestrator": {Key: "orchestrator", Role: domain.RoleStandalone}}}

	res, out := rbRebuild(t, rbInput(t, roleCase, true, withFileName("orchestrator.md"), withCatalogLookup(cat)))

	assertRole(t, res, out, domain.RoleOrchestrator)
}

func TestRebuildRole_NameOnlySimilarToOrchestratorIsNotOrchestrator(t *testing.T) {
	res, out := rbRebuild(t, rbInput(t, roleCase, true, withFileName("orchestrator-script-custom.md")))

	assertRole(t, res, out, domain.RoleSubagent)
}

func TestRebuildRole_CatalogEntryByIDGivesCatalogRoleAndRefKey(t *testing.T) {
	cat := MockCatalogAgentLookup{
		ByID:  map[string]domain.Agent{"55": {Key: "code-reviewer-catalog", Role: domain.RoleStandalone}},
		ByKey: map[string]domain.Agent{"code-reviewer": {Key: "code-reviewer", Role: domain.RoleUtility}},
	}

	res, out := rbRebuild(t, rbInput(t, roleCase, false, withoutRoleStamp, withCatalogLookup(cat)))

	assertRole(t, res, out, domain.RoleStandalone)
	if res.RefKey != "code-reviewer-catalog" {
		t.Errorf("RefKey: want the catalog entry key found by id, got %q", res.RefKey)
	}
	if rbHasKey(t, out, "mosaic_role") {
		t.Errorf("mosaic_role must not be invented from the catalog role:\n%s", out)
	}
}

func TestRebuildRole_LegacyRoleKeyIsAccepted(t *testing.T) {
	legacy := withSourceEdit(replaceText("mosaic_role: subagent", "role: utility"))

	res, out := rbRebuild(t, rbInput(t, roleCase, false, legacy))

	assertRole(t, res, out, domain.RoleUtility)
}

func TestRebuildRole_UnknownIDDoesNotFallBackToKeyEntry(t *testing.T) {
	cat := MockCatalogAgentLookup{
		ByKey: map[string]domain.Agent{"code-reviewer": {Key: "code-reviewer", Role: domain.RoleStandalone}},
	}
	unknownID := withSourceEdit(replaceText("mosaic_id: 55", "mosaic_id: 999"))

	res, out := rbRebuild(t, rbInput(t, roleCase, false, withoutRoleStamp, unknownID, withCatalogLookup(cat)))

	assertRole(t, res, out, domain.RoleSubagent)
	if res.RefKey != "code-reviewer" {
		t.Errorf("RefKey: want the agent key when the id is unknown, got %q", res.RefKey)
	}
}

func TestRebuildRole_CatalogEntryByKeyWhenSourceHasNoID(t *testing.T) {
	cat := MockCatalogAgentLookup{ByKey: map[string]domain.Agent{"notes-helper": {Key: "notes-helper", Role: domain.RoleUtility}}}

	res, out := rbRebuild(t, rbInput(t, roleCase, true, withCatalogLookup(cat)))

	assertRole(t, res, out, domain.RoleUtility)
	if res.RefKey != "notes-helper" {
		t.Errorf("RefKey: want notes-helper, got %q", res.RefKey)
	}
}

func TestRebuildRole_FallbackIsSubagent(t *testing.T) {
	cases := map[string]struct {
		stampless bool
		opts      []rbOpt
	}{
		"no catalog":        {true, nil},
		"empty catalog":     {true, []rbOpt{withCatalogLookup(MockCatalogAgentLookup{})}},
		"id missing in cat": {false, []rbOpt{withoutRoleStamp, withCatalogLookup(MockCatalogAgentLookup{})}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, roleCase, c.stampless, c.opts...))

			assertRole(t, res, out, domain.RoleSubagent)
			if res.RefKey == "" {
				t.Errorf("RefKey must fall back to the agent key, got empty")
			}
		})
	}
}

func TestRebuildRole_NoRoleStampWrittenForSourceWithoutOne(t *testing.T) {
	cat := MockCatalogAgentLookup{ByKey: map[string]domain.Agent{"notes-helper": {Key: "notes-helper", Role: domain.RoleStandalone}}}
	cases := map[string][]rbOpt{
		"orchestrator by file name": {withFileName("orchestrator.md")},
		"standalone by catalog":     {withCatalogLookup(cat)},
		"subagent by fallback":      nil,
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, roleCase, true, opts...))

			if rbHasKey(t, out, "mosaic_role") {
				t.Errorf("mosaic_role must not be written for a source that carried none:\n%s", out)
			}
		})
	}
}

func TestRebuildRole_CatalogToolsResolveAmbiguousReverseMapping(t *testing.T) {
	// GHCP CLI maps both file_search and content_search to "search". The catalog's generic
	// tool list says which one the agent uses; without a catalog entry the first candidate wins.
	pair := rbPairs[3]
	narrowToSearch := withSourceEdit(replaceText("tools: ['read', 'ask_user']", "tools: ['search']"))
	cat := MockCatalogAgentLookup{ByKey: map[string]domain.Agent{
		"notes-helper": {Key: "notes-helper", Role: domain.RoleSubagent, Tools: []string{"content_search"}},
	}}

	_, outCat := rbRebuild(t, rbInput(t, pair, true, narrowToSearch, withCatalogLookup(cat)))
	_, outNone := rbRebuild(t, rbInput(t, pair, true, narrowToSearch))

	if got := rbToolNames(t, outCat, "tools"); !equalStrings(got, []string{"Grep"}) {
		t.Errorf("with a catalog entry listing content_search: want [Grep], got %v", got)
	}
	if got := rbToolNames(t, outNone, "tools"); !equalStrings(got, []string{"Glob"}) {
		t.Errorf("without a catalog entry the first reverse candidate (file_search) wins: want [Glob], got %v", got)
	}
}
