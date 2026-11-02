package docformat_test

// Tests for the role rule on the orchestrator-only harness region
// (OrchestratorHarnessConstraints): a subagent document must not carry it, an
// orchestrator (or an unspecified role) may. The existing conduct-region rules
// must keep working unchanged.

import (
	"strings"
	"testing"

	"mosaic-common/docformat"
)

const orchestratorHarnessRegion = "OrchestratorHarnessConstraints"

// docWithOrchestratorHarnessRegion builds a document whose Constraints section carries the
// shared harness region and the orchestrator harness region with the given body.
func docWithOrchestratorHarnessRegion(t *testing.T, role, regionBody string) *docformat.Document {
	t.Helper()
	src := "---\nname: test-agent\nrole: " + role + "\n---\n\n" +
		"<Constraints type=\"core\">\n" +
		"<HarnessConstraints type=\"managed\">\n</HarnessConstraints>\n\n" +
		"<OrchestratorHarnessConstraints type=\"managed\">\n" + regionBody +
		"</OrchestratorHarnessConstraints>\n" +
		"</Constraints>\n"
	return parseInlineDoc(t, src)
}

func orchestratorRegionRoleIssues(issues []docformat.Issue) []docformat.Issue {
	var out []docformat.Issue
	for _, iss := range issues {
		if iss.Code == "role-forbidden-region" && iss.Node == orchestratorHarnessRegion {
			out = append(out, iss)
		}
	}
	return out
}

func TestValidate_RoleRegion_DocumentWithRegionParsesWithoutUnknownNameIssue(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "orchestrator", "- some bullet\n")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "orchestrator"})

	for _, iss := range issues {
		if iss.Code == "unknown-deployed" || iss.Code == "wrong-parent" {
			t.Errorf("orchestrator harness region produced unexpected issue: code=%q message=%q", iss.Code, iss.Message)
		}
	}
}

func TestValidate_RoleRegion_SubagentWithPopulatedRegion_ReportsWarning(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "subagent", "- some bullet\n")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "subagent"})

	got := orchestratorRegionRoleIssues(issues)
	if len(got) != 1 {
		t.Fatalf("want exactly one role-forbidden-region issue for %s, got %d; issues: %v",
			orchestratorHarnessRegion, len(got), issues)
	}
	if got[0].Severity != docformat.SeverityWarning {
		t.Errorf("severity: want SeverityWarning, got %q", got[0].Severity)
	}
	if !strings.Contains(got[0].Message, orchestratorHarnessRegion) {
		t.Errorf("message should name the region %q, got %q", orchestratorHarnessRegion, got[0].Message)
	}
}

func TestValidate_RoleRegion_SubagentWithEmptyRegion_ReportsWarning(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "subagent", "")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "subagent"})

	if got := orchestratorRegionRoleIssues(issues); len(got) != 1 {
		t.Fatalf("want exactly one role-forbidden-region issue for an empty %s region, got %d; issues: %v",
			orchestratorHarnessRegion, len(got), issues)
	}
}

func TestValidate_RoleRegion_SubagentStrict_PromotesToError(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "subagent", "")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "subagent", Strict: true})

	got := orchestratorRegionRoleIssues(issues)
	if len(got) != 1 {
		t.Fatalf("want exactly one role-forbidden-region issue, got %d; issues: %v", len(got), issues)
	}
	if got[0].Severity != docformat.SeverityError {
		t.Errorf("strict severity: want SeverityError, got %q", got[0].Severity)
	}
}

func TestValidate_RoleRegion_OrchestratorWithRegion_NoIssue(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "orchestrator", "- some bullet\n")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "orchestrator"})

	if got := orchestratorRegionRoleIssues(issues); len(got) != 0 {
		t.Errorf("orchestrator must be allowed to carry %s; got issues: %v", orchestratorHarnessRegion, got)
	}
}

func TestValidate_RoleRegion_UnspecifiedRole_NoIssue(t *testing.T) {
	doc := docWithOrchestratorHarnessRegion(t, "subagent", "")

	issues := docformat.Validate(doc, docformat.ValidateOptions{})

	if got := orchestratorRegionRoleIssues(issues); len(got) != 0 {
		t.Errorf("no role-forbidden-region issue expected without ValidateOptions.Role; got: %v", got)
	}
}

func TestValidate_RoleRegion_SubagentWithoutRegion_NoIssue(t *testing.T) {
	src := "---\nname: test-agent\nrole: subagent\n---\n\n" +
		"<Constraints type=\"core\">\n" +
		"<HarnessConstraints type=\"managed\">\n</HarnessConstraints>\n" +
		"</Constraints>\n"
	doc := parseInlineDoc(t, src)

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "subagent"})

	if got := orchestratorRegionRoleIssues(issues); len(got) != 0 {
		t.Errorf("subagent without the region must not be flagged; got: %v", got)
	}
}

func TestValidate_RoleRegion_SubagentConductRegionRulesUnchanged(t *testing.T) {
	// A subagent lacking the conduct regions still gets missing-conduct-region, whether or
	// not it also carries the orchestrator harness region.
	doc := docWithOrchestratorHarnessRegion(t, "subagent", "")

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "subagent"})

	if !hasIssueWithCode(issues, "missing-conduct-region") {
		t.Errorf("expected missing-conduct-region to keep firing for a subagent; issues: %v", issues)
	}
}

func TestValidate_RoleRegion_OrchestratorConductRegionRuleUnchanged(t *testing.T) {
	// An orchestrator carrying a subagent conduct region is still flagged for that region.
	src := "---\nname: orchestrator\nrole: orchestrator\n---\n\n" +
		"<Identity type=\"core\">\n<AuthorityHierarchy type=\"managed\">\n</AuthorityHierarchy>\n</Identity>\n"
	doc := parseInlineDoc(t, src)

	issues := docformat.Validate(doc, docformat.ValidateOptions{Role: "orchestrator"})

	found := false
	for _, iss := range issues {
		if iss.Code == "role-forbidden-region" && iss.Node == "AuthorityHierarchy" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected role-forbidden-region for AuthorityHierarchy on an orchestrator; issues: %v", issues)
	}
}
