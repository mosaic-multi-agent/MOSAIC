package app_test

// rebuild_answers_test.go covers the caller-supplied tool answers of the deployed-agent rebuild
// (custom-tool answers and skipped generics) and the main orchestrator whose tools are the source
// placeholder expansion plus extras, in both the list case and the placeholder case.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// claudeToOpencode is the pair whose target has no equivalent for web_search.
var claudeToOpencode = rbPairs[1]

// claudeToClaude is the pair whose tools field is a comma scalar on both sides, which
// shows the order of the main tools field directly.
var claudeToClaude = rbPair{"claude-to-claude", hClaude, hClaude}

// withWebSearch adds the unmappable WebSearch tool to the Claude Code fixture.
var withWebSearch = withSourceEdit(replaceText("tools: Read, Grep", "tools: Read, Grep, WebSearch"))

func TestRebuild_AnsweredUnmappedTool_BecomesCustomServerAndReportsNoGap(t *testing.T) {
	answer := withCustomToolAnswers(map[string]string{"web_search": "srv"})

	res, out := rbRebuild(t, rbInput(t, claudeToOpencode, true, withWebSearch, answer))

	if names := rbToolNames(t, out, "permission"); countName(names, "srv") != 1 {
		t.Errorf("the answered server must be allowed once in the target's custom form, got %v", names)
	}
	if len(res.UnmappedTools) != 0 {
		t.Errorf("an answered tool must not stay unmapped, got %v", res.UnmappedTools)
	}
	if n := countGaps(res.Gaps, domain.GapUnmappedTool); n != 0 {
		t.Errorf("an answered tool must have no unmapped-tool gap, got %d in %+v", n, res.Gaps)
	}
}

func TestRebuild_SkippedUnmappedTool_StaysUnmappedWithOneGapAndWritesNothing(t *testing.T) {
	skip := withSkippedTools(map[string]bool{"web_search": true})

	res, out := rbRebuild(t, rbInput(t, claudeToOpencode, true, withWebSearch, skip))

	if len(res.UnmappedTools) != 1 || res.UnmappedTools[0] != "web_search" {
		t.Errorf("UnmappedTools: want [web_search], got %v", res.UnmappedTools)
	}
	if n := countGaps(res.Gaps, domain.GapUnmappedTool); n != 1 {
		t.Errorf("exactly one unmapped-tool gap expected, got %d in %+v", n, res.Gaps)
	}
	if strings.Contains(out, "web_search") || strings.Contains(strings.ToLower(out), "websearch") {
		t.Errorf("a skipped tool must not be written:\n%s", out)
	}
}

// orchestratorWithExtras makes the Claude Code stamp-less fixture a main orchestrator whose
// tools are the source's placeholder expansion plus a custom server and a mapped extra.
func orchestratorWithExtras(t *testing.T, extras ...string) []rbOpt {
	t.Helper()
	expansion := rbModule(t, hClaude).Descriptor().Tools.PlaceholderExpansion
	line := "tools: " + strings.Join(append(append([]string(nil), expansion...), extras...), ", ")
	return []rbOpt{
		withFileName("orchestrator.md"),
		withSourceEdit(replaceText("tools: Read, Grep", line)),
	}
}

func TestRebuild_MainOrchestratorPlaceholder_ExpansionThenExtrasEachOnce(t *testing.T) {
	expansion := rbModule(t, hClaude).Descriptor().Tools.PlaceholderExpansion
	opts := orchestratorWithExtras(t, "mcp__srv__*", "WebFetch")

	res, out := rbRebuild(t, rbInput(t, claudeToClaude, true, opts...))

	names := splitToolScalar(t, out, "tools")
	want := append(append([]string(nil), expansion...), "mcp__srv__*", "WebFetch")
	if len(names) != len(want) || !equalStrings(names[:len(expansion)], expansion) {
		t.Errorf("main tools: want the expansion first, then the two extras\nwant %v\ngot  %v", want, names)
	}
	for _, name := range want {
		if countName(names, name) != 1 {
			t.Errorf("%q must appear exactly once in the main tools field, got %v", name, names)
		}
	}
	if len(res.UnmappedTools) != 0 {
		t.Errorf("a mapped extra and a server are not unmapped, got %v", res.UnmappedTools)
	}
}

func TestRebuild_MainOrchestratorPlaceholder_UnmappedExtraNeedsAnswerOrGetsGap(t *testing.T) {
	opts := orchestratorWithExtras(t, "WebSearch")

	resOpen, outOpen := rbRebuild(t, rbInput(t, claudeToOpencode, true, opts...))
	resSkip, _ := rbRebuild(t, rbInput(t, claudeToOpencode, true,
		append(append([]rbOpt(nil), opts...), withSkippedTools(map[string]bool{"web_search": true}))...))
	resAnswered, outAnswered := rbRebuild(t, rbInput(t, claudeToOpencode, true,
		append(append([]rbOpt(nil), opts...), withCustomToolAnswers(map[string]string{"web_search": "srv"}))...))

	for name, res := range map[string]app.RebuildResult{"unanswered": resOpen, "skipped": resSkip} {
		if len(res.UnmappedTools) != 1 || res.UnmappedTools[0] != "web_search" {
			t.Errorf("%s: UnmappedTools want [web_search], got %v", name, res.UnmappedTools)
		}
		if n := countGaps(res.Gaps, domain.GapUnmappedTool); n != 1 {
			t.Errorf("%s: exactly one unmapped-tool gap expected, got %d in %+v", name, n, res.Gaps)
		}
	}
	if containsString(rbToolNames(t, outOpen, "permission"), "srv") {
		t.Errorf("an unanswered extra must write nothing:\n%s", outOpen)
	}
	if countName(rbToolNames(t, outAnswered, "permission"), "srv") != 1 {
		t.Errorf("the answered extra must be allowed once in the custom form:\n%s", outAnswered)
	}
	if len(resAnswered.UnmappedTools) != 0 || countGaps(resAnswered.Gaps, domain.GapUnmappedTool) != 0 {
		t.Errorf("an answered extra has no unmapped entry or gap, got %v %+v", resAnswered.UnmappedTools, resAnswered.Gaps)
	}
}

func TestRebuild_OrchestratorHarnessRegionCarriesTargetOrchestratorVersion(t *testing.T) {
	in := rbInput(t, claudeToOpencode, false, withFileName("orchestrator.md"), withoutRoleStamp)
	version := in.TargetModule.Descriptor().OrchestratorInjectionsVersion
	if version == "" {
		t.Skip("target declares no orchestrator injections version")
	}

	_, out := rbRebuild(t, in)

	assertOnce(t, out, `<HarnessConstraints type="managed" version="`+version+`">`)
}
