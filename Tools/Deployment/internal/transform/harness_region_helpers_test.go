package transform_test

// harness_region_helpers_test.go holds the shared sources, stub modules and apply helpers for
// the per-region harness injection tests (two-region orchestrator output, missing-region
// warning, merged-only external module).

import (
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const (
	sharedRegionText = "SHARED_REGION_TEXT: shared harness constraint."
	orchRegionText   = "ORCH_REGION_TEXT: orchestrator harness constraint."

	sharedRegionVersion = "1.4.0"
	orchRegionVersion   = "2.3.0"
)

const regionSourceFrontmatter = `---
id: 80
version: 2.0.0
name: harness-region-test
description: Agent for per-region harness injection tests
model: {model-identifier}
tools: [file_read]
recommended_tier: LOW
tier_rationale: region testing
required_skills: []
---

<Identity type="core">
# RegionTest Agent

You are the RegionTest agent.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

`

// orchestratorSourceBothMarkers carries both harness-class markers, the shape an orchestrator
// catalog source has once it declares a second region.
const orchestratorSourceBothMarkers = regionSourceFrontmatter + `<HarnessConstraints type="managed">
</HarnessConstraints>

<OrchestratorHarnessConstraints type="managed">
</OrchestratorHarnessConstraints>
</Constraints>
`

// sourceSingleMarker carries only the shared harness marker: a subagent source, or an
// orchestrator source that has not been given a second marker.
const sourceSingleMarker = regionSourceFrontmatter + `<HarnessConstraints type="managed">
</HarnessConstraints>
</Constraints>
`

// deployedLegacyMergedOrchestrator is an orchestrator deployed before the split: one
// HarnessConstraints region holding merged text, tagged with a version, plus a user-owned
// custom region nested inside it.
const deployedLegacyMergedOrchestrator = `---
id: 80
version: 1.0.0
transform_version: 3.0.0
description: Agent for per-region harness injection tests
mode: primary
model: claude/claude-sonnet
tools: [read-file]
---

<Identity type="core">
# RegionTest Agent

You are the RegionTest agent.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

<HarnessConstraints type="managed" version="1.2.1">
OLD_SHARED_TEXT merged line.

OLD_ORCH_TEXT merged line.

<ConstraintsNote type="custom">
NESTED_USER_NOTE kept by the user.
</ConstraintsNote>
</HarnessConstraints>
</Constraints>
`

// regionModule wraps the fixture harness module and replaces only Injection.
type regionModule struct {
	domain.HarnessModule
	inject func(domain.InjectionRequest) (string, bool)
}

func (m *regionModule) Injection(req domain.InjectionRequest) (string, bool) {
	return m.inject(req)
}

// newSplitModule serves content per region name like a current harness module: the shared
// region for every role, the orchestrator region for the orchestrator role only. A false
// orchOK means the harness has no HarnessInjectionsOrchestrator.md.
func newSplitModule(t *testing.T, shared, orch string, orchOK bool) domain.HarnessModule {
	t.Helper()
	return &regionModule{HarnessModule: newFixtureModule(t), inject: func(req domain.InjectionRequest) (string, bool) {
		switch req.Name {
		case docformat.HarnessRegionName:
			return shared, true
		case docformat.OrchestratorHarnessRegionName:
			if req.Role == domain.RoleOrchestrator {
				return orch, orchOK
			}
		}
		return "", false
	}}
}

// newMergedOnlyModule behaves like an older external module that does not know the
// orchestrator region name: it serves shared and orchestrator text merged under
// HarnessConstraints for the orchestrator role and answers false for the new name.
func newMergedOnlyModule(t *testing.T, shared, orch string) domain.HarnessModule {
	t.Helper()
	return &regionModule{HarnessModule: newFixtureModule(t), inject: func(req domain.InjectionRequest) (string, bool) {
		if req.Name != docformat.HarnessRegionName {
			return "", false
		}
		if req.Role == domain.RoleOrchestrator {
			return shared + "\n\n" + orch, true
		}
		return shared, true
	}}
}

// regionApply runs transform.Apply with both injection versions populated and parses the output.
func regionApply(t *testing.T, mod domain.HarnessModule, source string, key string, role domain.AgentRole, deployed string) (*docformat.Document, transform.Result) {
	t.Helper()
	return regionApplyDeployed(t, mod, source, key, role, deployed, sharedRegionVersion, orchRegionVersion)
}

// regionApplyWithVersions runs a create transform with explicit injection versions.
func regionApplyWithVersions(t *testing.T, mod domain.HarnessModule, source string, key string, role domain.AgentRole, injV, orchV string) (*docformat.Document, transform.Result) {
	t.Helper()
	return regionApplyDeployed(t, mod, source, key, role, "", injV, orchV)
}

func regionApplyDeployed(t *testing.T, mod domain.HarnessModule, source string, key string, role domain.AgentRole, deployed, injV, orchV string) (*docformat.Document, transform.Result) {
	t.Helper()
	req := transform.Request{
		Source:                        []byte(source),
		Kind:                          domain.ArtifactAgent,
		Key:                           key,
		Role:                          role,
		Module:                        mod,
		Model:                         fixtureModel(),
		Scope:                         domain.ScopeProject,
		InjectionsVersion:             injV,
		OrchestratorInjectionsVersion: orchV,
	}
	if deployed != "" {
		req.Deployed = []byte(deployed)
	}
	result, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("transform.Apply: %v", err)
	}
	doc, err := docformat.Parse(result.Output)
	if err != nil {
		t.Fatalf("docformat.Parse output: %v", err)
	}
	return doc, result
}

// undeliveredGaps returns the gaps of kind GapUndeliveredOrchestratorInjection.
func undeliveredGaps(gaps []domain.Gap) []domain.Gap {
	var out []domain.Gap
	for _, g := range gaps {
		if g.Kind == domain.GapUndeliveredOrchestratorInjection {
			out = append(out, g)
		}
	}
	return out
}

func assertNoUndeliveredGap(t *testing.T, gaps []domain.Gap) {
	t.Helper()
	if got := undeliveredGaps(gaps); len(got) != 0 {
		t.Errorf("unexpected undelivered-orchestrator-injection gaps: %+v", got)
	}
}

// mustRegion returns the named managed region or fails the test.
func mustRegion(t *testing.T, doc *docformat.Document, name string) *docformat.Node {
	t.Helper()
	node, ok := doc.Body().Deployed(name)
	if !ok {
		t.Fatalf("region %s absent from output", name)
	}
	return node
}
