package otelingest_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"mosaic-common/otelingest"
)

const conformanceDir = "testdata/conformance"

// caseSpec mirrors case.json of a conformance case.
type caseSpec struct {
	Description string     `json:"description"`
	RunID       string     `json:"run_id"`
	Steps       []stepSpec `json:"steps"`
}

// stepSpec is one ingest of a case: the OTel file contents, an optional log overlay and the ingest parameters.
type stepSpec struct {
	Name        string   `json:"name"`
	OTel        *string  `json:"otel"`
	Logs        *string  `json:"logs"`
	Final       bool     `json:"final"`
	WholeFile   bool     `json:"whole_file"`
	StartOffset *int64   `json:"start_offset"`
	Scope       []string `json:"scope"`
}

// caseWorkspace is the temporary workspace one case runs in.
type caseWorkspace struct {
	caseDir   string
	logsRoot  string // <tmp>/OrchestrationLogs
	runDir    string
	otelPath  string
	logsAtRun map[string][]byte // MOSAIC log files as they were before the ingest
}

var fixedNow = time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)

func listConformanceCases(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance folder: %v", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("no conformance cases in %s", conformanceDir)
	}
	return names
}

func loadCaseSpec(t *testing.T, caseDir string) caseSpec {
	t.Helper()
	var spec caseSpec
	readJSONInto(t, filepath.Join(caseDir, "case.json"), &spec)
	if spec.RunID == "" || len(spec.Steps) == 0 {
		t.Fatalf("%s: case.json needs run_id and at least one step", caseDir)
	}
	return spec
}

// newCaseWorkspace builds <tmp>/OrchestrationLogs/<run_id>/ from the case's logs/ folder.
func newCaseWorkspace(t *testing.T, caseDir string, spec caseSpec) *caseWorkspace {
	t.Helper()
	tmp := t.TempDir()
	w := &caseWorkspace{
		caseDir:  caseDir,
		logsRoot: filepath.Join(tmp, "OrchestrationLogs"),
		otelPath: filepath.Join(tmp, "otel", "copilot-otel.jsonl"),
	}
	w.runDir = filepath.Join(w.logsRoot, spec.RunID)
	if err := os.MkdirAll(filepath.Dir(w.otelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(w.runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyTree(t, filepath.Join(caseDir, "logs"), w.runDir)
	return w
}

// prepareStep places the step's OTel file and log overlay before the ingest runs.
func (w *caseWorkspace) prepareStep(t *testing.T, step stepSpec) {
	t.Helper()
	if step.OTel == nil {
		if err := os.Remove(w.otelPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	} else {
		data, err := os.ReadFile(filepath.Join(w.caseDir, filepath.FromSlash(*step.OTel)))
		if err != nil {
			t.Fatalf("read step otel file: %v", err)
		}
		if err := os.WriteFile(w.otelPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if step.Logs != nil {
		copyTree(t, filepath.Join(w.caseDir, filepath.FromSlash(*step.Logs)), w.runDir)
	}
	w.logsAtRun = snapshotLogs(t, w.runDir)
}

func (w *caseWorkspace) request(step stepSpec) otelingest.Request {
	return otelingest.Request{
		RunDir:      w.runDir,
		OTelPath:    w.otelPath,
		Final:       step.Final,
		WholeFile:   step.WholeFile,
		StartOffset: step.StartOffset,
		Scope:       step.Scope,
		Now:         func() time.Time { return fixedNow },
	}
}

func readJSONInto(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

// copyTree copies every file under src into dst, overwriting existing files. A missing src is ignored.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// snapshotLogs reads every file of the run folder outside the dot-prefixed ingest state.
func snapshotLogs(t *testing.T, runDir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(runDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != runDir && d.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(runDir, p)
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot logs: %v", err)
	}
	return out
}
