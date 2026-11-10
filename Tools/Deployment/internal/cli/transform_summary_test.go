package cli_test

// transform_summary_test.go covers the transform command as a whole-deployment move:
// the workspace flag, plan confirmation flag, the plan listing shown for a real run, a
// dry run and an unconfirmed plan, the summary content, the "where transform looked"
// message, the cancelled outcome, exit codes and the JSON result contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

const (
	tfWorkspace  = "/ws"
	tfAgentsDir  = "/ws/.claude/agents"
	tfTodoPath   = "/ws/mosaic-deploy-todo.md"
	tfBothMsg    = "agent folder holds both copies of reviewer"
	tfNewerMsg   = "harness injections and hooks come from the target and may be newer"
	tfNoAgentMsg = "no MOSAIC agents were found"

	// tfOrphanAgent is an agent name that appears nowhere in planResult's file listing, so
	// seeing it in the output proves the unresolved-model section is rendered.
	tfOrphanAgent = "orphan-agent"
)

// transformArgs returns the minimal valid transform arguments plus extra flags.
func transformArgs(extra ...string) []string {
	base := []string{"transform", "--harness", "claude-code", "--target-harness", "opencode",
		"--workspace", tfWorkspace}
	return append(base, extra...)
}

// planResult builds a result covering every outcome kind the summary must show.
func planResult() app.TransformHarnessResult {
	return app.TransformHarnessResult{
		SourceHarnessID: "claude-code",
		TargetHarnessID: "opencode",
		Workspace:       tfWorkspace,
		SourceAgentsDir: tfAgentsDir,
		Files: []app.TransformFileOutcome{
			{Kind: domain.ArtifactAgent, Key: "writer", SourcePath: tfAgentsDir + "/writer.md",
				DestinationPath: ".opencode/agents/writer.md", Action: app.PlanWriteCreate,
				Status: app.StatusTransformed},
			{Kind: domain.ArtifactAgent, Key: "notes", SourcePath: tfAgentsDir + "/notes.md",
				DestinationPath: "", Action: app.PlanSkip, Status: app.StatusSkipped,
				Skip: app.SkipNotAgent, Reason: "no version stamp in frontmatter"},
			{Kind: domain.ArtifactAgent, Key: "broken", SourcePath: tfAgentsDir + "/broken.md",
				DestinationPath: ".opencode/agents/broken.md", Action: app.PlanWriteCreate,
				Status: app.StatusFailed, Reason: "frontmatter could not be parsed"},
			{Kind: domain.ArtifactSkill, Key: "lint", SourcePath: "/ws/.claude/skills/lint",
				DestinationPath: ".opencode/skills/lint", Action: app.PlanWriteOverwrite,
				Status: app.StatusTransformed},
			{Kind: domain.ArtifactHook, Key: "logger", DestinationPath: ".opencode/hooks/logger",
				Action: app.PlanSkip, Status: app.StatusSkipped, Skip: app.SkipNoTargetVariant,
				Reason: "bundle has no opencode variant"},
		},
		Transformed: 2, Skipped: 2, Failed: 1,
	}
}

// listingLines returns the output lines that mention a planned file or hook folder.
func listingLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "writer.md") || strings.Contains(l, "notes.md") ||
			strings.Contains(l, "broken.md") || strings.Contains(l, "skills/lint") ||
			strings.Contains(l, "hooks/logger") {
			lines = append(lines, strings.TrimSpace(l))
		}
	}
	return lines
}

func runTransform(svc *spyService, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), args, svc, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestTransformCmd_PathFlag_IsUnknown(t *testing.T) {
	svc := &spyService{}
	code, _, _ := runTransform(svc, "transform", "--harness", "claude-code",
		"--target-harness", "opencode", "--path", "/agents/a.md")
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want ExitUsage; --path no longer exists", code)
	}
	if svc.transformReq != nil {
		t.Error("TransformHarness was called despite the removed --path flag")
	}
}

