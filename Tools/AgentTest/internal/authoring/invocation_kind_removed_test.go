package authoring_test

// The subject.invocation_kind key is retired: every subject is launched by
// agent name, so the key never had an effect. A test definition that still
// declares it is rejected with the specific "removed-key-subject-invocation-kind"
// diagnostic (not the generic unknown-field one), and the same definition
// without the key parses cleanly.

import (
	"strings"
	"testing"

	"mosaic-agent-test/internal/authoring"
)

const invocationKindRemovedCode = "removed-key-subject-invocation-kind"

const subjectWithoutInvocationKindYAML = `
schema_version: 1
name: d
id: 1
version: 1
changelog:
  - version: 1
    date: "2026-01-01"
    changes: "Initial."
layer: orchestrator
subject:
  identity: orchestrator
  agent: orchestrator
  infrastructure_agents: []
  opening_message: "Build the thing."
stub_registry: stubs/d.stubs.json
`

// subjectWithInvocationKindYAML is the same definition plus the retired key.
func subjectWithInvocationKindYAML(value string) string {
	return strings.Replace(subjectWithoutInvocationKindYAML,
		"  opening_message: \"Build the thing.\"\n",
		"  opening_message: \"Build the thing.\"\n  invocation_kind: "+value+"\n", 1)
}

func TestParseTestDefinition_InvocationKindKey_RejectedWithRemovedKeyDiagnostic(t *testing.T) {
	for _, value := range []string{"orchestrator", "subagent", `""`} {
		t.Run("value="+value, func(t *testing.T) {
			src := authoring.Source{Path: "d.test.yaml", Data: []byte(subjectWithInvocationKindYAML(value))}

			_, report := authoring.ParseTestDefinition(src)

			if !report.HasErrors() {
				t.Fatal("expected a definition declaring subject.invocation_kind to be rejected")
			}
			d, ok := diagnosticWithCode(report, invocationKindRemovedCode)
			if !ok {
				t.Fatalf("expected a diagnostic with code %q, got: %v", invocationKindRemovedCode, report.Diagnostics)
			}
			if d.Pointer != "subject.invocation_kind" {
				t.Errorf("diagnostic Pointer = %q, want %q", d.Pointer, "subject.invocation_kind")
			}
			if d.Severity != authoring.SeverityError {
				t.Errorf("diagnostic Severity = %v, want an error", d.Severity)
			}
			if !strings.Contains(d.Message, "invocation_kind") {
				t.Errorf("diagnostic Message %q must name the key invocation_kind", d.Message)
			}
			if !strings.Contains(strings.ToLower(d.Message), "no longer") {
				t.Errorf("diagnostic Message %q must say the key is no longer used", d.Message)
			}
		})
	}
}

// TestParseTestDefinition_InvocationKindKey_NotAlsoReportedAsUnknownField pins
// that the specific diagnostic replaces the generic one for this key.
func TestParseTestDefinition_InvocationKindKey_NotAlsoReportedAsUnknownField(t *testing.T) {
	src := authoring.Source{Path: "d.test.yaml", Data: []byte(subjectWithInvocationKindYAML("orchestrator"))}

	_, report := authoring.ParseTestDefinition(src)

	if !hasDiagnosticCode(report, invocationKindRemovedCode) {
		t.Fatalf("expected a diagnostic with code %q, got: %v", invocationKindRemovedCode, report.Diagnostics)
	}
	count := 0
	for _, d := range report.Diagnostics {
		if d.Pointer == "subject.invocation_kind" {
			count++
			if d.Code == "unknown-field" {
				t.Errorf("the retired key must not also be reported as unknown-field: %+v", d)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one diagnostic for subject.invocation_kind, got %d: %v", count, report.Diagnostics)
	}
}

func TestParseTestDefinition_NoInvocationKindKey_ParsesWithoutDiagnosticAboutIt(t *testing.T) {
	src := authoring.Source{Path: "d.test.yaml", Data: []byte(subjectWithoutInvocationKindYAML)}

	_, report := authoring.ParseTestDefinition(src)

	if report.HasErrors() {
		t.Fatalf("expected a definition without subject.invocation_kind to parse cleanly, got: %v", report.Diagnostics)
	}
	for _, d := range report.Diagnostics {
		if d.Pointer == "subject.invocation_kind" || d.Code == invocationKindRemovedCode {
			t.Errorf("unexpected diagnostic about invocation_kind: %+v", d)
		}
	}
}
