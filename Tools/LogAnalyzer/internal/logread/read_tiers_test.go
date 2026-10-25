package logread_test

// Tests for decoding the cache-write tier sub-fields (cache_creation_5m_tokens,
// cache_creation_1h_tokens) of token_usage, and for schema version 1.2.0
// acceptance.
//
// Rules under test:
//   - Neither tier key present: valid, tier fields absent, no finding.
//   - Both tier keys present, cache_creation_tokens present, 5m + 1h equals the
//     total: the split is decoded on usage_record, turn and invocation_end.
//   - Any other combination: both tier fields stay absent, the four categories
//     still decode, and exactly one invalid_cache_tier_split warning is raised
//     for that line. The reader never invents a tier.
//   - Schema 1.2.0 raises no unknown_schema_version finding; 1.1.0 and 1.0.0
//     still do not either.

import (
	"context"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
	"mosaic-log-analyzer/internal/logread"
)

const tierRunID = "20261001T100000Z-ab01"

// tierLine builds one JSONL event line with the given schema version, event type
// and raw token_usage JSON object.
func tierLine(version, event, usageJSON string) string {
	extra := ""
	switch event {
	case "usage_record":
		extra = `,"agent_instance_id":"a#1","record_id":"r1","model":"m"`
	case "invocation_end":
		extra = `,"agent_instance_id":"a#1","status_code":"SUCCESS","model":"m"`
	case "turn":
		extra = `,"role":"assistant","model":"m"`
	}
	return `{"schema_version":"` + version + `","event":"` + event +
		`","timestamp":"2026-10-01T10:00:01.000Z","harness":"claude-code","session_id":"s1","run_id":"` +
		tierRunID + `"` + extra + `,"token_usage":` + usageJSON + `}`
}

// readTierLines reads the given lines and returns events and findings.
func readTierLines(t *testing.T, lines ...string) ([]domain.Event, []domain.Finding) {
	t.Helper()
	path := writeTempFile(t, strings.Join(lines, "\n")+"\n")
	events, findings, err := logread.NewReader().ReadAll(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return events, findings
}

// usageOf returns the decoded usage of a usage_record, turn or invocation_end event.
func usageOf(t *testing.T, ev domain.Event) domain.TokenUsage {
	t.Helper()
	switch {
	case ev.UsageRecord != nil:
		return ev.UsageRecord.Usage
	case ev.Turn != nil:
		return ev.Turn.Usage
	case ev.InvocationEnd != nil:
		return ev.InvocationEnd.Usage
	}
	t.Fatalf("event %q carries no usage block", ev.RawType)
	return domain.TokenUsage{}
}

func requireTokens(t *testing.T, name string, tc domain.TokenCount, want int64) {
	t.Helper()
	v, ok := tc.Value()
	if !ok || v != want {
		t.Errorf("%s = (%d, present=%v), want present %d", name, v, ok, want)
	}
}

func requireAbsentTokens(t *testing.T, name string, tc domain.TokenCount) {
	t.Helper()
	if tc.IsPresent() {
		v, _ := tc.Value()
		t.Errorf("%s = %d, want absent", name, v)
	}
}

func tierFindings(findings []domain.Finding) []domain.Finding {
	var out []domain.Finding
	for _, f := range findings {
		if f.Kind == domain.FindingInvalidCacheTierSplit {
			out = append(out, f)
		}
	}
	return out
}

var tierEventTypes = []string{"usage_record", "turn", "invocation_end"}

const validSplitUsage = `{"input_tokens":10,"output_tokens":20,"cache_read_tokens":30,` +
	`"cache_creation_tokens":150,"cache_creation_5m_tokens":100,"cache_creation_1h_tokens":50}`

func TestReader_TierSplit_BothPresentAndSumming_DecodedOnEveryUsageEvent(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, findings := readTierLines(t, tierLine("1.2.0", event, validSplitUsage))

			if len(findings) != 0 {
				t.Fatalf("findings = %v, want none", findings)
			}
			u := usageOf(t, events[0])
			requireTokens(t, "CacheCreation5m", u.CacheCreation5m, 100)
			requireTokens(t, "CacheCreation1h", u.CacheCreation1h, 50)
			requireTokens(t, "CacheCreation (total)", u.CacheCreation, 150)
			requireTokens(t, "Input", u.Input, 10)
			requireTokens(t, "Output", u.Output, 20)
			requireTokens(t, "CacheRead", u.CacheRead, 30)
		})
	}
}

func TestReader_TierSplit_BothZeroWithZeroTotal_DecodedAsPresentZero(t *testing.T) {
	usage := `{"cache_creation_tokens":0,"cache_creation_5m_tokens":0,"cache_creation_1h_tokens":0}`

	events, findings := readTierLines(t, tierLine("1.2.0", "usage_record", usage))

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	u := usageOf(t, events[0])
	requireTokens(t, "CacheCreation5m", u.CacheCreation5m, 0)
	requireTokens(t, "CacheCreation1h", u.CacheCreation1h, 0)
}

func TestReader_TierSplit_OneTierZeroOtherEqualsTotal_Decoded(t *testing.T) {
	usage := `{"cache_creation_tokens":80,"cache_creation_5m_tokens":80,"cache_creation_1h_tokens":0}`

	events, findings := readTierLines(t, tierLine("1.2.0", "turn", usage))

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	u := usageOf(t, events[0])
	requireTokens(t, "CacheCreation5m", u.CacheCreation5m, 80)
	requireTokens(t, "CacheCreation1h", u.CacheCreation1h, 0)
}

