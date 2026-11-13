package findingsfile_test

// Shared fixtures and helpers of the findings file tests.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/findingsfile"
)

const (
	runA = "20261031T073445Z-0ada"
	runB = "20261101T080000Z-beef"
)

// sevenKeys are the keys every finding entry carries, empty values included.
var sevenKeys = []string{"kind", "severity", "path", "line", "run_id", "actor", "detail"}

// document is the decoded findings file.
type document struct {
	Counts   map[string]int
	Findings []map[string]any
	raw      map[string]json.RawMessage
	text     string
}

// writeDoc writes findings to a new file in a temp dir and returns its path.
func writeDoc(t *testing.T, findings []domain.Finding) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := findingsfile.NewWriter().WriteFindings(context.Background(), path, findings, false); err != nil {
		t.Fatalf("WriteFindings: %v", err)
	}
	return path
}

// readDoc decodes the findings file at path.
func readDoc(t *testing.T, path string) document {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	doc := document{text: string(data)}
	if err := json.Unmarshal(data, &doc.raw); err != nil {
		t.Fatalf("the file is not one JSON object: %v\n%s", err, data)
	}
	if err := json.Unmarshal(doc.raw["counts"], &doc.Counts); err != nil {
		t.Fatalf("counts is not an object of numbers: %v\n%s", err, data)
	}
	if err := json.Unmarshal(doc.raw["findings"], &doc.Findings); err != nil {
		t.Fatalf("findings is not an array of objects: %v\n%s", err, data)
	}
	return doc
}

func finding(kind domain.FindingKind, sev domain.FindingSeverity, run domain.RunRef) domain.Finding {
	return domain.Finding{Kind: kind, Severity: sev, Run: run}
}

// mixedFindings spans two named runs, the unattributable bucket and no run, with
// several kinds, severities and a finding that carries every field.
func mixedFindings() []domain.Finding {
	a, b := domain.NamedRun(runA), domain.NamedRun(runB)
	w, i := domain.SeverityWarning, domain.SeverityInfo
	return []domain.Finding{
		{Kind: domain.FindingMalformedLine, Severity: w, Run: a, Actor: domain.AgentInstance("coder#8"), Path: "logs/a/events.jsonl", Line: 42, Detail: "bad json"},
		finding(domain.FindingCrossStreamRecord, w, a),
		finding(domain.FindingCrossStreamRecord, w, b),
		finding(domain.FindingMissingModel, i, b),
		finding(domain.FindingMalformedLine, w, domain.UnattributableRun()),
		finding(domain.FindingInvalidSettingsEntry, w, domain.RunRef{}),
	}
}
