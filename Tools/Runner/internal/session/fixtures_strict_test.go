package session_test

// Guard: every workflow region of every orchestrator fixture under
// testdata/session must be valid WorkflowFormat 2.16. The guard uses the
// strict parser and the admission check.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-run/internal/compat"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/orchfile"
	"mosaic-run/internal/workflow"
)

// fixtureAdmissionModes names the mode a fixture is admitted in when it is not
// the default Auto mode (the mode its tests run it in).
var fixtureAdmissionModes = map[string]domain.ExecutionMode{
	"fork-join-orch.md":                  domain.ExecutionModeOrchestrated,
	"fork-branch-ends-phase-orch.md":     domain.ExecutionModeOrchestrated,
	"phase-end-fork-all-leave-orch.md":   domain.ExecutionModeOrchestrated,
	"phase-end-fork-in-phase-orch.md":    domain.ExecutionModeOrchestrated,
	"phase-end-fork-join-leaves-orch.md": domain.ExecutionModeOrchestrated,
}

// admissionRefusedFixtures are valid 2.16 notation that admission refuses on
// purpose: their tests assert the start refusal.
var admissionRefusedFixtures = map[string]bool{
	"fork-branch-ends-phase-orch.md": true,
}

func TestSessionFixtures_EveryWorkflowRegion_ParsesStrictlyAndIsAdmitted(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(testdataDir, "*.md"))
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	checked := 0
	for _, path := range paths {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(data), "<Workflow") {
			continue // plan and schedule fixtures carry no workflow region
		}
		checked++
		t.Run(name, func(t *testing.T) {
			regions, err := orchfile.EnumerateWorkflows(path)
			if err != nil {
				t.Fatalf("enumerate workflows: %v", err)
			}
			mode := domain.ExecutionModeAuto
			if m, ok := fixtureAdmissionModes[name]; ok {
				mode = m
			}
			for _, region := range regions {
				table, err := workflow.Parse(region.Content, region.Info)
				if err != nil {
					t.Fatalf("workflow %q: strict parse refused: %v", region.Info.ID, err)
				}
				_, err = compat.Admit(table, mode)
				if admissionRefusedFixtures[name] {
					if err == nil {
						t.Fatalf("workflow %q: admission in mode %q admitted a fixture its tests expect refused", region.Info.ID, mode)
					}
					if !strings.Contains(err.Error(), "fork branch") {
						t.Fatalf("workflow %q: admission refused for an unexpected reason: %v", region.Info.ID, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("workflow %q: admission in mode %q refused: %v", region.Info.ID, mode, err)
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("no orchestrator fixture was checked")
	}
}
