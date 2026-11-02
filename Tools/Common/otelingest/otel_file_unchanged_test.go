package otelingest_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-common/otelingest"
)

// The OTel file belongs to the harness and the user: after an ingest it must be byte-identical, also when it
// holds corrupt lines and a trailing partial line, and no file may appear next to it.
func TestIngestLeavesOTelFileByteIdentical(t *testing.T) {
	caseDir := filepath.Join(conformanceDir, "robust_corrupt_and_interleaved")
	spec := loadCaseSpec(t, caseDir)
	w := newCaseWorkspace(t, caseDir, spec)
	step := spec.Steps[0]
	w.prepareStep(t, step)

	before, err := os.ReadFile(w.otelPath)
	if err != nil {
		t.Fatal(err)
	}
	siblingsBefore := dirNames(t, filepath.Dir(w.otelPath))

	res := otelingest.Ingest(w.request(step))

	if res.Status != otelingest.StatusIngested {
		t.Fatalf("status = %q (err %v), want %q", res.Status, res.Err, otelingest.StatusIngested)
	}
	after, err := os.ReadFile(w.otelPath)
	if err != nil {
		t.Fatalf("the OTel file is gone after the ingest: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the OTel file changed: %d bytes before, %d after", len(before), len(after))
	}
	if got := dirNames(t, filepath.Dir(w.otelPath)); !reflect.DeepEqual(siblingsBefore, got) {
		t.Errorf("files next to the OTel file changed: before %v, after %v", siblingsBefore, got)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
