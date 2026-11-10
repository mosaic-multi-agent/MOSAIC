package stack_test

import (
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
)

// identityOf is the catalog identity expected on issues raised in a newTestCatalogRoot layer.
func identityOf(id string) domain.CatalogIdentity {
	return domain.CatalogIdentity{ID: id, Name: "Catalog " + id, Version: "1.0.0"}
}

// addTestAgentWithInvalidRole writes a worker agent whose frontmatter role is not recognised,
// so loading its layer reports an invalid-role issue.
func addTestAgentWithInvalidRole(t *testing.T, root, key string) {
	t.Helper()
	mustWrite(t, root, filepath.Join("Subagents", "Cat", key+".md"),
		"---\nid: \""+key+"-id\"\nversion: \"1.0.0\"\nname: "+key+"\ndescription: Test agent.\nrole: bogus-role\nrequired_skills: []\n---\nBody.\n")
}

// issuesWithCode returns the issues of c carrying code.
func issuesWithCode(issues []catalog.Issue, code string) []catalog.Issue {
	var out []catalog.Issue
	for _, i := range issues {
		if i.Code == code {
			out = append(out, i)
		}
	}
	return out
}

func TestStack_Issues_LayerIssueIdentifiesItsCatalog(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "fine", "1.0.0")
	addTestTamperedHook(t, low, "bad")

	issues := newTestStack(t, mosaic, high, low).Issues()

	if len(issues) != 1 {
		t.Fatalf("Issues() = %+v, want exactly the low layer's one issue", issues)
	}
	if issues[0].Code != "hook-hash-mismatch" {
		t.Errorf("Code = %q, want hook-hash-mismatch", issues[0].Code)
	}
	if issues[0].Catalog != identityOf("low") {
		t.Errorf("Catalog = %+v, want the low layer's identity %+v", issues[0].Catalog, identityOf("low"))
	}
	if !underRoot(issues[0].Path, low) {
		t.Errorf("Path = %q, want it to stay the offending file under %s", issues[0].Path, low)
	}
}

func TestStack_Issues_InvalidRoleIsAttributedToItsLayer(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestAgentWithInvalidRole(t, high, "odd")
	addTestSubagent(t, low, "Cat", "fine", "1.0.0")

	issues := issuesWithCode(newTestStack(t, mosaic, high, low).Issues(), "invalid-role")

	if len(issues) != 1 {
		t.Fatalf("invalid-role issues = %+v, want exactly one", issues)
	}
	if issues[0].Catalog != identityOf("high") {
		t.Errorf("Catalog = %+v, want the high layer's identity", issues[0].Catalog)
	}
}

func TestStack_Issues_SameCodeInTwoLayersYieldsTwoAttributedIssues(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestTamperedHook(t, high, "bad-high")
	addTestTamperedHook(t, low, "bad-low")

	issues := newTestStack(t, mosaic, high, low).Issues()

	if len(issues) != 2 {
		t.Fatalf("Issues() = %+v, want one issue per layer", issues)
	}
	for i, id := range []string{"high", "low"} {
		if issues[i].Code != "hook-hash-mismatch" || issues[i].Catalog != identityOf(id) {
			t.Errorf("Issues()[%d] = %+v, want hook-hash-mismatch attributed to %s (layer order)", i, issues[i], id)
		}
	}
}

func TestStack_Issues_IssueOfShadowedItemStaysWithItsOwnCatalog(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestHook(t, high, "dup", "h1")
	addTestTamperedHook(t, low, "dup")
	addTestAgentWithInvalidRole(t, low, "shadowed-odd")
	addTestSubagent(t, high, "Cat", "shadowed-odd", "2.0.0")
	c := newTestProvenanceStack(t, mosaic, high, low)

	issues := c.Issues()

	hook := issuesWithCode(issues, "hook-hash-mismatch")
	role := issuesWithCode(issues, "invalid-role")
	if len(hook) != 1 || hook[0].Catalog != identityOf("low") {
		t.Errorf("hook-hash-mismatch issues = %+v, want one attributed to low, not to the winner", hook)
	}
	if len(role) != 1 || role[0].Catalog != identityOf("low") {
		t.Errorf("invalid-role issues = %+v, want one attributed to low, not to the winner", role)
	}
	if rec := shadowFor(t, c.Shadows(), domain.CatalogItem{Kind: domain.CatalogItemHook, Key: "dup"}); rec.Winner.Identity.ID != "high" {
		t.Errorf("precondition: hook dup winner = %q, want high", rec.Winner.Identity.ID)
	}
}

