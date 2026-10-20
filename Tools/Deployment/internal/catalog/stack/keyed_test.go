package stack_test

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStack_Agent_HigherLayerWinsAcrossRoles(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "9.0.0")
	addTestSubagent(t, low, "Cat", "worker", "1.0.0")
	addTestUtility(t, high, "util", "9.0.0")
	addTestUtility(t, low, "util", "1.0.0")
	addTestStandalone(t, high, "solo", "9.0.0")
	addTestStandalone(t, low, "solo", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	for _, key := range []string{"worker", "util", "solo"} {
		a, ok := c.Agent(key)
		if !ok {
			t.Fatalf("Agent(%q) not found", key)
		}
		if a.Version != "9.0.0" || !underRoot(a.SourcePath, high) {
			t.Errorf("Agent(%q) = version %s path %s, want the high layer's", key, a.Version, a.SourcePath)
		}
	}
	if n := len(c.Agents()); n != 1 {
		t.Errorf("Agents() has %d workers, want 1 (no duplicate for the shadowed key)", n)
	}
	if n := len(c.UtilityAgents()); n != 1 {
		t.Errorf("UtilityAgents() has %d, want 1", n)
	}
	if n := len(c.StandaloneAgents()); n != 1 {
		t.Errorf("StandaloneAgents() has %d, want 1", n)
	}
}

func TestStack_Agent_KeyResolvesAcrossRolesBetweenLayers(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "shared", "9.0.0")
	addTestUtility(t, low, "shared", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	a, ok := c.Agent("shared")
	if !ok || !underRoot(a.SourcePath, high) {
		t.Fatalf("Agent(shared) = %+v, %v; want the high layer's worker", a, ok)
	}
	if n := len(c.UtilityAgents()); n != 0 {
		t.Errorf("UtilityAgents() has %d entries, want 0: the lower utility is shadowed by key", n)
	}
}

func TestStack_Items_PresentInOneLayerAreAllPresent(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "alpha", "1.0.0")
	addTestSubagent(t, low, "Other", "beta", "1.0.0")
	addTestUtility(t, low, "util", "1.0.0")
	addTestStandalone(t, high, "solo", "1.0.0")
	addTestSkill(t, high, "skill-a", "1.0.0")
	addTestSkill(t, low, "skill-b", "1.0.0")
	addTestHook(t, high, "hook-a", "h1")
	addTestHook(t, low, "hook-b", "h1")
	addTestWorkflow(t, high, "Build", "wf-a", "1.0.0")
	addTestWorkflow(t, low, "Ship", "wf-b", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	var keys []string
	for _, a := range c.Agents() {
		keys = append(keys, a.Key)
	}
	if !reflect.DeepEqual(keys, []string{"alpha", "beta"}) {
		t.Errorf("Agents() keys = %v, want [alpha beta] sorted", keys)
	}
	if len(c.UtilityAgents()) != 1 || len(c.StandaloneAgents()) != 1 {
		t.Errorf("utility/standalone counts = %d/%d, want 1/1", len(c.UtilityAgents()), len(c.StandaloneAgents()))
	}
	if len(c.Skills()) != 2 || len(c.Hooks()) != 2 || len(c.Workflows()) != 2 {
		t.Errorf("skills/hooks/workflows = %d/%d/%d, want 2/2/2", len(c.Skills()), len(c.Hooks()), len(c.Workflows()))
	}
}

