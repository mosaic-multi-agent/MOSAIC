package settings_test

// Tests for Store.Load across the global and project levels: built-in defaults <
// global < project, merged per setting and, for tool lists, per harness.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

func TestLoadLayers_NoFiles_YieldsBuiltInDefaultsWithoutFindings(t *testing.T) {
	l := newLayered(t)

	got, findings := loadLayered(t, l)

	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadLayers_CommentsOnlyFiles_YieldDefaults(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "# only comments\n")
	writeAt(t, l.projFile, "# only comments\n")

	got, findings := loadLayered(t, l)

	assertBuiltInDefaults(t, got)
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestLoadLayers_GlobalOnly_AppliesGlobalSettings(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "write_tools:\n  claude-code: [GlobalWrite]\ndetected_compaction:\n  drop_percent: 40\n")

	got, _ := loadLayered(t, l)

	if !got.IsWriteTool(domain.HarnessClaudeCode, "GlobalWrite") || got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("the global claude-code write list must replace the default list")
	}
	if got.CompactionDropPercent != 40 {
		t.Errorf("CompactionDropPercent = %d, want 40", got.CompactionDropPercent)
	}
}

func TestLoadLayers_ProjectOnly_AppliesProjectSettings(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, "detected_compaction:\n  drop_percent: 55\n")

	got, _ := loadLayered(t, l)

	if got.CompactionDropPercent != 55 {
		t.Errorf("CompactionDropPercent = %d, want 55", got.CompactionDropPercent)
	}
}

func TestLoadLayers_ToolLists_MergePerHarnessProjectReplacesGlobalListOfThatHarness(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "write_tools:\n  claude-code: [GlobalClaude]\n  opencode: [GlobalOpen]\n")
	writeAt(t, l.projFile, "write_tools:\n  claude-code: [ProjectClaude]\n")

	got, _ := loadLayered(t, l)

	if !got.IsWriteTool(domain.HarnessClaudeCode, "ProjectClaude") || got.IsWriteTool(domain.HarnessClaudeCode, "GlobalClaude") {
		t.Error("within one harness the project list must replace the global list")
	}
	if !got.IsWriteTool(domain.HarnessOpenCode, "GlobalOpen") || got.IsWriteTool(domain.HarnessOpenCode, "apply_patch") {
		t.Error("a harness the project does not list must keep the global list")
	}
	if !got.IsWriteTool(domain.HarnessGHCPCLI, "create") {
		t.Error("a harness no file lists must keep its built-in default")
	}
}

func TestLoadLayers_UserInteractionTools_MergePerHarness(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "user_interaction_tools:\n  claude-code: [GlobalAsk]\n  ghcp-cli: [GlobalGhcp]\n")
	writeAt(t, l.projFile, "user_interaction_tools:\n  ghcp-cli: [ProjectGhcp]\n")

	got, _ := loadLayered(t, l)

	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "GlobalAsk") {
		t.Error("global claude-code list must stay when the project lists another harness")
	}
	if !got.IsUserInteractionTool(domain.HarnessGHCPCLI, "ProjectGhcp") || got.IsUserInteractionTool(domain.HarnessGHCPCLI, "GlobalGhcp") {
		t.Error("project ghcp-cli list must replace the global one")
	}
}

func TestLoadLayers_ProjectFileDoesNotHideOtherGlobalSettings(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "detected_compaction:\n  drop_percent: 40\nuser_interaction_tools:\n  claude-code: [GlobalAsk]\n")
	writeAt(t, l.projFile, "write_tools:\n  claude-code: [ProjectWrite]\n")

	got, _ := loadLayered(t, l)

	if got.CompactionDropPercent != 40 {
		t.Errorf("CompactionDropPercent = %d, want the global 40", got.CompactionDropPercent)
	}
	if !got.IsUserInteractionTool(domain.HarnessClaudeCode, "GlobalAsk") {
		t.Error("global user_interaction_tools must survive a project file that sets other keys")
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "ProjectWrite") {
		t.Error("project write_tools must apply")
	}
}

func TestLoadLayers_DropPercent_ProjectWinsOverGlobal(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "detected_compaction:\n  drop_percent: 40\n")
	writeAt(t, l.projFile, "detected_compaction:\n  drop_percent: 55\n")

	got, _ := loadLayered(t, l)

	if got.CompactionDropPercent != 55 {
		t.Errorf("CompactionDropPercent = %d, want the project's 55", got.CompactionDropPercent)
	}
}

func TestLoadLayers_UnknownKeys_AreFindingsCarryingTheirFilePath(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "global_unknown: 1\n")
	writeAt(t, l.projFile, "project_unknown: 1\n")

	_, findings := loadLayered(t, l)

	g, p := findingsAt(findings, l.globalFile), findingsAt(findings, l.projFile)
	if len(g) != 1 || !strings.Contains(g[0].Detail, "global_unknown") {
		t.Errorf("global findings = %v, want one naming global_unknown", g)
	}
	if len(p) != 1 || !strings.Contains(p[0].Detail, "project_unknown") {
		t.Errorf("project findings = %v, want one naming project_unknown", p)
	}
	if countKind(findings, domain.FindingInvalidSettingsEntry) != 2 {
		t.Errorf("findings = %v, want two invalid_settings_entry findings", findings)
	}
}

func TestLoadLayers_InvalidValues_AreFindingsWithTheirFilePathAndOtherSettingsStillApply(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "detected_compaction:\n  drop_percent: 0\n")
	writeAt(t, l.projFile, "write_tools:\n  claude-code: not-a-list\nuser_interaction_tools:\n  opencode: [ProjectAsk]\n")

	got, findings := loadLayered(t, l)

	if len(findingsAt(findings, l.globalFile)) != 1 {
		t.Errorf("want one finding for the invalid global drop_percent, got %v", findings)
	}
	if len(findingsAt(findings, l.projFile)) != 1 {
		t.Errorf("want one finding for the invalid project write_tools, got %v", findings)
	}
	if !got.IsUserInteractionTool(domain.HarnessOpenCode, "ProjectAsk") {
		t.Error("a valid project entry must apply next to an invalid one")
	}
	if !got.IsWriteTool(domain.HarnessClaudeCode, "Write") {
		t.Error("an invalid project list must leave the default list in place")
	}
}

func TestLoadLayers_UnparseableGlobalFile_ErrorNamesThatFile(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.globalFile, "write_tools: {bad yaml: [")

	_, _, err := l.store.Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), l.globalFile) {
		t.Errorf("error = %v, want one naming %s", err, l.globalFile)
	}
}

func TestLoadLayers_UnparseableProjectFile_ErrorNamesThatFile(t *testing.T) {
	l := newLayered(t)
	writeAt(t, l.projFile, "write_tools: {bad yaml: [")

	_, _, err := l.store.Load(context.Background())

	if err == nil || !strings.Contains(err.Error(), l.projFile) {
		t.Errorf("error = %v, want one naming %s", err, l.projFile)
	}
}

func TestLoadLayers_Load_NeverCreatesEitherFile(t *testing.T) {
	l := newLayered(t)

	loadLayered(t, l)

	if fileExists(l.globalFile) || fileExists(l.projFile) {
		t.Error("Load must not write")
	}
}