func TestTransformCmd_YesFlag_SetsAutoConfirmPlan(t *testing.T) {
	svc := &spyService{transformResp: planResult()}
	runTransform(svc, transformArgs("--yes")...)
	if svc.transformReq == nil || !svc.transformReq.AutoConfirmPlan {
		t.Error("--yes must set AutoConfirmPlan")
	}
	svc2 := &spyService{transformResp: planResult()}
	runTransform(svc2, transformArgs()...)
	if svc2.transformReq == nil || svc2.transformReq.AutoConfirmPlan {
		t.Error("AutoConfirmPlan must be false without --yes")
	}
}

func TestTransformCmd_HelpText_DescribesWholeDeploymentMove(t *testing.T) {
	svc := &spyService{}
	_, out, errOut := runTransform(svc, "transform", "--help")
	help := strings.ToLower(out + errOut)

	for _, want := range []string{"--workspace", "--yes", "--dry-run"} {
		if !strings.Contains(help, want) {
			t.Errorf("transform help does not mention %s:\n%s", want, help)
		}
	}
	if strings.Contains(help, "--path") {
		t.Errorf("transform help still mentions --path:\n%s", help)
	}
	for _, old := range []string{"agent file or", "non-recursive", "one or more deployed agent files"} {
		if strings.Contains(help, old) {
			t.Errorf("transform help still describes single-file input (%q):\n%s", old, help)
		}
	}
	if !strings.Contains(help, "workspace root") {
		t.Errorf("transform help does not describe the workspace root as the input:\n%s", help)
	}
	if !strings.Contains(help, "deployment") {
		t.Errorf("transform help does not describe moving a deployment:\n%s", help)
	}
}

func TestTransformCmd_WorkspaceFlag_MissingOrEmpty_IsUsageError(t *testing.T) {
	cases := map[string][]string{
		"missing": {"transform", "--harness", "claude-code", "--target-harness", "opencode"},
		"empty": {"transform", "--harness", "claude-code", "--target-harness", "opencode",
			"--workspace", ""},
	}
	for name, args := range cases {
		svc := &spyService{transformResp: planResult()}
		code, _, _ := runTransform(svc, args...)
		if code != cli.ExitUsage {
			t.Errorf("%s: exit code = %d, want ExitUsage", name, code)
		}
		if svc.transformReq != nil {
			t.Errorf("%s: TransformHarness was called without a workspace", name)
		}
	}
}

func TestTransformCmd_WorkspaceFlag_PassesWorkspaceToService(t *testing.T) {
	svc := &spyService{transformResp: planResult()}
	runTransform(svc, transformArgs()...)
	if svc.transformReq == nil || svc.transformReq.Workspace != tfWorkspace {
		t.Errorf("request = %+v, want Workspace %q", svc.transformReq, tfWorkspace)
	}
}

func TestTransformCmd_DryRun_ShowsDryRunIndicatorAndNoCancelText(t *testing.T) {
	res := planResult()
	res.DryRun = true
	svc := &spyService{transformResp: res}

	_, out, _ := runTransform(svc, transformArgs("--dry-run")...)

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "dry run") && !strings.Contains(lower, "dry-run") {
		t.Errorf("dry-run output carries no dry-run indicator:\n%s", out)
	}
	if strings.Contains(lower, "cancel") {
		t.Errorf("a dry run is shown as cancelled:\n%s", out)
	}
}

func TestTransformCmd_PlanListing_ShowsEveryOutcomeWithReasons(t *testing.T) {
	svc := &spyService{transformResp: planResult()}
	code, out, _ := runTransform(svc, transformArgs("--dry-run")...)
	if code == cli.ExitUsage {
		t.Fatalf("unexpected usage error")
	}
	lines := strings.Join(listingLines(out), "\n")
	for _, want := range []string{
		"writer.md", ".opencode/agents/writer.md", "create",
		"notes.md", "no version stamp in frontmatter",
		"broken.md", "frontmatter could not be parsed",
		"skills/lint", "overwrite",
		"hooks/logger", "bundle has no opencode variant",
	} {
		if !strings.Contains(lines, want) {
			t.Errorf("plan listing lacks %q; listing:\n%s\nfull output:\n%s", want, lines, out)
		}
	}
}

