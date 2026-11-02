package app_test

// role_region_fixtures_test.go holds the shared sources and helpers for the tests that specify
// the role-forbidden-region warning in deploy and Update workspace output: a non-orchestrator
// agent that carries the orchestrator harness region is flagged with a warning and the run
// continues.

import (
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

const roleRegionAgentKey = "test-runner"

const roleRegionSubagentHead = `---
role: subagent
version: "1.0"
name: Test Runner
description: Subagent for role region tests.
tools: []
---

<Identity type="core">
You are the test runner.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

`

// roleRegionSubagentWithRegion is a subagent source that carries the orchestrator region.
const roleRegionSubagentWithRegion = roleRegionSubagentHead + `<HarnessConstraints type="managed">
</HarnessConstraints>

<OrchestratorHarnessConstraints type="managed">
</OrchestratorHarnessConstraints>
</Constraints>
`

// roleRegionValidSubagent is a subagent source without the orchestrator region.
const roleRegionValidSubagent = roleRegionSubagentHead + `<HarnessConstraints type="managed">
</HarnessConstraints>
</Constraints>
`

// roleRegionHarnessOnlyFile returns a deployed harness-only file of the given role. When
// withRegion is true it carries the orchestrator harness region.
func roleRegionHarnessOnlyFile(role string, withRegion bool) []byte {
	doc := "---\n" +
		"transform_version: \"1.0.0\"\n" +
		"role: " + role + "\n" +
		"---\n" +
		"<CommunicationProtocol type=\"managed\">\n" +
		"OLD PROTOCOL CONTENT\n" +
		"</CommunicationProtocol>\n" +
		"<Identity type=\"core\">\n" +
		"# Harness-Only Agent\n" +
		"</Identity>\n" +
		"<Constraints type=\"core\">\n" +
		"## Constraints\n" +
		"<HarnessConstraints type=\"managed\" version=\"1.4.0\">\n" +
		"</HarnessConstraints>\n"
	if withRegion {
		doc += "<OrchestratorHarnessConstraints type=\"managed\" version=\"2.3.0\">\n" +
			"</OrchestratorHarnessConstraints>\n"
	}
	return []byte(doc + "</Constraints>\n")
}

// roleForbiddenGaps returns the recorded role-forbidden-region gaps owned by an owner that
// contains ownerPart.
func roleForbiddenGaps(gaps []domain.Gap, ownerPart string) []domain.Gap {
	var out []domain.Gap
	for _, g := range gaps {
		if g.Kind == domain.GapRoleForbiddenRegion && strings.Contains(g.Owner, ownerPart) {
			out = append(out, g)
		}
	}
	return out
}

// assertOneRoleForbiddenWarning requires exactly one role-forbidden-region gap for the agent,
// about the orchestrator region and carrying the check's code.
func assertOneRoleForbiddenWarning(t *testing.T, ownerPart string, gaps []domain.Gap) {
	t.Helper()
	got := roleForbiddenGaps(gaps, ownerPart)
	if len(got) != 1 {
		t.Fatalf("%s: %d role-forbidden-region warnings in the output, want exactly 1 (gaps: %v)", ownerPart, len(got), gaps)
	}
	if got[0].Subject != docformat.OrchestratorHarnessRegionName {
		t.Errorf("%s: warning subject = %q, want %q", ownerPart, got[0].Subject, docformat.OrchestratorHarnessRegionName)
	}
	if !strings.Contains(got[0].Detail, "role-forbidden-region") {
		t.Errorf("%s: warning detail %q does not carry the role-forbidden-region code", ownerPart, got[0].Detail)
	}
}

// assertNoRoleForbiddenWarning requires that no role-forbidden-region gap is recorded at all.
func assertNoRoleForbiddenWarning(t *testing.T, gaps []domain.Gap) {
	t.Helper()
	for _, g := range gaps {
		if g.Kind == domain.GapRoleForbiddenRegion {
			t.Errorf("unexpected role-forbidden-region warning: %+v", g)
		}
	}
}

// assertNoGapsOwnedBy requires that no gap of any kind is recorded for the exact owner key.
func assertNoGapsOwnedBy(t *testing.T, gaps []domain.Gap, owners ...string) {
	t.Helper()
	for _, g := range gaps {
		for _, owner := range owners {
			if g.Owner == owner {
				t.Errorf("%s: unexpected warning: %+v", owner, g)
			}
		}
	}
}

// assertOnlyGapOwnedBy requires that the exact owner key has exactly one gap of any kind.
func assertOnlyGapOwnedBy(t *testing.T, gaps []domain.Gap, owner string) {
	t.Helper()
	n := 0
	for _, g := range gaps {
		if g.Owner == owner {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%s: %d warnings of any kind, want exactly 1 (only the orchestrator-region one): %v", owner, n, gaps)
	}
}

// roleRegionOrchestratorWithConductRegion is an orchestrator source that also carries a
// subagent conduct region, which docformat reports under the same code as the orchestrator
// region on a subagent. It must not produce the orchestrator-region warning.
const roleRegionOrchestratorWithConductRegion = stampSourceBothMarkers + `
<AuthorityHierarchy type="managed">
</AuthorityHierarchy>
`

// roleRegionConductBundleLoader serves an orchestrator block for the conduct region carried by
// roleRegionOrchestratorWithConductRegion, so the content callback can fill it.
func roleRegionConductBundleLoader() *stubBundleLoader {
	return &stubBundleLoader{content: domain.BundleContent{
		Version: "1.0.0",
		Blocks: []domain.BundleBlock{
			{Name: "AuthorityHierarchy:Orchestrator", Target: "AuthorityHierarchy", AppliesTo: "orchestrator", Content: []byte("authority content\n")},
		},
	}}
}

// roleRegionHarnessOnlyFileWithConduct is roleRegionHarnessOnlyFile (without the orchestrator
// region) plus a subagent conduct region.
func roleRegionHarnessOnlyFileWithConduct(role string) []byte {
	return append(roleRegionHarnessOnlyFile(role, false),
		[]byte("<AuthorityHierarchy type=\"managed\">\n</AuthorityHierarchy>\n")...)
}
