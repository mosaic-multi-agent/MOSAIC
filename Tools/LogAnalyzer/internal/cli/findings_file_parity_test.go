package cli_test

// Field-name parity: an entry of the findings export file carries exactly the
// keys of a finding in `report --format json` (quality.findings), so the two
// can be read with the same code. The export differs only in always writing
// every key; report omits empty ones.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/findingsfile"
)

// fullFinding sets every field, so that report's omit-empty rule drops no key.
func fullFinding() domain.Finding {
	return domain.Finding{
		Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning,
		Run: domain.NamedRun("20261031T073445Z-0ada"), Actor: domain.AgentInstance("coder#8"),
		Path: "logs/a/events.jsonl", Line: 42, Detail: "bad json",
	}
}

func entryKeysSorted(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func reportFindingKeys(t *testing.T, f domain.Finding) []string {
	t.Helper()
	r := domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary([]domain.Finding{f})}
	var buf bytes.Buffer
	if err := cli.EncodeReport(&buf, r); err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	var doc struct {
		Quality struct {
			Findings []map[string]any `json:"findings"`
			Counts   map[string]int   `json:"counts"`
		} `json:"quality"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil || len(doc.Quality.Findings) != 1 {
		t.Fatalf("report json: err = %v findings = %d\n%s", err, len(doc.Quality.Findings), buf.String())
	}
	return entryKeysSorted(doc.Quality.Findings[0])
}

func TestFindingsFile_EntryKeysMatchTheFindingKeysOfReportJSON(t *testing.T) {
	f := fullFinding()
	path := filepath.Join(t.TempDir(), "findings.json")

	if err := findingsfile.NewWriter().WriteFindings(context.Background(), path, []domain.Finding{f}, false); err != nil {
		t.Fatalf("WriteFindings: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || len(doc.Findings) != 1 {
		t.Fatalf("findings file: err = %v entries = %d\n%s", err, len(doc.Findings), data)
	}

	got, want := entryKeysSorted(doc.Findings[0]), reportFindingKeys(t, f)

	if len(got) != len(want) {
		t.Fatalf("export keys = %v, report keys = %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("export keys = %v, report keys = %v", got, want)
			break
		}
	}
}

func TestFindingsFile_EntryValuesMatchTheFindingOfReportJSON(t *testing.T) {
	f := fullFinding()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := findingsfile.NewWriter().WriteFindings(context.Background(), path, []domain.Finding{f}, false); err != nil {
		t.Fatalf("WriteFindings: %v", err)
	}
	data, _ := os.ReadFile(path)
	var exported struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(data, &exported); err != nil || len(exported.Findings) != 1 {
		t.Fatalf("findings file: err = %v\n%s", err, data)
	}
	var buf bytes.Buffer
	if err := cli.EncodeReport(&buf, domain.Report{Currency: domain.Currency, Quality: domain.NewQualitySummary([]domain.Finding{f})}); err != nil {
		t.Fatal(err)
	}
	var reported struct {
		Quality struct {
			Findings []map[string]any `json:"findings"`
		} `json:"quality"`
	}
	if err := json.Unmarshal(buf.Bytes(), &reported); err != nil || len(reported.Quality.Findings) != 1 {
		t.Fatalf("report json: err = %v", err)
	}

	for k, want := range reported.Quality.Findings[0] {
		if got := exported.Findings[0][k]; got != want {
			t.Errorf("%s = %v in the export, %v in report json", k, got, want)
		}
	}
}