func TestTransformCmd_PlanListing_SameForRealRunDryRunAndUnconfirmedPlan(t *testing.T) {
	realRun := &spyService{transformResp: planResult()}
	_, realOut, _ := runTransform(realRun, transformArgs("--yes")...)

	dryResult := planResult()
	dryResult.DryRun = true
	dry := &spyService{transformResp: dryResult}
	_, dryOut, _ := runTransform(dry, transformArgs("--dry-run")...)

	declinedResult := planResult()
	declinedResult.Cancelled = true
	declined := &spyService{transformResp: declinedResult, transformErr: app.ErrPlanNotConfirmed}
	_, declinedOut, declinedErr := runTransform(declined, transformArgs()...)

	want := listingLines(realOut)
	if len(want) < 5 {
		t.Fatalf("real run listing has %d lines, want one per outcome (5):\n%s", len(want), realOut)
	}
	for name, got := range map[string][]string{
		"dry run":     listingLines(dryOut),
		"unconfirmed": listingLines(declinedOut),
	} {
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s listing differs from the real run listing:\n got:\n%s\nwant:\n%s",
				name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	if strings.Contains(declinedErr, "writer.md") {
		t.Errorf("the plan listing belongs on stdout, found on stderr:\n%s", declinedErr)
	}
}

func TestTransformCmd_UnconfirmedPlan_SaysNothingWrittenAndYesConfirms(t *testing.T) {
	res := planResult()
	res.Cancelled = true
	svc := &spyService{transformResp: res, transformErr: app.ErrPlanNotConfirmed}

	code, out, errOut := runTransform(svc, transformArgs()...)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want ExitFailure", code)
	}
	all := strings.ToLower(out + errOut)
	if !regexp.MustCompile(`not (been )?confirmed|unconfirmed|declined`).MatchString(all) {
		t.Errorf("output does not say the plan was not confirmed:\n%s", all)
	}
	if !strings.Contains(all, "nothing was written") {
		t.Errorf("output does not say nothing was written:\n%s", all)
	}
	if !strings.Contains(all, "--yes") {
		t.Errorf("output does not tell the user --yes confirms:\n%s", all)
	}
}