func TestReader_TierSplit_NeitherPresent_AbsentAndNoFinding(t *testing.T) {
	usage := `{"input_tokens":10,"output_tokens":20,"cache_read_tokens":30,"cache_creation_tokens":150}`
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, findings := readTierLines(t, tierLine("1.2.0", event, usage))

			if len(findings) != 0 {
				t.Fatalf("findings = %v, want none", findings)
			}
			u := usageOf(t, events[0])
			requireAbsentTokens(t, "CacheCreation5m", u.CacheCreation5m)
			requireAbsentTokens(t, "CacheCreation1h", u.CacheCreation1h)
			requireTokens(t, "CacheCreation (total)", u.CacheCreation, 150)
		})
	}
}

func TestReader_TierSplit_Invalid_DropsSplitKeepsCategoriesAndWarnsOnce(t *testing.T) {
	cases := []struct {
		name       string
		usage      string
		wantDetail string
	}{
		{"only 5m present",
			`{"input_tokens":10,"cache_creation_tokens":150,"cache_creation_5m_tokens":150}`,
			"one tier field missing"},
		{"only 1h present",
			`{"input_tokens":10,"cache_creation_tokens":150,"cache_creation_1h_tokens":150}`,
			"one tier field missing"},
		{"tiers do not sum to total",
			`{"input_tokens":10,"cache_creation_tokens":150,"cache_creation_5m_tokens":100,"cache_creation_1h_tokens":60}`,
			"do not sum"},
		{"total missing",
			`{"input_tokens":10,"cache_creation_5m_tokens":100,"cache_creation_1h_tokens":50}`,
			"total missing"},
		{"total missing and one tier missing reports total first",
			`{"input_tokens":10,"cache_creation_5m_tokens":100}`,
			"total missing"},
	}
	for _, event := range tierEventTypes {
		for _, c := range cases {
			t.Run(event+"/"+c.name, func(t *testing.T) {
				events, findings := readTierLines(t, tierLine("1.2.0", event, c.usage))

				tf := tierFindings(findings)
				if len(tf) != 1 || len(findings) != 1 {
					t.Fatalf("findings = %v, want exactly one invalid_cache_tier_split", findings)
				}
				if tf[0].Severity != domain.SeverityWarning {
					t.Errorf("Severity = %v, want warning", tf[0].Severity)
				}
				if tf[0].Line != 1 || tf[0].Path == "" {
					t.Errorf("finding location = %q:%d, want a path and line 1", tf[0].Path, tf[0].Line)
				}
				if !strings.Contains(tf[0].Detail, c.wantDetail) {
					t.Errorf("Detail = %q, want it to contain %q", tf[0].Detail, c.wantDetail)
				}
				u := usageOf(t, events[0])
				requireAbsentTokens(t, "CacheCreation5m", u.CacheCreation5m)
				requireAbsentTokens(t, "CacheCreation1h", u.CacheCreation1h)
				requireTokens(t, "Input", u.Input, 10)
			})
		}
	}
}

func TestReader_TierSplit_InvalidLines_OneFindingPerLineWithOwnLineNumber(t *testing.T) {
	bad := `{"cache_creation_tokens":10,"cache_creation_5m_tokens":10}`

	_, findings := readTierLines(t,
		tierLine("1.2.0", "usage_record", bad),
		tierLine("1.2.0", "usage_record", validSplitUsage),
		tierLine("1.2.0", "turn", bad),
	)

	tf := tierFindings(findings)
	if len(tf) != 2 || tf[0].Line != 1 || tf[1].Line != 3 {
		t.Fatalf("tier findings = %v, want two, on lines 1 and 3", tf)
	}
}

func TestReader_SchemaVersions_AllSupportedVersionsRaiseNoUnknownSchemaFinding(t *testing.T) {
	for _, version := range []string{"1.2.0", "1.1.0", "1.0.0"} {
		t.Run(version, func(t *testing.T) {
			_, findings := readTierLines(t, tierLine(version, "turn", `{"input_tokens":1}`))

			for _, f := range findings {
				if f.Kind == domain.FindingUnknownSchema {
					t.Errorf("version %s raised unknown_schema_version: %v", version, f)
				}
			}
		})
	}
}

func TestReader_SchemaVersions_OlderVersionsDecodeTotalsWithoutSplit(t *testing.T) {
	usage := `{"input_tokens":10,"cache_creation_tokens":150}`
	for _, version := range []string{"1.1.0", "1.0.0"} {
		t.Run(version, func(t *testing.T) {
			events, findings := readTierLines(t, tierLine(version, "invocation_end", usage))

			if len(findings) != 0 {
				t.Fatalf("findings = %v, want none", findings)
			}
			u := usageOf(t, events[0])
			requireTokens(t, "CacheCreation", u.CacheCreation, 150)
			requireAbsentTokens(t, "CacheCreation5m", u.CacheCreation5m)
			requireAbsentTokens(t, "CacheCreation1h", u.CacheCreation1h)
		})
	}
}

func TestReader_SchemaVersions_UnsupportedNewerVersionStillRaisesFinding(t *testing.T) {
	_, findings := readTierLines(t, tierLine("1.4.0", "turn", `{"input_tokens":1}`))

	found := false
	for _, f := range findings {
		if f.Kind == domain.FindingUnknownSchema {
			found = true
		}
	}
	if !found {
		t.Errorf("findings = %v, want unknown_schema_version for 1.4.0", findings)
	}
}