func TestStack_Issues_PlainLoadStaysUnattributed(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "only")
	addTestTamperedHook(t, root, "bad")

	plain, err := catalog.LoadSingleRoot(mosaic, root)
	if err != nil {
		t.Fatalf("LoadSingleRoot: %v", err)
	}

	issues := plain.Issues()
	if len(issues) != 1 || issues[0].Catalog != (domain.CatalogIdentity{}) {
		t.Errorf("Issues() = %+v, want one issue with a zero Catalog", issues)
	}
}

func TestStack_References_PerLayerLoadDoesNotCheckReferences(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "user")
	addTestSubagentRequiringSkill(t, root, "Cat", "needy", "elsewhere")

	plain, err := catalog.LoadSingleRoot(mosaic, root)
	if err != nil {
		t.Fatalf("LoadSingleRoot: %v", err)
	}

	if got := issuesWithCode(plain.Issues(), "missing-skill-folder"); len(got) != 0 {
		t.Errorf("single-root load reported %+v, want reference checks left to ValidateRegistry", got)
	}
}

func TestStack_References_SkillFromLowerLayerSatisfiesUserLayerAgent(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	user, base := newTestCatalogRoot(t, "user"), newTestCatalogRoot(t, "base")
	addTestSubagentRequiringSkill(t, user, "Cat", "needy", "base-skill")
	addTestSkill(t, base, "base-skill", "1.0.0")
	c := newTestStack(t, mosaic, user, base)

	issues := catalog.ValidateRegistry(c)

	if got := issuesWithCode(issues, "missing-skill-folder"); len(got) != 0 {
		t.Errorf("ValidateRegistry flagged %+v, want none: the skill is supplied by a lower layer", got)
	}
}

func TestStack_References_SkillFromHigherLayerSatisfiesLowerLayerAgent(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	user, base := newTestCatalogRoot(t, "user"), newTestCatalogRoot(t, "base")
	addTestSkill(t, user, "user-skill", "1.0.0")
	addTestSubagentRequiringSkill(t, base, "Cat", "needy", "user-skill")
	c := newTestStack(t, mosaic, user, base)

	issues := catalog.ValidateRegistry(c)

	if got := issuesWithCode(issues, "missing-skill-folder"); len(got) != 0 {
		t.Errorf("ValidateRegistry flagged %+v, want none: the skill is supplied by a higher layer", got)
	}
}

func TestStack_References_SkillNoLayerSuppliesIsFlaggedAgainstRequiringAgentsCatalog(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	user, base := newTestCatalogRoot(t, "user"), newTestCatalogRoot(t, "base")
	addTestSubagentRequiringSkill(t, user, "Cat", "user-needy", "ghost")
	addTestSubagentRequiringSkill(t, base, "Cat", "base-needy", "phantom")
	c := newTestStack(t, mosaic, user, base)

	issues := issuesWithCode(catalog.ValidateRegistry(c), "missing-skill-folder")

	if len(issues) != 2 {
		t.Fatalf("missing-skill-folder issues = %+v, want one per unsatisfied requirement", issues)
	}
	bySubject := map[string]domain.CatalogIdentity{}
	for _, i := range issues {
		bySubject[i.Subject] = i.Catalog
	}
	if bySubject["user-needy"] != identityOf("user") {
		t.Errorf("user-needy attributed to %+v, want the user catalog", bySubject["user-needy"])
	}
	if bySubject["base-needy"] != identityOf("base") {
		t.Errorf("base-needy attributed to %+v, want the base catalog", bySubject["base-needy"])
	}
}

func TestValidateRegistry_PlainCatalogIssuesStayUnattributed(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	root := newTestCatalogRoot(t, "only")
	addTestSubagentRequiringSkill(t, root, "Cat", "needy", "ghost")
	plain, err := catalog.LoadSingleRoot(mosaic, root)
	if err != nil {
		t.Fatalf("LoadSingleRoot: %v", err)
	}

	issues := issuesWithCode(catalog.ValidateRegistry(plain), "missing-skill-folder")

	if len(issues) != 1 || issues[0].Catalog != (domain.CatalogIdentity{}) {
		t.Errorf("ValidateRegistry on a plain catalog = %+v, want one issue with a zero Catalog", issues)
	}
}
