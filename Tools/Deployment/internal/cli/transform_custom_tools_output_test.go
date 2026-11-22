package cli_test

// transform_custom_tools_output_test.go verifies that the transform result shows every custom
// tool entry of an agent, in the terminal summary and in the JSON result. Regression and support
// coverage: the rendering exists and must keep listing a mixed answer's entries unchanged.

import (
	"encoding/json"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
)

func customToolsResult() app.TransformHarnessResult {
	res := planResult()
	res.Files[0].CustomTools = []string{"github/get_pr", "jira"}
	return res
}

func TestTransformCmd_Summary_ListsEveryCustomToolEntry(t *testing.T) {
	svc := &spyService{transformResp: customToolsResult()}

	_, out, _ := runTransform(svc, transformArgs("--yes")...)

	if !lineWithAll(out, "custom tools", "github/get_pr", "jira") {
		t.Errorf("no line lists both custom tool entries:\n%s", out)
	}
}

func TestTransformCmd_JSON_CarriesCustomToolEntriesAsAList(t *testing.T) {
	svc := &spyService{transformResp: customToolsResult()}

	_, out, _ := runTransform(svc, transformArgs("--output", "json", "--yes")...)

	var got struct {
		Files []struct {
			Key         string   `json:"key"`
			CustomTools []string `json:"customTools"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	for _, f := range got.Files {
		if f.Key == "writer" {
			if len(f.CustomTools) != 2 || f.CustomTools[0] != "github/get_pr" || f.CustomTools[1] != "jira" {
				t.Errorf("customTools = %q, want [github/get_pr jira]", f.CustomTools)
			}
			return
		}
	}
	t.Errorf("writer file missing from JSON:\n%s", out)
}
