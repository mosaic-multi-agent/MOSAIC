package app

// transform_plan_destination_test.go specifies destination and action planning: where agents
// are written, the target-extension exclusion, existing destinations, destinations that are
// source files, repeat runs and the both-forms warning.

import (
	"reflect"
	"strings"
	"testing"
)

// tpMaterialize writes a stand-in output file for every planned write, as a finished
// transform leaves them on disk.
func tpMaterialize(t *testing.T, ws string, p TransformPlan) {
	t.Helper()
	for _, a := range p.Agents {
		if a.Action == PlanWriteCreate || a.Action == PlanWriteOverwrite {
			tpWriteFile(t, ws, a.DestinationPath, tpFixture(t, "agents/subagent.md"))
		}
	}
}

// tpWrites returns the planned agents whose action writes.
func tpWrites(p TransformPlan) []PlannedAgent {
	var out []PlannedAgent
	for _, a := range p.Agents {
		if a.Action == PlanWriteCreate || a.Action == PlanWriteOverwrite {
			out = append(out, a)
		}
	}
	return out
}

func TestPlanTransform_DestinationIsTargetAgentsFolderWithTargetExtension(t *testing.T) {
	cases := []struct{ name, source, target, srcName, want string }{
		{"to ghcp-cli keeps agent.md naming", tpClaude, tpCLI, "x.md", ".github/agents/x.agent.md"},
		{"to vscode-ghcp uses the same agent.md naming", tpClaude, tpVSCode, "x.md", ".github/agents/x.agent.md"},
		{"to claude-code", tpCLI, tpClaude, "x.agent.md", ".claude/agents/x.md"},
		{"from vscode-ghcp agent.md file", tpVSCode, tpClaude, "x.agent.md", ".claude/agents/x.md"},
		{"from vscode-ghcp legacy md file", tpVSCode, tpClaude, "x.md", ".claude/agents/x.md"},
		{"to opencode", tpClaude, tpOpen, "x.md", ".opencode/agents/x.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, c.source), tpHarness(t, c.target)
			tpPlaceAgent(t, ws, src.desc.Paths.Agents.Project, c.srcName, "subagent.md")

			a := tpAgent(t, tpPlan(t, ws, src, tgt, false), c.srcName)

			if a.Action != PlanWriteCreate || tpDest(a) != c.want {
				t.Errorf("action %q destination %q, want create at %q", a.Action, tpDest(a), c.want)
			}
		})
	}
}

func TestPlanTransform_PlanningLeavesTheWorkspaceUnchanged(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAllAgents(t, ws, src)
	before := tpSnapshot(t, ws)

	tpPlan(t, ws, src, tgt, true)

	if after := tpSnapshot(t, ws); !reflect.DeepEqual(before, after) {
		t.Errorf("planning changed the workspace: before %d files, after %d files", len(before), len(after))
	}
}

func TestPlanTransform_TargetExtensionFilesAreExcludedOnlyWhenBothHarnessesShareTheFolder(t *testing.T) {
	t.Run("same folder: vscode-ghcp agent files of both names are recognised, none is a target-extension skip", func(t *testing.T) {
		ws := t.TempDir()
		src, tgt := tpHarness(t, tpVSCode), tpHarness(t, tpCLI)
		tpPlaceAgent(t, ws, ".github/agents", "x.agent.md", "subagent.md")
		tpPlaceAgent(t, ws, ".github/agents", "y.md", "subagent.md")
		tpRecord(t, ws, "vscode-ghcp",
			tpAgentEntry("x", ".github/agents/x.agent.md"),
			tpAgentEntry("y", ".github/agents/y.md"))

		p := tpPlan(t, ws, src, tgt, false)

		for _, f := range []string{"x.agent.md", "y.md"} {
			a := tpAgent(t, p, f)
			if a.Skip == SkipTargetExtension || a.Skip == SkipNotAgent {
				t.Errorf("%s skipped as %q: %s", f, a.Skip, a.Reason)
			}
			if a.Action != PlanWriteCreate && a.Action != PlanWriteOverwrite {
				t.Errorf("%s action %q, want a planned write", f, a.Action)
			}
			if k := tpAgentKeyOf(f); a.Key != k || tpDest(a) != ".github/agents/"+k+".agent.md" {
				t.Errorf("%s key %q destination %q, want %s at .github/agents/%s.agent.md", f, a.Key, tpDest(a), k, k)
			}
		}
	})

	t.Run("different folders: a target-extension name stays a source", func(t *testing.T) {
		ws := t.TempDir()
		src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpCLI)
		tpPlaceAgent(t, ws, ".claude/agents", "x.agent.md", "subagent.md")

		a := tpAgent(t, tpPlan(t, ws, src, tgt, false), "x.agent.md")

		if a.Skip == SkipTargetExtension {
			t.Errorf("x.agent.md skipped as target-extension although the folders differ")
		}
		if tpDest(a) != ".github/agents/x.agent.md" || a.Action != PlanWriteCreate {
			t.Errorf("x.agent.md = action %q destination %q, want create at .github/agents/x.agent.md", a.Action, tpDest(a))
		}
	})
}