func TestTransformCmd_Summary_ShowsCountsModelsToolsNotesAndTodo(t *testing.T) {
	res := planResult()
	res.UnresolvedModels = []string{tfOrphanAgent}
	res.Files[0].UnmappedTools = []string{"WebSearch", "Notebook"}
	res.TodoPath = tfTodoPath
	res.Notes = []app.TransformNote{
		{Kind: app.NoteBothForms, Message: tfBothMsg,
			Paths: []string{".opencode/agents/reviewer.md", ".opencode/agents/reviewer.agent.md"}},
		{Kind: app.NoteNewerTargetContent, Message: tfNewerMsg},
	}
	svc := &spyService{transformResp: res}

	_, out, _ := runTransform(svc, transformArgs("--yes")...)

	counts := regexp.MustCompile(`(?i)transformed\W+2\W+skipped\W+2\W+failed\W+1`)
	if !counts.MatchString(out) {
		t.Errorf("output lacks the transformed/skipped/failed counts:\n%s", out)
	}
	for _, want := range []string{
		"WebSearch", "Notebook", // unmapped tools
		tfBothMsg, ".opencode/agents/reviewer.md", ".opencode/agents/reviewer.agent.md",
		tfNewerMsg, tfTodoPath,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if !lineWithAll(out, tfOrphanAgent, "model") {
		t.Errorf("output does not list %q on a line labelled as a model problem:\n%s", tfOrphanAgent, out)
	}
}

// lineWithAll reports whether one line of out contains every word (case-insensitive).
func lineWithAll(out string, words ...string) bool {
	for _, l := range strings.Split(strings.ToLower(out), "\n") {
		all := true
		for _, w := range words {
			all = all && strings.Contains(l, strings.ToLower(w))
		}
		if all {
			return true
		}
	}
	return false
}

func TestTransformCmd_Summary_NoUnresolvedModels_NoModelLabelOnAgentLine(t *testing.T) {
	res := planResult()
	res.Files = append(res.Files, app.TransformFileOutcome{Kind: domain.ArtifactAgent,
		Key: tfOrphanAgent, SourcePath: tfAgentsDir + "/" + tfOrphanAgent + ".md",
		DestinationPath: ".opencode/agents/" + tfOrphanAgent + ".md", Action: app.PlanWriteCreate,
		Status: app.StatusTransformed})
	res.Transformed++
	svc := &spyService{transformResp: res}

	_, out, _ := runTransform(svc, transformArgs("--yes")...)

	if !strings.Contains(out, tfOrphanAgent) {
		t.Fatalf("fixture agent %q not shown at all:\n%s", tfOrphanAgent, out)
	}
	if lineWithAll(out, tfOrphanAgent, "model") {
		t.Errorf("agent shown as a model problem though UnresolvedModels is empty:\n%s", out)
	}
}

func TestTransformCmd_NoAgentsFound_NamesFolderTransformLookedIn(t *testing.T) {
	res := app.TransformHarnessResult{
		SourceHarnessID: "claude-code", TargetHarnessID: "opencode",
		Workspace: tfWorkspace, SourceAgentsDir: tfAgentsDir,
		Notes: []app.TransformNote{{Kind: app.NoteNoAgentsFound, Message: tfNoAgentMsg,
			Paths: []string{tfAgentsDir}}},
	}
	svc := &spyService{transformResp: res}

	_, out, _ := runTransform(svc, transformArgs()...)

	if !strings.Contains(out, tfAgentsDir) {
		t.Errorf("output does not name the folder transform looked in (%s):\n%s", tfAgentsDir, out)
	}
	if !strings.Contains(out, tfNoAgentMsg) {
		t.Errorf("output does not show the no-agents note:\n%s", out)
	}
}

func TestTransformCmd_CancelledRun_SaysCancelledAndNothingWritten(t *testing.T) {
	for _, svcErr := range []error{app.ErrModelSelectionCancelled, app.ErrSelectionCancelled} {
		res := app.TransformHarnessResult{SourceHarnessID: "claude-code",
			TargetHarnessID: "opencode", Workspace: tfWorkspace, Cancelled: true}
		svc := &spyService{transformResp: res, transformErr: svcErr}

		code, out, errOut := runTransform(svc, transformArgs()...)

		if code != cli.ExitFailure {
			t.Errorf("%v: exit code = %d, want ExitFailure", svcErr, code)
		}
		all := strings.ToLower(out + errOut)
		if !strings.Contains(all, "cancelled") {
			t.Errorf("%v: output does not say the run was cancelled:\n%s", svcErr, all)
		}
		if !strings.Contains(all, "nothing was written") {
			t.Errorf("%v: output does not say nothing was written:\n%s", svcErr, all)
		}
	}
}

func TestTransformCmd_ExitCodes(t *testing.T) {
	failed := planResult()
	unresolved := app.TransformHarnessResult{Transformed: 1, UnresolvedModels: []string{"writer"},
		Files: []app.TransformFileOutcome{{Kind: domain.ArtifactAgent, Key: "writer",
			Action: app.PlanWriteCreate, Status: app.StatusTransformed}}}
	unmapped := app.TransformHarnessResult{Transformed: 1,
		Files: []app.TransformFileOutcome{{Kind: domain.ArtifactAgent, Key: "writer",
			Action: app.PlanWriteCreate, Status: app.StatusTransformed, UnmappedTools: []string{"X"}}}}
	hookSkipped := func(k app.TransformSkipKind) app.TransformHarnessResult {
		return app.TransformHarnessResult{Skipped: 1,
			Files: []app.TransformFileOutcome{{Kind: domain.ArtifactHook, Key: "h",
				Action: app.PlanSkip, Status: app.StatusSkipped, Skip: k, Reason: "r"}}}
	}
	clean := app.TransformHarnessResult{Transformed: 1,
		Files: []app.TransformFileOutcome{{Kind: domain.ArtifactAgent, Key: "writer",
			Action: app.PlanWriteCreate, Status: app.StatusTransformed}}}

	cases := []struct {
		name string
		res  app.TransformHarnessResult
		want int
	}{
		{"failed file", failed, cli.ExitFailure},
		{"unresolved models", unresolved, cli.ExitWithSkips},
		{"unmapped tools", unmapped, cli.ExitWithSkips},
		{"hook unsupported", hookSkipped(app.SkipUnsupported), cli.ExitWithSkips},
		{"hook no target variant", hookSkipped(app.SkipNoTargetVariant), cli.ExitWithSkips},
		{"hook shared folder", hookSkipped(app.SkipSharedHooksFolder), cli.ExitWithSkips},
		{"hook already present", hookSkipped(app.SkipDestinationExists), cli.ExitSuccess},
		{"clean", clean, cli.ExitSuccess},
	}
	for _, tc := range cases {
		code, _, _ := runTransform(&spyService{transformResp: tc.res}, transformArgs("--yes")...)
		if code != tc.want {
			t.Errorf("%s: exit code = %d, want %d", tc.name, code, tc.want)
		}
	}
}

func TestTransformCmd_JSON_EncodesResultContract(t *testing.T) {
	res := planResult()
	res.SourceAgentsDir = tfAgentsDir
	res.UnresolvedModels = []string{"writer"}
	res.TodoPath = tfTodoPath
	res.Notes = []app.TransformNote{{Kind: app.NoteNoAgentsFound, Message: "m", Paths: []string{tfAgentsDir}}}
	svc := &spyService{transformResp: res}

	_, out, _ := runTransform(svc, transformArgs("--output", "json", "--yes")...)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("output is not one JSON document: %v\n%s", err, out)
	}
	for _, key := range []string{"sourceAgentsDir", "cancelled", "unresolvedModels", "files", "notes",
		"todoPath", "transformed", "skipped", "failed"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON result lacks %q:\n%s", key, out)
		}
	}
	for _, removed := range []string{"inputPath", "inputIsDirectory", "skippedMismatch", "skippedNotAgent"} {
		if _, ok := raw[removed]; ok {
			t.Errorf("JSON result still has removed field %q", removed)
		}
	}
	var got app.TransformHarnessResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Files[1].Skip != app.SkipNotAgent || got.Files[1].Reason == "" || got.Files[0].Action != app.PlanWriteCreate {
		t.Errorf("file outcomes lost skip/reason/action: %+v", got.Files)
	}
}

