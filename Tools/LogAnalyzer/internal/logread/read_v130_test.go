package logread_test

// Tests for schema 1.3.0 decoding: acceptance of the version, and the
// provider, reasoning_tokens and effort fields.
//
// Rules under test:
//   - Schema 1.3.0 raises no unknown_schema_version finding, alone or mixed
//     with older versions in one stream.
//   - provider is decoded on usage_record, invocation_end and turn; effort on
//     usage_record; reasoning_tokens wherever token_usage is decoded.
//   - A missing key decodes as absent ("" for strings, absent for reasoning);
//     reasoning_tokens of 0 is present-zero.

import (
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// v130Line builds one event line. extra is raw JSON members (with a leading
// comma) appended after the envelope, e.g. `,"model":"m","provider":"p"`.
func v130Line(version, event, extra string) string {
	return `{"schema_version":"` + version + `","event":"` + event +
		`","timestamp":"2026-10-22T10:00:01.000Z","harness":"opencode","session_id":"s1","run_id":"` +
		tierRunID + `"` + extra + `}`
}

// v130Extras returns the event-specific members for a usage block, a provider
// member (omitted when empty) and, for usage_record, an effort member.
func v130Extras(event, provider, effort, usageJSON string) string {
	var b strings.Builder
	switch event {
	case "usage_record":
		b.WriteString(`,"agent_instance_id":"a#1","record_id":"r1"`)
	case "invocation_end":
		b.WriteString(`,"agent_instance_id":"a#1","status_code":"SUCCESS"`)
	case "turn":
		b.WriteString(`,"role":"assistant"`)
	}
	b.WriteString(`,"model":"m"`)
	if provider != "" {
		b.WriteString(`,"provider":"` + provider + `"`)
	}
	if effort != "" {
		b.WriteString(`,"effort":"` + effort + `"`)
	}
	if usageJSON != "" {
		b.WriteString(`,"token_usage":` + usageJSON)
	}
	return b.String()
}

func providerOf(t *testing.T, ev domain.Event) domain.ProviderID {
	t.Helper()
	switch {
	case ev.UsageRecord != nil:
		return ev.UsageRecord.Provider
	case ev.Turn != nil:
		return ev.Turn.Provider
	case ev.InvocationEnd != nil:
		return ev.InvocationEnd.Provider
	}
	t.Fatalf("event %q carries no provider block", ev.RawType)
	return ""
}

func unknownSchemaFindings(findings []domain.Finding) int {
	n := 0
	for _, f := range findings {
		if f.Kind == domain.FindingUnknownSchema {
			n++
		}
	}
	return n
}

func TestReader_Schema130_RaisesNoFinding(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			_, findings := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "anthropic", "", `{"input_tokens":1}`)))

			if len(findings) != 0 {
				t.Errorf("findings = %v, want none for supported schema 1.3.0", findings)
			}
		})
	}
}

func TestReader_MixedSchemaVersions_RaiseNoFinding(t *testing.T) {
	_, findings := readTierLines(t,
		v130Line("1.1.0", "run_start", `,"model":"m"`),
		v130Line("1.3.0", "usage_record", v130Extras("usage_record", "anthropic", "high", `{"input_tokens":1}`)),
		v130Line("1.2.0", "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)),
	)

	if n := unknownSchemaFindings(findings); n != 0 {
		t.Errorf("unknown_schema_version findings = %d, want 0 for a stream of supported versions", n)
	}
}

func TestReader_UnknownMajorVersion_StillRaisesOneFinding(t *testing.T) {
	_, findings := readTierLines(t, v130Line("2.0.0", "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)))

	if n := unknownSchemaFindings(findings); n != 1 {
		t.Errorf("unknown_schema_version findings = %d, want 1", n)
	}
}

func TestReader_Provider_DecodedOnEveryEventCarryingModel(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, _ := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "amazon-bedrock", "", `{"input_tokens":1}`)))

			if got := providerOf(t, events[0]); got != "amazon-bedrock" {
				t.Errorf("Provider = %q, want %q", got, "amazon-bedrock")
			}
		})
	}
}

