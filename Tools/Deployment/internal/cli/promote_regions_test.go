package cli_test

// promote_regions_test.go covers the CLI side of project and custom region handling in
// "promote":
//   - project region content is kept by default; --empty-project-regions empties it;
//   - promote never asks about it (the request is built from flags alone);
//   - human output lists each removed custom region directly after the stripped fields,
//     dry runs included;
//   - JSON output carries removedCustomRegions, omitted when empty.

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
)

const regionsAgentPath = "/path/to/agent.md"

func runPromoteWithRegions(t *testing.T, svc *spyService, extra ...string) (string, int) {
	t.Helper()
	args := append([]string{"promote", "--file", regionsAgentPath, "--harness", "stub-harness"}, extra...)
	out := &bytes.Buffer{}
	code := cli.Run(context.Background(), args, svc, out, &bytes.Buffer{})
	return out.String(), code
}

func TestPromoteCmd_Default_KeepsProjectRegionContent(t *testing.T) {
	svc := &spyService{}

	runPromoteWithRegions(t, svc)

	if svc.promoteReq == nil {
		t.Fatal("svc.Promote was not called")
	}
	if svc.promoteReq.EmptyProjectRegions {
		t.Error("EmptyProjectRegions = true without the flag; project content must be kept by default")
	}
}

func TestPromoteCmd_EmptyProjectRegionsFlag_EmptiesProjectRegions(t *testing.T) {
	svc := &spyService{}

	_, code := runPromoteWithRegions(t, svc, "--empty-project-regions")

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess (%d)", code, cli.ExitSuccess)
	}
	if svc.promoteReq == nil {
		t.Fatal("svc.Promote was not called")
	}
	if !svc.promoteReq.EmptyProjectRegions {
		t.Error("EmptyProjectRegions = false with --empty-project-regions")
	}
}

func TestPromoteCmd_EmptyProjectRegionsFlag_OtherRequestFieldsUnchanged(t *testing.T) {
	svc := &spyService{}

	runPromoteWithRegions(t, svc, "--empty-project-regions", "--category", "Audit", "--overwrite")

	if svc.promoteReq == nil {
		t.Fatal("svc.Promote was not called")
	}
	want := app.PromoteRequest{
		FilePath:            regionsAgentPath,
		Category:            "Audit",
		Overwrite:           true,
		HarnessID:           "stub-harness",
		EmptyProjectRegions: true,
	}
	if !reflect.DeepEqual(*svc.promoteReq, want) {
		t.Errorf("PromoteRequest = %+v, want %+v", *svc.promoteReq, want)
	}
}

func TestPromoteCmd_HumanOutput_ListsRemovedCustomRegionsAfterStrippedFields(t *testing.T) {
	svc := &spyService{promoteResp: app.PromoteResult{
		SourcePath:      regionsAgentPath,
		DestinationPath: "/dest/agent.md",
		Key:             "agent",
		NumericID:       "9",
		StrippedFields: []app.StrippedField{
			{Key: "customField", Reason: app.StripReasonUnknownField},
		},
		RemovedCustomRegions: []string{"TopNote", "LateNote"},
	}}

	out, _ := runPromoteWithRegions(t, svc)

	stripped := strings.Index(out, "stripped field: customField")
	first := strings.Index(out, "removed custom region: TopNote\n")
	second := strings.Index(out, "removed custom region: LateNote\n")
	if stripped < 0 || first < 0 || second < 0 {
		t.Fatalf("output lacks expected lines:\n%s", out)
	}
	if !(stripped < first && first < second) {
		t.Errorf("removed custom region lines must follow the stripped field lines, in order:\n%s", out)
	}
}

func TestPromoteCmd_DryRunHumanOutput_ListsRemovedCustomRegions(t *testing.T) {
	svc := &spyService{promoteResp: app.PromoteResult{
		SourcePath:           regionsAgentPath,
		DestinationPath:      "/dest/agent.md",
		Key:                  "agent",
		NumericID:            "9",
		DryRun:               true,
		RemovedCustomRegions: []string{"TopNote"},
	}}

	out, _ := runPromoteWithRegions(t, svc, "--dry-run")

	if !strings.Contains(out, "removed custom region: TopNote") {
		t.Errorf("dry-run output lacks the removed custom region line:\n%s", out)
	}
}

func TestPromoteCmd_HumanOutput_NoCustomRegionRemoved_OmitsRegionLines(t *testing.T) {
	svc := &spyService{promoteResp: app.PromoteResult{
		SourcePath: regionsAgentPath, DestinationPath: "/dest/agent.md", Key: "agent", NumericID: "9",
	}}

	out, _ := runPromoteWithRegions(t, svc)

	if strings.Contains(out, "removed custom region") {
		t.Errorf("output mentions removed custom regions although none were removed:\n%s", out)
	}
}

func TestPromoteCmd_JSONOutput_CarriesRemovedCustomRegions(t *testing.T) {
	svc := &spyService{promoteResp: app.PromoteResult{
		SourcePath: regionsAgentPath, DestinationPath: "/dest/agent.md", Key: "agent", NumericID: "9",
		RemovedCustomRegions: []string{"TopNote", "LateNote"},
	}}

	out, _ := runPromoteWithRegions(t, svc, "--output", "json")

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	got, ok := decoded["removedCustomRegions"].([]any)
	if !ok || len(got) != 2 || got[0] != "TopNote" || got[1] != "LateNote" {
		t.Errorf("removedCustomRegions = %v, want [TopNote LateNote]", decoded["removedCustomRegions"])
	}
}

func TestPromoteCmd_JSONOutput_NoRemovedCustomRegions_OmitsField(t *testing.T) {
	svc := &spyService{promoteResp: app.PromoteResult{
		SourcePath: regionsAgentPath, DestinationPath: "/dest/agent.md", Key: "agent", NumericID: "9",
	}}

	out, _ := runPromoteWithRegions(t, svc, "--output", "json")

	if strings.Contains(out, "removedCustomRegions") {
		t.Errorf("JSON output carries removedCustomRegions although none were removed:\n%s", out)
	}
}
