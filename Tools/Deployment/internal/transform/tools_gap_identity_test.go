package transform_test

// tools_gap_identity_test.go verifies the identity fields of the unmapped-tool gaps the
// transform reports: Subject is the generic tool name and Owner is the agent key, for both
// an unmapped tool and a tool the user skipped. The TODO collector relies on this to treat
// the transform's gap and the planner's gap for the same tool as one.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

func unmappedToolGapOf(t *testing.T, key string, skipped map[string]bool) domain.Gap {
	t.Helper()
	result, err := transform.Apply(transform.Request{
		Source:       []byte(unmappedToolSource),
		Kind:         domain.ArtifactAgent,
		Key:          key,
		Module:       newFixtureModule(t),
		Model:        fixtureModel(),
		SkippedTools: skipped,
		Scope:        domain.ScopeProject,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, g := range result.Report.Gaps {
		if g.Kind == domain.GapUnmappedTool {
			return g
		}
	}
	t.Fatalf("no GapUnmappedTool in Report.Gaps; kinds: %v", gapKinds(result.Report.Gaps))
	return domain.Gap{}
}

// TestApply_UnmappedToolGap_SubjectIsToolAndOwnerIsAgent verifies the gap identity of an
// unmapped tool.
func TestApply_UnmappedToolGap_SubjectIsToolAndOwnerIsAgent(t *testing.T) {
	g := unmappedToolGapOf(t, "identity-agent", nil)

	if g.Subject != "skill" {
		t.Errorf("Subject = %q, want the generic tool name %q", g.Subject, "skill")
	}
	if g.Owner != "identity-agent" {
		t.Errorf("Owner = %q, want the agent key %q", g.Owner, "identity-agent")
	}
}

// TestApply_SkippedToolGap_SubjectIsToolAndOwnerIsAgent verifies the gap identity of a tool
// the user chose to skip.
func TestApply_SkippedToolGap_SubjectIsToolAndOwnerIsAgent(t *testing.T) {
	g := unmappedToolGapOf(t, "identity-agent", map[string]bool{"skill": true})

	if g.Subject != "skill" {
		t.Errorf("Subject = %q, want the generic tool name %q", g.Subject, "skill")
	}
	if g.Owner != "identity-agent" {
		t.Errorf("Owner = %q, want the agent key %q", g.Owner, "identity-agent")
	}
}