func TestReader_Provider_MissingKey_DecodesAsEmpty(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, findings := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "", "", `{"input_tokens":1}`)))

			if got := providerOf(t, events[0]); got != "" {
				t.Errorf("Provider = %q, want empty when the key is missing", got)
			}
			if len(findings) != 0 {
				t.Errorf("findings = %v, want none (a missing provider is not a defect)", findings)
			}
		})
	}
}

func TestReader_Effort_DecodedVerbatimOnUsageRecord(t *testing.T) {
	events, _ := readTierLines(t, v130Line("1.3.0", "usage_record", v130Extras("usage_record", "anthropic", "xhigh", `{"input_tokens":1}`)))

	if got := events[0].UsageRecord.Effort; got != "xhigh" {
		t.Errorf("Effort = %q, want %q verbatim", got, "xhigh")
	}
}

func TestReader_Effort_MissingKey_DecodesAsEmpty(t *testing.T) {
	events, findings := readTierLines(t, v130Line("1.3.0", "usage_record", v130Extras("usage_record", "anthropic", "", `{"input_tokens":1}`)))

	if got := events[0].UsageRecord.Effort; got != "" {
		t.Errorf("Effort = %q, want empty when the key is missing", got)
	}
	if len(findings) != 0 {
		t.Errorf("findings = %v, want none", findings)
	}
}

func TestReader_ReasoningTokens_DecodedOnEveryUsageBlock(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			usage := `{"input_tokens":10,"output_tokens":90,"reasoning_tokens":35}`
			events, findings := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "p", "", usage)))

			if len(findings) != 0 {
				t.Fatalf("findings = %v, want none", findings)
			}
			u := usageOf(t, events[0])
			requireTokens(t, "Reasoning", u.Reasoning, 35)
			requireTokens(t, "Output", u.Output, 90)
		})
	}
}

func TestReader_ReasoningTokens_ZeroIsPresentZero(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, _ := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "p", "", `{"output_tokens":9,"reasoning_tokens":0}`)))

			requireTokens(t, "Reasoning", usageOf(t, events[0]).Reasoning, 0)
		})
	}
}

func TestReader_ReasoningTokens_MissingKey_StaysAbsent(t *testing.T) {
	for _, event := range tierEventTypes {
		t.Run(event, func(t *testing.T) {
			events, findings := readTierLines(t, v130Line("1.3.0", event, v130Extras(event, "p", "", `{"output_tokens":9}`)))

			requireAbsentTokens(t, "Reasoning", usageOf(t, events[0]).Reasoning)
			if len(findings) != 0 {
				t.Errorf("findings = %v, want none", findings)
			}
		})
	}
}

func TestReader_ReasoningTokens_DoesNotDisturbCacheTierSplit(t *testing.T) {
	usage := `{"cache_creation_tokens":100,"cache_creation_5m_tokens":60,"cache_creation_1h_tokens":40,"reasoning_tokens":3}`
	events, findings := readTierLines(t, v130Line("1.3.0", "usage_record", v130Extras("usage_record", "p", "", usage)))

	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	u := usageOf(t, events[0])
	requireTokens(t, "CacheCreation5m", u.CacheCreation5m, 60)
	requireTokens(t, "Reasoning", u.Reasoning, 3)
}

func TestReader_UnsupportedVersionFinding_DetailNamesOffendingVersionAndReadMajor(t *testing.T) {
	_, findings := readTierLines(t, v130Line("2.0.0", "turn", v130Extras("turn", "", "", `{"input_tokens":1}`)))

	for _, f := range findings {
		if f.Kind == domain.FindingUnknownSchema {
			if !strings.Contains(f.Detail, "2.0.0") {
				t.Errorf("finding detail %q should name the offending version 2.0.0", f.Detail)
			}
			if !strings.Contains(f.Detail, "1.x.y") {
				t.Errorf("finding detail %q should state that major version 1 (1.x.y) is read", f.Detail)
			}
			return
		}
	}
	t.Fatal("expected an unknown_schema_version finding for 2.0.0")
}
