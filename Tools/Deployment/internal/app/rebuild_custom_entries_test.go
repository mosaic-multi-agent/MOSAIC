package app_test

// rebuild_custom_entries_test.go covers a custom tool answer of several MCP entries in the
// deployed-agent rebuild: every entry is written in the target's own form, and the answered
// tool is reported neither as unmapped nor as a gap.

import (
	"testing"

	"mosaic-deploy/internal/domain"
)

// Regression and support coverage, not evidence for new behavior: RebuildInput already takes the
// answered entries, and the carry-through from the transform run into the rebuild is specified
// by the transform tests (transform_custom_tool_list_test.go), which fail without it.
func TestRebuild_MixedAnswerForUnmappedTool_WritesEveryEntryAndReportsNoGap(t *testing.T) {
	answer := withCustomToolAnswers(domain.CustomToolAnswers{"web_search": {
		{Server: "github", Tool: "get_pr"}, {Server: "jira"},
	}})

	res, out := rbRebuild(t, rbInput(t, claudeToOpencode, true, withWebSearch, answer))

	names := rbToolNames(t, out, "permission")
	for _, want := range []string{"github_get_pr", "jira_*"} {
		if countName(names, want) != 1 {
			t.Errorf("entry %q must be allowed once in the target's form, got %v", want, names)
		}
	}
	if len(res.UnmappedTools) != 0 {
		t.Errorf("an answered tool must not stay unmapped, got %v", res.UnmappedTools)
	}
	if n := countGaps(res.Gaps, domain.GapUnmappedTool); n != 0 {
		t.Errorf("an answered tool must have no unmapped-tool gap, got %d in %+v", n, res.Gaps)
	}
}
