package cli_test

// dropped_entries_output_test.go specifies how the CLI lists the manifest entries dropped
// before planning: in the dry-run report of the five write commands and in the human output of
// the transform command, each as "drop <path> - <reason>" under "dropped manifest entries:".

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	dropGhostPath    = ".github/agents/ghost.agent.md"
	dropMigratedPath = "docs/notes.md"
)

func droppedEntries() []domain.DroppedEntry {
	return []domain.DroppedEntry{
		{Entry: domain.ManifestEntry{Harness: "ghcp-cli", Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "ghost"},
			TargetPath: dropGhostPath}, Reason: domain.DropReasonMissing},
		{Entry: domain.ManifestEntry{Ref: domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "notes"},
			TargetPath: dropMigratedPath}, Reason: domain.DropReasonUnattributed},
	}
}

func TestDryRunCommands_ListEveryDroppedEntryWithPathAndReason(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			plan := dryRunPlan(ws, c.mode, true)
			plan.DroppedEntries = droppedEntries()

			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

			want := "dropped manifest entries:\n" +
				"  drop " + dropGhostPath + " - " + domain.DropReasonMissing + "\n" +
				"  drop " + dropMigratedPath + " - " + domain.DropReasonUnattributed + "\n"
			if !strings.Contains(out, want) {
				t.Errorf("report lacks the dropped section.\nwant:\n%s\ngot:\n%s", want, out)
			}
		})
	}
}

func TestDryRunCommands_DroppedSectionFollowsRemovalsAndReplacements(t *testing.T) {
	ws := "/ws/dry-run"
	c := dryRunCommands(ws)[0]
	plan := dryRunPlan(ws, c.mode, true)
	plan.DroppedEntries = droppedEntries()

	out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

	replacements := strings.Index(out, "replacements:")
	dropped := strings.Index(out, "dropped manifest entries:")
	if replacements < 0 || dropped < replacements {
		t.Errorf("the dropped section (at %d) must follow the replacements section (at %d):\n%s", dropped, replacements, out)
	}
}

func TestDryRunCommands_WithoutDroppedEntries_PrintNoDroppedSection(t *testing.T) {
	ws := "/ws/dry-run"
	for _, c := range dryRunCommands(ws) {
		t.Run(c.name, func(t *testing.T) {
			out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, dryRunPlan(ws, c.mode, true)))

			if strings.Contains(out, "dropped manifest entries") {
				t.Errorf("a plan without dropped entries printed a dropped section:\n%s", out)
			}
		})
	}
}

func TestDryRunCommands_DroppedEntriesAlone_AreStillListed(t *testing.T) {
	ws := "/ws/dry-run"
	c := dryRunCommands(ws)[1]
	plan := &domain.Plan{Mode: c.mode, WorkspacePath: ws, DroppedEntries: droppedEntries()}

	out, _, _, _ := runDryRun(t, c, dryRunSummary(ws, c.mode, plan))

	if !strings.Contains(out, "  drop "+dropGhostPath+" - "+domain.DropReasonMissing) {
		t.Errorf("a plan with nothing but dropped entries did not list them:\n%s", out)
	}
}

func transformDropped() []app.TransformDroppedEntry {
	return []app.TransformDroppedEntry{
		{Harness: "ghcp-cli", Kind: domain.ArtifactAgent, Key: "ghost", Path: dropGhostPath, Reason: domain.DropReasonMissing},
		{Kind: domain.ArtifactAgent, Key: "notes", Path: dropMigratedPath, Reason: domain.DropReasonUnattributed},
	}
}

func TestTransformCmd_ListsEveryDroppedEntryWithPathAndReason(t *testing.T) {
	for _, flag := range []string{"--dry-run", "--yes"} {
		t.Run(flag, func(t *testing.T) {
			res := planResult()
			res.DroppedEntries = transformDropped()

			_, out, _ := runTransform(&spyService{transformResp: res}, transformArgs(flag)...)

			want := "dropped manifest entries:\n" +
				"  drop " + dropGhostPath + " - " + domain.DropReasonMissing + "\n" +
				"  drop " + dropMigratedPath + " - " + domain.DropReasonUnattributed + "\n"
			if !strings.Contains(out, want) {
				t.Errorf("transform output lacks the dropped section.\nwant:\n%s\ngot:\n%s", want, out)
			}
		})
	}
}

func TestTransformCmd_WithoutDroppedEntries_PrintsNoDroppedSection(t *testing.T) {
	_, out, _ := runTransform(&spyService{transformResp: planResult()}, transformArgs("--yes")...)

	if strings.Contains(out, "dropped manifest entries") {
		t.Errorf("output without dropped entries printed a dropped section:\n%s", out)
	}
}
