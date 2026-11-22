package app_test

// tools_update_custom_entries_test.go verifies that an update keeps every custom tool entry a
// deployed agent already holds (several servers and single tools, in their deployed order), and
// that it does not ask the custom tool question to do so.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestUpdateTools_DeployedCustomEntries_AllSurviveAndNothingIsAsked(t *testing.T) {
	deployed := tuDeployed("0.9.0", "[read, github/get_pr, github/list_comments, jira]", "")

	run := runToolsUpdate(t, deployed, nil)

	requireToolsRunItem(t, run)
	line := toolsLine(run.written(t))
	for _, want := range []string{"github/get_pr", "github/list_comments", "jira", "edit"} {
		if !strings.Contains(line, want) {
			t.Errorf("written tools line %q lost %q", line, want)
		}
	}
	for _, c := range run.calls {
		if c.ID == domain.QCustomTool {
			t.Errorf("the update asked the custom tool question for %q", c.Subject)
		}
	}
}