func TestStack_Workflow_HigherLayerWinsPerID(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Build", "flow", "9.0.0")
	addTestWorkflow(t, low, "Build", "flow", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	w, ok := c.Workflow("flow")
	if !ok || w.Version != "9.0.0" || !underRoot(w.SourcePath, high) {
		t.Fatalf("Workflow(flow) = %+v, %v; want the high layer's", w, ok)
	}
	if n := len(c.Workflows()); n != 1 {
		t.Errorf("Workflows() has %d entries, want 1", n)
	}
}

func TestStack_WorkflowCategories_GroupSameNamedFoldersAcrossLayers(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Build", "b-two", "1.0.0")
	addTestWorkflow(t, low, "Build", "b-one", "1.0.0")
	addTestWorkflow(t, low, "Alpha", "a-one", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	cats := c.WorkflowCategories()
	if len(cats) != 2 || cats[0].Name != "Alpha" || cats[1].Name != "Build" {
		t.Fatalf("categories = %+v, want [Alpha Build]", cats)
	}
	var ids []string
	for _, w := range cats[1].Workflows {
		ids = append(ids, w.ID)
	}
	if !reflect.DeepEqual(ids, []string{"b-one", "b-two"}) {
		t.Errorf("Build workflow ids = %v, want [b-one b-two] ascending", ids)
	}
}

func TestStack_Workflow_ShadowedIDLeavesLowerCategory(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Ship", "flow", "9.0.0")
	addTestWorkflow(t, low, "Build", "flow", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	for _, cat := range c.WorkflowCategories() {
		if cat.Name == "Build" {
			t.Errorf("category Build still listed with %d workflows; its only workflow is shadowed", len(cat.Workflows))
		}
	}
	if w, _ := c.Workflow("flow"); w.Category != "Ship" {
		t.Errorf("Workflow(flow).Category = %q, want Ship", w.Category)
	}
}

func TestStack_Skill_ShadowedSkillDeploysOnlyWinnersFiles(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSkill(t, high, "sk", "9.0.0", "high-only.md")
	addTestSkill(t, low, "sk", "1.0.0", "low-only.md", filepath.Join("extra", "deep.md"))

	c := newTestStack(t, mosaic, high, low)

	s, ok := c.Skill("sk")
	if !ok || s.Version != "9.0.0" || !underRoot(s.SourceDir, high) {
		t.Fatalf("Skill(sk) = %+v, %v; want the high layer's", s, ok)
	}
	if !reflect.DeepEqual(s.Files, []string{"SKILL.md", "high-only.md"}) {
		t.Errorf("Skill(sk).Files = %v, want only the winner's files", s.Files)
	}
	if n := len(c.Skills()); n != 1 {
		t.Errorf("Skills() has %d entries, want 1", n)
	}
	if _, err := c.ReadSource(filepath.Join(low, "Skills", "sk", "low-only.md")); err == nil {
		t.Error("ReadSource served a file of the shadowed lower-layer skill")
	}
	if _, err := c.ReadSource(filepath.Join(high, "Skills", "sk", "high-only.md")); err != nil {
		t.Errorf("ReadSource of the winner's file: %v", err)
	}
}

func TestStack_Hook_ShadowedHookKeepsOnlyWinnersVariants(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestHook(t, high, "hk", "harness-a")
	addTestHook(t, low, "hk", "harness-a", "harness-b")

	c := newTestStack(t, mosaic, high, low)

	h, ok := c.Hook("hk")
	if !ok || !underRoot(h.SourceDir, high) {
		t.Fatalf("Hook(hk) = %+v, %v; want the high layer's", h, ok)
	}
	if _, has := h.Variants["harness-b"]; has || len(h.Variants) != 1 {
		t.Errorf("Hook(hk) variants = %v, want only harness-a: lower-layer variants are never merged in", h.Variants)
	}
	if n := len(c.Hooks()); n != 1 {
		t.Errorf("Hooks() has %d entries, want 1", n)
	}
	if _, err := c.ReadSource(filepath.Join(low, "Hooks", "hk", "harness-b", "harness-b-hook.sh")); err == nil {
		t.Error("ReadSource served a variant file of the shadowed lower-layer hook")
	}
}

func TestStack_ReadSource_ServesOnlyWinningItemPaths(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "9.0.0")
	addTestSubagent(t, low, "Cat", "worker", "1.0.0")
	addTestWorkflow(t, high, "Build", "flow", "9.0.0")
	addTestWorkflow(t, low, "Build", "flow", "1.0.0")
	addTestSubagent(t, low, "Cat", "low-only", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	winners := []string{
		filepath.Join(high, "Subagents", "Cat", "worker.md"),
		filepath.Join(high, "Workflows", "Build", "flow.md"),
		filepath.Join(low, "Subagents", "Cat", "low-only.md"),
	}
	for _, p := range winners {
		if _, err := c.ReadSource(p); err != nil {
			t.Errorf("ReadSource(%s): %v", p, err)
		}
	}
	shadowed := []string{
		filepath.Join(low, "Subagents", "Cat", "worker.md"),
		filepath.Join(low, "Workflows", "Build", "flow.md"),
	}
	for _, p := range shadowed {
		if _, err := c.ReadSource(p); err == nil {
			t.Errorf("ReadSource(%s) succeeded for a shadowed item", p)
		}
	}
}

func TestStack_Tiers_ComputedOverResolvedSetOnly(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagentWithTier(t, high, "Cat", "worker", "1.0.0", "deep")
	addTestSubagentWithTier(t, low, "Cat", "worker", "1.0.0", "fast")
	addTestSubagentWithTier(t, low, "Cat", "other", "1.0.0", "fast")

	c := newTestStack(t, mosaic, high, low)

	got := map[string][]string{}
	for _, ti := range c.Tiers() {
		got[string(ti.Tier)] = ti.AgentKeys
	}
	want := map[string][]string{"deep": {"worker"}, "fast": {"other"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tiers() = %v, want %v (a shadowed agent must not count toward its tier)", got, want)
	}
}

func TestStack_AgentByNumericID_ResolvesWinners(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestSubagent(t, high, "Cat", "worker", "9.0.0")
	addTestSubagent(t, low, "Cat", "worker", "1.0.0")
	addTestSubagent(t, low, "Cat", "low-only", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	if a, ok := c.AgentByNumericID("worker-id"); !ok || a.Version != "9.0.0" {
		t.Errorf("AgentByNumericID(worker-id) = %+v, %v; want the high layer's agent", a, ok)
	}
	if a, ok := c.AgentByNumericID("low-only-id"); !ok || a.Key != "low-only" {
		t.Errorf("AgentByNumericID(low-only-id) = %+v, %v", a, ok)
	}
}

func TestStack_References_CrossLayers(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	high, low := newTestCatalogRoot(t, "high"), newTestCatalogRoot(t, "low")
	addTestWorkflow(t, high, "Build", "flow", "1.0.0", "base-agent")
	addTestSubagentRequiringSkill(t, low, "Cat", "base-agent", "base-skill")
	addTestSkill(t, low, "base-skill", "1.0.0")

	c := newTestStack(t, mosaic, high, low)

	w, ok := c.Workflow("flow")
	if !ok || len(w.ReferencedAgents) != 1 {
		t.Fatalf("Workflow(flow) = %+v, %v", w, ok)
	}
	a, ok := c.Agent(w.ReferencedAgents[0])
	if !ok || !underRoot(a.SourcePath, low) || len(a.RequiredSkills) != 1 {
		t.Fatalf("referenced agent %q not resolved from the lower layer: %+v, %v", w.ReferencedAgents[0], a, ok)
	}
	if _, ok := c.Skill(a.RequiredSkills[0]); !ok {
		t.Errorf("required skill %q of the lower-layer agent does not resolve", a.RequiredSkills[0])
	}
}

func TestStack_Keyed_ThreeLayersResolveByPrecedence(t *testing.T) {
	mosaic := newTestMosaicRoot(t)
	top, middle, bottom := newTestCatalogRoot(t, "top"), newTestCatalogRoot(t, "middle"), newTestCatalogRoot(t, "bottom")
	addTestSubagent(t, top, "Cat", "in-all", "9.0.0")
	addTestSubagent(t, middle, "Cat", "in-all", "5.0.0")
	addTestSubagent(t, bottom, "Cat", "in-all", "1.0.0")
	addTestSubagent(t, middle, "Cat", "middle-bottom", "5.0.0")
	addTestSubagent(t, bottom, "Cat", "middle-bottom", "1.0.0")
	addTestSkill(t, middle, "sk", "5.0.0")
	addTestSkill(t, bottom, "sk", "1.0.0")
	addTestWorkflow(t, middle, "Build", "flow", "5.0.0")
	addTestWorkflow(t, bottom, "Build", "flow", "1.0.0")

	c := newTestStack(t, mosaic, top, middle, bottom)

	if a, _ := c.Agent("in-all"); a.Version != "9.0.0" || !underRoot(a.SourcePath, top) {
		t.Errorf("Agent(in-all) = version %s path %s, want the top layer's", a.Version, a.SourcePath)
	}
	if a, _ := c.Agent("middle-bottom"); a.Version != "5.0.0" || !underRoot(a.SourcePath, middle) {
		t.Errorf("Agent(middle-bottom) = version %s path %s, want the middle layer's", a.Version, a.SourcePath)
	}
	if s, _ := c.Skill("sk"); s.Version != "5.0.0" || !underRoot(s.SourceDir, middle) {
		t.Errorf("Skill(sk) = %+v, want the middle layer's", s)
	}
	if w, _ := c.Workflow("flow"); w.Version != "5.0.0" || !underRoot(w.SourcePath, middle) {
		t.Errorf("Workflow(flow) = %+v, want the middle layer's", w)
	}
	if n := len(c.Agents()); n != 2 {
		t.Errorf("Agents() has %d entries, want 2", n)
	}
}
