package testcatalog_test

// Admission check for the MosaicTest workflows shipped in the Runner's own
// TestCatalog. Every workflow must parse strictly (WorkflowFormat 2.16, no 1.x
// notation) and be admitted in each execution mode its frontmatter declares.
// One subtest per workflow and mode: a failing subtest names a workflow that is
// still written in 1.x notation or that the Runner would refuse at run start.
//
// The test reads only the Runner's own tree (TestCatalog/), never Catalog/.

import (
	"path/filepath"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/testcatalog"
	"mosaic-run/internal/workflow"
)

const mosaicTestCatalogRoot = "../../TestCatalog"

func TestMosaicTestWorkflows_ParseStrictAndAdmitInEveryDeclaredMode(t *testing.T) {
	cat, err := testcatalog.Load(mosaicTestCatalogRoot)
	if err != nil {
		t.Fatalf("load MosaicTest catalog: %v", err)
	}
	entries := cat.FullSuite()
	if len(entries) == 0 {
		t.Fatal("MosaicTest catalog lists no workflows")
	}

	for _, entry := range entries {
		entry := entry
		t.Run(entry.WorkflowID+"/"+entry.Mode, func(t *testing.T) {
			path := filepath.Join(mosaicTestCatalogRoot, "Workflows", "MosaicTest", entry.WorkflowID+".md")
			region, err := orchfile.GetWorkflow(path, entry.WorkflowID)
			if err != nil {
				t.Fatalf("read workflow region: %v", err)
			}
			table, err := workflow.Parse(region.Content, region.Info)
			if err != nil {
				t.Fatalf("strict parse refused: %v", err)
			}
			mode, err := domain.ParseExecutionMode(entry.Mode)
			if err != nil {
				t.Fatalf("mode %q: %v", entry.Mode, err)
			}
			if _, err := compat.Admit(table, mode); err != nil {
				t.Fatalf("admission refused in mode %s: %v", entry.Mode, err)
			}
		})
	}
}