func TestPlanTransform_ExistingDestinationIsSkippedAndKeptUnlessOverwrite(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
	tpPlaceAgent(t, ws, ".claude/agents", "x.md", "subagent.md")
	tpWriteFile(t, ws, ".opencode/agents/x.md", "KEEP ME\n")

	skipped := tpAgent(t, tpPlan(t, ws, src, tgt, false), "x.md")
	rebuilt := tpAgent(t, tpPlan(t, ws, src, tgt, true), "x.md")

	if skipped.Action != PlanSkip || skipped.Skip != SkipDestinationExists || skipped.Reason == "" {
		t.Errorf("without overwrite: action %q skip %q reason %q, want skip %q with a reason", skipped.Action, skipped.Skip, skipped.Reason, SkipDestinationExists)
	}
	if tpDest(skipped) != ".opencode/agents/x.md" {
		t.Errorf("skipped destination = %q, want .opencode/agents/x.md", tpDest(skipped))
	}
	if rebuilt.Action != PlanWriteOverwrite || rebuilt.Skip != "" {
		t.Errorf("with overwrite: action %q skip %q, want overwrite and no skip", rebuilt.Action, rebuilt.Skip)
	}
	if got := tpSnapshot(t, ws)[".opencode/agents/x.md"]; got != "KEEP ME\n" {
		t.Errorf("planning changed the existing destination: %q", got)
	}
}

// tpAgentKeyOf returns the agent key of a GHCP agent file name (X.agent.md or X.md).
func tpAgentKeyOf(fileName string) string {
	return strings.TrimSuffix(strings.TrimSuffix(fileName, ".md"), ".agent")
}

func TestPlanTransform_DestinationThatIsASourceFileIsStillSkippedOutsideTheGhcpPair(t *testing.T) {
	// A target whose agents folder and naming equal the source's: the destination is the
	// source file, which transform never writes over.
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpClaude)
	tpPlaceAgent(t, ws, ".claude/agents", "x.md", "subagent.md")
	before := tpSnapshot(t, ws)

	p := tpPlan(t, ws, src, tgt, true)

	a := tpAgent(t, p, "x.md")
	if a.Action != PlanSkip || a.Skip != SkipDestinationIsSource || a.Reason == "" {
		t.Errorf("x.md = action %q skip %q reason %q, want skip %q with a reason", a.Action, a.Skip, a.Reason, SkipDestinationIsSource)
	}
	if after := tpSnapshot(t, ws); !reflect.DeepEqual(before, after) {
		t.Errorf("source files changed by planning")
	}
}

func TestPlanTransform_DestinationIsSourceComparisonIgnoresCase(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpVSCode), tpHarness(t, tpCLI)
	tpPlaceAgent(t, ws, ".github/agents", "Foo.md", "subagent.md")
	tpPlaceAgent(t, ws, ".github/agents", "foo.agent.md", "subagent.md")

	a := tpAgent(t, tpPlan(t, ws, src, tgt, true), "Foo.md")

	if a.Action == PlanWriteCreate || a.Action == PlanWriteOverwrite {
		t.Errorf("Foo.md planned to write %s over a source file that differs only in case", a.DestinationPath)
	}
}

