package app_test

// rebuild_target_test.go covers what the deployed-agent rebuild takes from the target harness
// instead of the source file: tool names and the shape of the tools field, harness version
// stamps, role-dependent frontmatter fields, and harness-filled regions with their version
// attribute. It also covers the source-only frontmatter keys that must not leak.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// wantTools is the target tool set a fully stamped fixture converts to, per pair.
var wantTools = map[string][]string{
	"opencode-to-claude": {"Bash", "Grep", "Read"},
	// Skill access is explicit on OpenCode; the search tool also brings the target's by-convention
	// companion (list / search/listDirectory), as a fresh deploy writes it.
	"claude-to-opencode": {"bash", "grep", "list", "read", "skill"},
	"claude-to-vscode":   {"execute/runInTerminal", "read/readFile", "search/listDirectory", "search/textSearch"},
	"ghcpcli-to-claude":  {"AskUserQuestion", "Bash", "Read"},
}

// targetToolsKey is the frontmatter key that holds the tools of each target harness.
var targetToolsKey = map[string]string{hClaude: "tools", hOpen: "permission", hVSCode: "tools", hCLI: "tools"}

func TestRebuild_ToolNamesComeFromTarget(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, p, false))

			got := rbToolNames(t, out, targetToolsKey[p.tgt])
			if !equalStrings(got, sortedCopy(wantTools[p.name]...)) {
				t.Errorf("tools under %q: want %v, got %v", targetToolsKey[p.tgt], wantTools[p.name], got)
			}
			if !equalStrings(sortedCopy(res.Tools...), sortedCopy(wantTools[p.name]...)) {
				t.Errorf("RebuildResult.Tools: want %v, got %v", wantTools[p.name], res.Tools)
			}
		})
	}
}

func TestRebuild_ToolsFieldShapeFollowsTarget(t *testing.T) {
	kinds := map[string]domain.ValueKind{
		"opencode-to-claude": domain.KindScalar,  // Claude Code: comma-separated scalar
		"claude-to-opencode": domain.KindMapping, // OpenCode: permission mapping
		"claude-to-vscode":   domain.KindList,    // VS Code GHCP: list
		"ghcpcli-to-claude":  domain.KindScalar,
	}
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false))

			v, ok := rbFrontmatter(t, out).Get(targetToolsKey[p.tgt])
			if !ok {
				t.Fatalf("target tools key %q missing:\n%s", targetToolsKey[p.tgt], out)
			}
			if v.Kind != kinds[p.name] {
				t.Errorf("shape of %q: want %v, got %v", targetToolsKey[p.tgt], kinds[p.name], v.Kind)
			}
		})
	}
}

func TestRebuild_SourceToolsKeyDoesNotLeakIntoPermissionTarget(t *testing.T) {
	p := rbPairs[1] // Claude Code to OpenCode: the target tools key differs from the source's
	res, out := rbRebuild(t, rbInput(t, p, false))

	if rbHasKey(t, out, "tools") {
		t.Errorf("the source tools key must not appear in the OpenCode output:\n%s", out)
	}
	if !containsString(res.DroppedFields, "tools") {
		t.Errorf("DroppedFields must list the source tools key, got %v", res.DroppedFields)
	}
}

func TestRebuild_SourceOnlyKeysDoNotLeakToClaude(t *testing.T) {
	cases := []struct {
		pair rbPair
		keys []string
	}{
		{rbPairs[0], []string{"permission", "mode"}}, // OpenCode source
		{rbPairs[3], []string{"user-invocable"}},     // GHCP CLI source
	}
	for _, c := range cases {
		t.Run(c.pair.name, func(t *testing.T) {
			res, out := rbRebuild(t, rbInput(t, c.pair, false))

			for _, key := range c.keys {
				if rbHasKey(t, out, key) {
					t.Errorf("source-only key %q leaked into the Claude Code output:\n%s", key, out)
				}
				if !containsString(res.DroppedFields, key) {
					t.Errorf("DroppedFields must list %q, got %v", key, res.DroppedFields)
				}
			}
		})
	}
}

func TestRebuild_HarnessVersionStampsComeFromTarget(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			in := rbInput(t, p, false)
			desc := in.TargetModule.Descriptor()

			res, out := rbRebuild(t, in)

			if got, _ := rbScalar(t, out, "mosaic_harness_version"); got != desc.TransformVersion {
				t.Errorf("mosaic_harness_version: want the target's %q, got %q", desc.TransformVersion, got)
			}
			if res.Stamps.HarnessVersion != desc.TransformVersion {
				t.Errorf("Stamps.HarnessVersion: want %q, got %q", desc.TransformVersion, res.Stamps.HarnessVersion)
			}
			if res.Stamps.InjectionsVersion != desc.InjectionsVersion {
				t.Errorf("Stamps.InjectionsVersion: want %q, got %q", desc.InjectionsVersion, res.Stamps.InjectionsVersion)
			}
		})
	}
}

func TestRebuild_SourceHarnessRegionContentIsReplacedByTargetInjection(t *testing.T) {
	for _, p := range rbPairs {
		t.Run(p.name, func(t *testing.T) {
			_, out := rbRebuild(t, rbInput(t, p, false))

			if strings.Contains(out, "SOURCE-HARNESS-CONSTRAINT-TEXT") {
				t.Errorf("harness-filled region must not keep the source harness's content:\n%s", out)
			}
			if strings.Contains(out, `<HarnessConstraints type="managed" version="0.5.0">`) {
				t.Errorf("harness region must not keep the source's version attribute:\n%s", out)
			}
		})
	}
}

func TestRebuild_HarnessRegionCarriesTargetContentAndVersion(t *testing.T) {
	cases := []struct {
		pair rbPair
		text string
	}{
		{rbPairs[1], "Working Directory vs Workspace Root"}, // OpenCode injection
		{rbPairs[2], "never assume the file is complete"},   // VS Code GHCP injection
	}
	for _, c := range cases {
		t.Run(c.pair.name, func(t *testing.T) {
			in := rbInput(t, c.pair, false)
			version := in.TargetModule.Descriptor().InjectionsVersion

			_, out := rbRebuild(t, in)

			if !strings.Contains(out, c.text) {
				t.Errorf("target harness injection %q missing from output:\n%s", c.text, out)
			}
			if !strings.Contains(out, `<HarnessConstraints type="managed" version="`+version+`">`) {
				t.Errorf("harness region must carry the target injections version %q:\n%s", version, out)
			}
		})
	}
}

func TestRebuild_RoleDependentFieldsComeFromTarget(t *testing.T) {
	p := rbPairs[2] // Claude Code to VS Code GHCP

	_, out := rbRebuild(t, rbInput(t, p, false))

	if got, ok := rbScalar(t, out, "user-invocable"); !ok || got != "false" {
		t.Errorf("user-invocable for a subagent: want false, got %q (present=%v)", got, ok)
	}
	if got, ok := rbScalar(t, out, "disable-model-invocation"); !ok || got != "false" {
		t.Errorf("disable-model-invocation: want the target's false, got %q (present=%v)", got, ok)
	}
}

func TestRebuild_ClaudeCodeTargetHarnessRegionCarriesTargetVersion(t *testing.T) {
	for _, p := range []rbPair{rbPairs[0], rbPairs[3]} {
		t.Run(p.name, func(t *testing.T) {
			in := rbInput(t, p, false)
			version := in.TargetModule.Descriptor().InjectionsVersion

			_, out := rbRebuild(t, in)

			assertOnce(t, out, `<HarnessConstraints type="managed" version="`+version+`">`)
		})
	}
}
