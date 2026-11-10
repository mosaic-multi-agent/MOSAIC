package otelingest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"mosaic-common/otelingest"
)

// TestConformance runs every case folder of testdata/conformance: for each step it builds the run folder,
// ingests the step's OTel file and compares streams, state, result and diagnostics with the expected files.
func TestConformance(t *testing.T) {
	for _, name := range listConformanceCases(t) {
		caseDir := filepath.Join(conformanceDir, name)
		spec := loadCaseSpec(t, caseDir)
		t.Run(name, func(t *testing.T) {
			w := newCaseWorkspace(t, caseDir, spec)
			for _, step := range spec.Steps {
				t.Run(step.Name, func(t *testing.T) { runConformanceStep(t, w, step) })
			}
		})
	}
}

func runConformanceStep(t *testing.T, w *caseWorkspace, step stepSpec) {
	w.prepareStep(t, step)
	otelBefore, _ := os.ReadFile(w.otelPath)

	res := otelingest.Ingest(w.request(step))

	assertResult(t, w, step.Name, res)
	assertDiagnostics(t, w, step.Name, res)
	assertStreams(t, w, step.Name)
	assertState(t, w, step.Name)
	assertLogsAppendOnly(t, w)
	otelAfter, _ := os.ReadFile(w.otelPath)
	if !bytes.Equal(otelBefore, otelAfter) {
		t.Errorf("the OTel file was modified by the ingest")
	}
}
