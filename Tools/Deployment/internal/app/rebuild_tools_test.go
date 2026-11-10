package app_test

// rebuild_tools_test.go covers the wiring of the tool conversion into the rebuilt file: target-native
// names kept verbatim and written after the mapped tools, custom servers in the target's custom
// form, dropped tools, unmapped generics with their gaps, and the skill-access rule per role.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// splitToolScalar returns the comma-separated tool names of a scalar field in file order.
func splitToolScalar(t *testing.T, out, key string) []string {
	t.Helper()
	v, ok := rbFrontmatter(t, out).Get(key)
	if !ok || v.Kind != domain.KindScalar {
		t.Fatalf("%q must be a comma scalar, got present=%v kind=%v:\n%s", key, ok, v.Kind, out)
	}
	var names []string
	for _, part := range strings.Split(v.Scalar, ",") {
		names = append(names, strings.TrimSpace(part))
	}
	return names
}

func countName(names []string, want string) int {
	n := 0
	for _, s := range names {
		if s == want {
			n++
		}
	}
	return n
}

func TestRebuild_KeptCustomAndUnmappedToolsReachTargetFile(t *testing.T) {
	p := rbPairs[1] // Claude Code to OpenCode
	addTools := withSourceEdit(replaceText("tools: Read, Grep, Bash", "tools: Read, Grep, Bash, patch, mcp__srv__*, WebSearch"))

	res, out := rbRebuild(t, rbInput(t, p, false, addTools))

	names := rbToolNames(t, out, "permission")
	for _, want := range []string{"patch", "srv"} {
		if countName(names, want) != 1 {
			t.Errorf("%q must be allowed exactly once under permission, got %v", want, names)
		}
		if countName(res.Tools, want) != 1 {
			t.Errorf("RebuildResult.Tools must list %q exactly once, got %v", want, res.Tools)
		}
	}
	if len(res.UnmappedTools) != 1 || res.UnmappedTools[0] != "web_search" {
		t.Errorf("UnmappedTools: want [web_search], got %v", res.UnmappedTools)
	}
	if countGaps(res.Gaps, domain.GapUnmappedTool) != 1 {
		t.Errorf("exactly one unmapped-tool gap expected, got %+v", res.Gaps)
	}
}

func TestRebuild_KeptToolFollowsMappedToolsAndCustomServerUsesTargetForm(t *testing.T) {
	p := rbPairs[0] // OpenCode to Claude Code
	addTools := withSourceEdit(replaceText("  bash: allow\n", "  bash: allow\n  lsp: allow\n  srv: allow\n  WebSearch: allow\n"))

	res, out := rbRebuild(t, rbInput(t, p, false, addTools))

	names := splitToolScalar(t, out, "tools")
	if countName(names, "mcp__srv__*") != 1 {
		t.Errorf("custom server must appear once in the target's custom form, got %v", names)
	}
	if countName(names, "WebSearch") != 1 || names[len(names)-1] != "WebSearch" {
		t.Errorf("kept target-native WebSearch must be written once, after the mapped tools: %v", names)
	}
	if countName(names, "lsp") != 0 {
		t.Errorf("a source tool without a target equivalent must be dropped, got %v", names)
	}
	dropped := false
	for _, d := range res.Dropped {
		if d.Name == "lsp" {
			dropped = true
		}
	}
	if !dropped {
		t.Errorf("Dropped must list lsp, got %+v", res.Dropped)
	}
}

func TestRebuild_SkillAccessIsAddedForSubagentButNotOrchestrator(t *testing.T) {
	p := rbPairs[1] // Claude Code to OpenCode

	_, outSub := rbRebuild(t, rbInput(t, p, true))
	_, outOrch := rbRebuild(t, rbInput(t, p, true, withFileName("orchestrator.md")))

	if !containsString(rbToolNames(t, outSub, "permission"), "skill") {
		t.Errorf("a subagent must get skill access:\n%s", outSub)
	}
	if containsString(rbToolNames(t, outOrch, "permission"), "skill") {
		t.Errorf("an orchestrator must not get skill access:\n%s", outOrch)
	}
}
