package app

// transform_plan_discovery_test.go specifies agent discovery of transform planning: which
// files of the source harness agents folder are MOSAIC agents, one entry per agent key,
// the reason a file is skipped, and what an empty result reports.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

var tpAgentKinds = []string{"subagent", "orchestrator", "orchestrator-script", "utility", "standalone", "harness-only"}

func TestPlanTransform_FindsEveryAgentKindOfEachSourceHarness(t *testing.T) {
	cases := []struct{ name, source, target string }{
		{"claude-code", tpClaude, tpOpen},
		{"opencode", tpOpen, tpClaude},
		{"ghcp-cli", tpCLI, tpClaude},
		{"vscode-ghcp", tpVSCode, tpClaude},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.source), tpHarness(t, c.target)
			tpPlaceAllAgents(t, ws, src)
			ext := src.desc.Extensions[domain.ArtifactAgent]

			p := tpPlan(t, ws, src, tgt, false)

			for _, kind := range tpAgentKinds {
				a := tpAgent(t, p, kind+ext)
				if a.Skip == SkipNotAgent {
					t.Errorf("%s%s was skipped as not-agent: %s", kind, ext, a.Reason)
				}
				if a.Key != kind {
					t.Errorf("%s%s Key = %q, want %q", kind, ext, a.Key, kind)
				}
				if a.Action != PlanWriteCreate {
					t.Errorf("%s%s Action = %q, want %q", kind, ext, a.Action, PlanWriteCreate)
				}
			}
			if len(p.Agents) != len(tpAgentKinds) {
				t.Errorf("planned %d agents %v, want %d", len(p.Agents), tpAgentNames(p), len(tpAgentKinds))
			}
			if got := tpNotes(p, NoteNoAgentsFound); len(got) != 0 {
				t.Errorf("unexpected no-agents-found note: %+v", got)
			}
		})
	}
}

func TestPlanTransform_AgentsAreListedInFileNameOrderWithAbsoluteSourcePaths(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAllAgents(t, ws, src)

	p := tpPlan(t, ws, src, tgt, false)

	names := tpAgentNames(p)
	if len(names) != len(tpAgentKinds) {
		t.Fatalf("planned agents = %v, want %d", names, len(tpAgentKinds))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("agents not in file-name order: %v", names)
		}
	}
	for _, a := range p.Agents {
		if !filepath.IsAbs(a.SourcePath) {
			t.Errorf("SourcePath %q is not absolute", a.SourcePath)
		}
	}
	if want := filepath.Join(ws, ".claude", "agents"); filepath.Clean(p.SourceAgentsDir) != want {
		t.Errorf("SourceAgentsDir = %q, want %q", p.SourceAgentsDir, want)
	}
}

func TestPlanTransform_OneEntryPerKeyWhenBothExtensionsExist(t *testing.T) {
	cases := []struct {
		name, source, target, kept, dropped string
	}{
		{"plain-md source keeps x.md", tpClaude, tpOpen, "x.md", "x.agent.md"},
		{"agent-md source keeps x.agent.md", tpCLI, tpClaude, "x.agent.md", "x.md"},
		{"vscode-ghcp source keeps x.agent.md", tpVSCode, tpClaude, "x.agent.md", "x.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.source), tpHarness(t, c.target)
			dir := src.desc.Paths.Agents.Project
			tpPlaceAgent(t, ws, dir, c.kept, "subagent.md")
			tpPlaceAgent(t, ws, dir, c.dropped, "subagent.md")

			p := tpPlan(t, ws, src, tgt, false)

			if got := tpAgent(t, p, c.kept); got.Action == PlanSkip {
				t.Errorf("%s was skipped (%s): %s", c.kept, got.Skip, got.Reason)
			}
			dup := tpAgent(t, p, c.dropped)
			if dup.Action != PlanSkip || dup.Skip != SkipDuplicateKey {
				t.Errorf("%s = action %q skip %q, want skip %q", c.dropped, dup.Action, dup.Skip, SkipDuplicateKey)
			}
			if dup.Reason == "" {
				t.Errorf("duplicate-key skip carries no reason")
			}
		})
	}
}

func TestPlanTransform_NonMosaicFilesAreSkippedWithReason(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	dir := src.desc.Paths.Agents.Project
	tpPlaceAgent(t, ws, dir, "subagent.md", "subagent.md")
	for _, f := range []string{"plain-notes", "no-frontmatter", "deploy-todo"} {
		tpPlaceAgent(t, ws, dir, f+".md", f+".md")
	}

	p := tpPlan(t, ws, src, tgt, false)

	for _, f := range []string{"plain-notes.md", "no-frontmatter.md", "deploy-todo.md"} {
		a := tpAgent(t, p, f)
		if a.Action != PlanSkip || a.Skip != SkipNotAgent {
			t.Errorf("%s = action %q skip %q, want skip %q", f, a.Action, a.Skip, SkipNotAgent)
		}
		if a.Reason == "" {
			t.Errorf("%s skipped without a reason", f)
		}
		if a.DestinationPath != "" || a.Key != "" {
			t.Errorf("%s carries key %q destination %q, want both empty", f, a.Key, a.DestinationPath)
		}
	}
	if a := tpAgent(t, p, "subagent.md"); a.Action != PlanWriteCreate {
		t.Errorf("subagent.md Action = %q, want create", a.Action)
	}
}