func TestTransformCmd_JSON_ServiceErrorStillEncodesResultAndErrorGoesToStderr(t *testing.T) {
	res := app.TransformHarnessResult{SourceHarnessID: "claude-code", TargetHarnessID: "opencode",
		Workspace: tfWorkspace, Cancelled: true, Files: []app.TransformFileOutcome{}}
	svc := &spyService{transformResp: res, transformErr: app.ErrModelSelectionCancelled}

	code, out, errOut := runTransform(svc, transformArgs("--output", "json")...)

	if code != cli.ExitFailure {
		t.Errorf("exit code = %d, want ExitFailure", code)
	}
	var got app.TransformHarnessResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not a JSON result on a service error: %v\n%s", err, out)
	}
	if !got.Cancelled {
		t.Error("JSON result must carry cancelled: true")
	}
	if !strings.Contains(errOut, app.ErrModelSelectionCancelled.Error()) {
		t.Errorf("stderr lacks the service error text:\n%s", errOut)
	}
}

func TestTransformCmd_OtherServiceError_ReportsErrorAndExitFailure(t *testing.T) {
	svc := &spyService{transformErr: errors.New("workspace unreadable")}
	code, _, errOut := runTransform(svc, transformArgs()...)
	if code != cli.ExitFailure || !strings.Contains(errOut, "workspace unreadable") {
		t.Errorf("code=%d stderr=%q; want ExitFailure with the error text", code, errOut)
	}
}