func TestPlanTransform_RepeatedPlanOverTransformedWorkspaceWritesNothing(t *testing.T) {
	harnesses := []string{tpClaude, tpOpen, tpCLI, tpVSCode}
	for _, s := range harnesses {
		for _, g := range harnesses {
			cliVsCode := (s == tpCLI && g == tpVSCode) || (s == tpVSCode && g == tpCLI)
			if s == g || cliVsCode {
				continue
			}
			t.Run(s+"->"+g, func(t *testing.T) {
				ws := t.TempDir()
				src, tgt := tpHarness(t, s), tpHarness(t, g)
				tpPlaceAllAgents(t, ws, src)

				first := tpPlan(t, ws, src, tgt, false)
				if len(tpWrites(first)) != len(tpAgentKinds) {
					t.Fatalf("first plan writes %d agents, want %d", len(tpWrites(first)), len(tpAgentKinds))
				}
				for _, a := range first.Agents {
					d := tpDest(a)
					if strings.Contains(d, ".agent.agent.") || strings.Contains(d, ".md.md") {
						t.Errorf("doubled extension in destination %q", d)
					}
				}
				tpMaterialize(t, ws, first)

				second := tpPlan(t, ws, src, tgt, false)

				if w := tpWrites(second); len(w) != 0 {
					t.Errorf("repeat plan writes %d agents: %+v", len(w), w)
				}
				for _, a := range second.Agents {
					if a.Skip != SkipDestinationExists {
						t.Errorf("%s repeat skip = %q, want %q", a.FileName, a.Skip, SkipDestinationExists)
					}
				}
			})
		}
	}
}

func TestPlanTransform_SharedFolderPairNeverWritesASourceFile(t *testing.T) {
	for _, pair := range [][2]string{{tpCLI, tpVSCode}, {tpVSCode, tpCLI}} {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			ws := t.TempDir()
			src, tgt := tpHarness(t, pair[0]), tpHarness(t, pair[1])
			for _, n := range []string{"a.md", "a.agent.md", "b.md", "c.agent.md"} {
				tpPlaceAgent(t, ws, ".github/agents", n, "subagent.md")
			}

			p := tpPlan(t, ws, src, tgt, true)

			for _, w := range tpWrites(p) {
				for _, s := range p.Agents {
					if strings.EqualFold(tpDest(w), ".github/agents/"+s.FileName) {
						t.Errorf("%s planned to write %s, which is source file %s", w.FileName, tpDest(w), s.FileName)
					}
				}
			}
		})
	}
}

func TestPlanTransform_BothFormsMatchesStemsIgnoringCase(t *testing.T) {
	ws := t.TempDir()
	src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpCLI)
	tpPlaceAgent(t, ws, ".claude/agents", "foo.md", "subagent.md")
	tpWriteFile(t, ws, ".github/agents/FOO.md", "existing plain copy\n")

	p := tpPlan(t, ws, src, tgt, false)

	if a := tpAgent(t, p, "foo.md"); a.Action != PlanWriteCreate {
		t.Fatalf("foo.md Action = %q, want create", a.Action)
	}
	notes := tpNotes(p, NoteBothForms)
	if len(notes) != 1 || !tpNoteMentions(notes[0], ".github/agents/FOO.md", ".github/agents/foo.agent.md") {
		t.Errorf("both-forms notes = %+v, want one naming FOO.md and foo.agent.md", notes)
	}
}

func TestPlanTransform_NoBothFormsNoteWhenNothingCollidesOrNothingIsWritten(t *testing.T) {
	t.Run("no collision", func(t *testing.T) {
		ws := t.TempDir()
		src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpOpen)
		tpPlaceAllAgents(t, ws, src)

		if notes := tpNotes(tpPlan(t, ws, src, tgt, false), NoteBothForms); len(notes) != 0 {
			t.Errorf("unexpected both-forms notes: %+v", notes)
		}
	})
	t.Run("existing pair and no planned write", func(t *testing.T) {
		ws := t.TempDir()
		src, tgt := tpHarness(t, tpClaude), tpHarness(t, tpCLI)
		tpPlaceAgent(t, ws, ".claude/agents", "x.md", "subagent.md")
		tpWriteFile(t, ws, ".github/agents/x.md", "plain\n")
		tpWriteFile(t, ws, ".github/agents/x.agent.md", "already transformed\n")

		p := tpPlan(t, ws, src, tgt, false)

		if a := tpAgent(t, p, "x.md"); a.Action != PlanSkip {
			t.Fatalf("x.md Action = %q, want skip (destination exists)", a.Action)
		}
		if notes := tpNotes(p, NoteBothForms); len(notes) != 0 {
			t.Errorf("both-forms note although no planned write is involved: %+v", notes)
		}
	})
}
