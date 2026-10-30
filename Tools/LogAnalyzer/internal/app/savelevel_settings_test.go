package app_test

// Tests for marking a tool as user-interaction at a chosen configuration level.
//
// Rules under test:
//   - Before saving, the flow asks once which level to save to, whatever the
//     number of harnesses; the chosen level decides which settings file is written.
//   - The result names the file written and its level.
//   - Skipping the level question saves nothing.
//   - When the global folder is unavailable the global option is disabled.

import (
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestMarkUserInteractionTool_AsksTheLevelOnceWithBothSettingsFiles(t *testing.T) {
	m := newMarkEnv(t, true)

	if _, err := m.mark(markedTool, domain.HarnessClaudeCode, domain.HarnessGHCPCLI); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	questions := levelQuestions(m.ia)
	if len(questions) != 1 {
		t.Fatalf("level questions asked = %d, want exactly 1 for a mark over two harnesses", len(questions))
	}
	assertLevelQuestionShape(t, questions[0], m.svc.SettingsPaths())
}

func TestMarkUserInteractionTool_GlobalUnavailable_DisablesTheGlobalOption(t *testing.T) {
	m := newMarkEnv(t, true)
	m.store.globalUnavailable = true

	if _, err := m.mark(markedTool, domain.HarnessClaudeCode); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	questions := levelQuestions(m.ia)
	if len(questions) == 0 {
		t.Fatal("the flow must ask the level question")
	}
	assertGlobalUnavailableShape(t, questions[0])
}

func TestMarkUserInteractionTool_TheChosenLevelDecidesTheFileWritten(t *testing.T) {
	cases := []struct {
		option string
		level  domain.ConfigLevel
		path   func(m markEnv) string
	}{
		{"global", domain.LevelGlobal, func(markEnv) string { return fakeGlobalSettingsPath }},
		{"project", domain.LevelProject, func(m markEnv) string { return m.store.filePath }},
	}
	for _, c := range cases {
		t.Run(c.option, func(t *testing.T) {
			m := newMarkEnv(t, true)
			m.ia.levelOption = c.option

			got, err := m.mark(markedTool, domain.HarnessClaudeCode, domain.HarnessGHCPCLI)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(m.store.levels) != 2 || m.store.levels[0] != c.level || m.store.levels[1] != c.level {
				t.Errorf("saved at levels %v, want [%v %v] (one save per harness)", m.store.levels, c.level, c.level)
			}
			if got.ConfigPath != c.path(m) || got.ConfigLevel != c.level {
				t.Errorf("ConfigPath/ConfigLevel = %q/%v, want %q/%v", got.ConfigPath, got.ConfigLevel, c.path(m), c.level)
			}
		})
	}
}

func TestMarkUserInteractionTool_GlobalSave_ReportReflectsTheMark(t *testing.T) {
	m := newMarkEnv(t, true)
	m.ia.levelOption = "global"

	got, err := m.mark(markedTool, domain.HarnessClaudeCode)
	if len(m.store.levels) != 1 || m.store.levels[0] != domain.LevelGlobal {
		t.Fatalf("saved at levels %v, want [global]", m.store.levels)
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, counted := markWaiting(t, got.Report); len(counted) != 1 || counted[0].Tool != markedTool {
		t.Errorf("counted tools = %v, want %s after a global save", counted, markedTool)
	}
}

func TestMarkUserInteractionTool_SkippedLevel_SavesNothing(t *testing.T) {
	m := newMarkEnv(t, true)
	m.ia.skipLevel = true

	got, _ := m.mark(markedTool, domain.HarnessClaudeCode)

	if len(m.store.adds) != 0 {
		t.Errorf("nothing may be saved after the level question is skipped; saved %v", m.store.adds)
	}
	if got.ConfigPath != "" {
		t.Errorf("ConfigPath = %q, want empty when nothing was written", got.ConfigPath)
	}
}
