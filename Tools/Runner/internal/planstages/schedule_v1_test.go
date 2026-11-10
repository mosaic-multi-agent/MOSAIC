package planstages_test

// Tests for V1 defects: missing or unreadable source, heading and table
// position, and the first column header.

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"mosaic-run/internal/planstages"
)

const stagesHeading = "## Stages"

func TestReadBlockSchedule_MissingFile_V1UnwrapsNotExist(t *testing.T) {
	req := stageRequest("does-not-exist.md", false)

	_, err := planstages.ReadBlockSchedule(req)

	de := requireDefect(t, err, req, "V1", 0, req.Source)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want error to unwrap to fs.ErrNotExist, got cause %v", de.Cause)
	}
}

func TestReadBlockSchedule_UnreadablePath_NoRuleAndNotNotExist(t *testing.T) {
	req := stageRequest("irrelevant.md", false)
	req.Path = t.TempDir() // a directory exists but cannot be read as a file

	_, err := planstages.ReadBlockSchedule(req)

	de := requireDefect(t, err, req, "", 0, req.Source)
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("unreadable path must not unwrap to fs.ErrNotExist: %v", err)
	}
	if de.Cause == nil {
		t.Errorf("want the I/O error as Cause, got nil")
	}
}

func TestReadBlockSchedule_HeadingDefects_V1NamesHeading(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
	}{
		{"missing heading", "sched-no-heading.md"},
		{"heading in another case", "sched-heading-lowercase.md"},
		{"level-3 heading of the same text", "sched-heading-level3-only.md"},
		{"heading twice", "sched-heading-twice.md"},
		{"table only after the next heading", "sched-table-after-next-heading.md"},
		{"no table before end of file", "sched-no-table-eof.md"},
		{"padded heading twice counts as twice", "sched-heading-padded-twice.md"},
		{"table only after a level-1 heading", "sched-table-after-level1-heading.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := stageRequest(tc.fixture, false)

			_, err := planstages.ReadBlockSchedule(req)

			de := requireDefect(t, err, req, "V1", 0, stagesHeading)
			if errors.Is(de, fs.ErrNotExist) {
				t.Errorf("a heading defect must not unwrap to fs.ErrNotExist")
			}
		})
	}
}

func TestReadBlockSchedule_FirstColumnNotBlockName_V1NamesFirstHeaderCell(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		token   string
	}{
		{"different case", "sched-first-column-lowercase.md", "stage"},
		{"block column not first", "sched-first-column-not-block.md", "Name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := stageRequest(tc.fixture, false)

			_, err := planstages.ReadBlockSchedule(req)

			requireDefect(t, err, req, "V1", 0, tc.token)
		})
	}
}

func TestReadBlockSchedule_BlockNameDiffersFromFirstColumn_V1(t *testing.T) {
	req := stageRequest("sched-planner-tdd-soft.md", true, groupedApproaches...)
	req.Block = "Generation"

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V1", 0, "Stage")
}

func TestReadBlockSchedule_DefectsCarryRequestSourceAndSection(t *testing.T) {
	req := stageRequest("sched-no-heading.md", false)
	req.Source = filepath.Join("Stage-{Run}", "Plan.md")
	req.Section = "Generation"
	req.Block = "Generation"

	_, err := planstages.ReadBlockSchedule(req)

	requireDefect(t, err, req, "V1", 0, "## Generation")
}

func TestReadBlockSchedule_NonDefaultSection_DefectsNameThatSection(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		rule    string
		row     int
		token   string
	}{
		{"missing heading", "sched-no-heading.md", "V1", 0, "## Generation"},
		{"heading without table", "sched-generation-no-table.md", "V1", 0, "## Generation"},
		{"invalid ID", "sched-generation-bad-id.md", "V2", 2, "b_2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := stageRequest(tc.fixture, false)
			req.Section = "Generation"
			req.Block = "Generation"

			_, err := planstages.ReadBlockSchedule(req)

			de := requireDefect(t, err, req, tc.rule, tc.row, tc.token)
			if de.Section != "Generation" {
				t.Errorf("Section: want Generation, got %q", de.Section)
			}
		})
	}
}