func TestPlanTransform_BlockingStructureIssueOrEmptyStampIsNotAnAgent(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	dir := src.desc.Paths.Agents.Project
	tpWriteFile(t, ws, dir+"/broken.md", "---\nmosaic_harness_version: 1.2.0\nname: broken\n---\n\n<Identity type=\"core\">\nnever closed\n")
	tpWriteFile(t, ws, dir+"/emptystamp.md", "---\nmosaic_harness_version: \"\"\nname: e\n---\n\nBody.\n")

	p := tpPlan(t, ws, src, tgt, false)

	for _, f := range []string{"broken.md", "emptystamp.md"} {
		if a := tpAgent(t, p, f); a.Skip != SkipNotAgent || a.Reason == "" {
			t.Errorf("%s = skip %q reason %q, want not-agent with a reason", f, a.Skip, a.Reason)
		}
	}
}

func TestPlanTransform_LegacyTransformVersionStampIsRecognised(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpWriteFile(t, ws, src.desc.Paths.Agents.Project+"/legacy.md", "---\ntransform_version: 1.0.0\nname: legacy\n---\n\nBody.\n")

	p := tpPlan(t, ws, src, tgt, false)

	if a := tpAgent(t, p, "legacy.md"); a.Skip == SkipNotAgent {
		t.Errorf("legacy-stamped agent skipped as not-agent: %s", a.Reason)
	}
}

func TestPlanTransform_OnlyMdFilesInTheFolderItselfAreListed(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	dir := src.desc.Paths.Agents.Project
	tpPlaceAgent(t, ws, dir, "subagent.md", "subagent.md")
	tpPlaceAgent(t, ws, dir+"/nested", "deep.md", "subagent.md")
	tpWriteFile(t, ws, dir+"/notes.txt", "text")

	p := tpPlan(t, ws, src, tgt, false)

	if names := tpAgentNames(p); len(names) != 1 || names[0] != "subagent.md" {
		t.Errorf("planned files = %v, want only subagent.md", names)
	}
}

func TestPlanTransform_MissingAgentsFolderGivesNoAgentsNoteNamingIt(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)

	p := tpPlan(t, ws, src, tgt, false)

	if len(p.Agents) != 0 {
		t.Errorf("Agents = %v, want none", tpAgentNames(p))
	}
	notes := tpNotes(p, NoteNoAgentsFound)
	if len(notes) != 1 {
		t.Fatalf("no-agents-found notes = %d, want 1: %+v", len(notes), p.Notes)
	}
	if !tpNoteMentions(notes[0], filepath.ToSlash(filepath.Join(ws, ".claude", "agents"))) {
		t.Errorf("note does not name the folder looked in: %+v", notes[0])
	}
	if want := filepath.Join(ws, ".claude", "agents"); filepath.Clean(p.SourceAgentsDir) != want {
		t.Errorf("SourceAgentsDir = %q, want %q", p.SourceAgentsDir, want)
	}
}

func TestPlanTransform_FolderWithoutAnyMosaicAgentGivesNoAgentsNote(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAgent(t, ws, src.desc.Paths.Agents.Project, "plain-notes.md", "plain-notes.md")

	p := tpPlan(t, ws, src, tgt, false)

	if len(p.Agents) != 1 || p.Agents[0].Skip != SkipNotAgent {
		t.Fatalf("Agents = %+v, want the one file listed as not-agent", p.Agents)
	}
	notes := tpNotes(p, NoteNoAgentsFound)
	if len(notes) != 1 || !strings.Contains(filepath.ToSlash(strings.Join(notes[0].Paths, " ")+notes[0].Message), ".claude/agents") {
		t.Errorf("no-agents-found notes = %+v, want one naming .claude/agents", notes)
	}
}

func TestPlanTransform_GhcpCliSourceWithPlainMdFileIsFoundWithKeyWithoutExtension(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpCLI), tpHarness(t, tpClaude)
	tpPlaceAgent(t, ws, src.desc.Paths.Agents.Project, "x.md", "subagent.md")

	a := tpAgent(t, tpPlan(t, ws, src, tgt, false), "x.md")

	if a.Key != "x" || a.Action != PlanWriteCreate || tpDest(a) != ".claude/agents/x.md" {
		t.Errorf("key %q action %q destination %q, want x, create at .claude/agents/x.md", a.Key, a.Action, tpDest(a))
	}
}

func TestPlanTransform_VSCodeGhcpSourceRecognisesAgentMdAndLegacyMdFiles(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpVSCode), tpHarness(t, tpClaude)
	tpPlaceAgent(t, ws, src.desc.Paths.Agents.Project, "x.agent.md", "subagent.md")
	tpPlaceAgent(t, ws, src.desc.Paths.Agents.Project, "y.md", "subagent.md")

	p := tpPlan(t, ws, src, tgt, false)

	for _, k := range []string{"x", "y"} {
		a := tpAgent(t, p, map[string]string{"x": "x.agent.md", "y": "y.md"}[k])
		if a.Key != k || a.Action != PlanWriteCreate || tpDest(a) != ".claude/agents/"+k+".md" {
			t.Errorf("%s: key %q action %q destination %q, want create at .claude/agents/%s.md", k, a.Key, a.Action, tpDest(a), k)
		}
	}
}

func TestPlanTransform_ReturnsErrorWhenWorkspaceRootCannotBeRead(t *testing.T) {
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	missing := filepath.Join(t.TempDir(), "no-such-workspace")

	_, err := tpService().planTransform(transformPlanInput{Workspace: missing, Source: src, Target: tgt})

	if err == nil || errors.Is(err, errTransformPlanNotImplemented) {
		t.Errorf("error = %v, want a workspace-read error for a missing workspace root", err)
	}
}

func TestPlanTransform_MissingAgentsFolderIsNotAnError(t *testing.T) {
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)

	p, err := tpService().planTransform(transformPlanInput{Workspace: t.TempDir(), Source: src, Target: tgt})

	if err != nil || len(p.Agents) != 0 {
		t.Errorf("err %v agents %v, want no error and no agents", err, tpAgentNames(p))
	}
}
