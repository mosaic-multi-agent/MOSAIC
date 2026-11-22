package screens_test

// transform_summary_custom_tools_test.go verifies that the transform summary screen shows every
// custom tool entry of an agent. Regression and support coverage: the rendering exists and must
// keep listing a mixed answer's entries unchanged.

import (
	"strings"
	"testing"
)

func TestTransformSummaryScreen_View_ListsEveryCustomToolEntry(t *testing.T) {
	result := tcDeploymentResult()
	result.Files[0].CustomTools = []string{"github/get_pr", "jira"}

	raw := tsView(result, plainStyles())

	if !tcLineWithAll(raw, "custom tools", "github/get_pr", "jira") {
		t.Errorf("no line lists both custom tool entries:\n%s", strings.TrimSpace(raw))
	}
}
