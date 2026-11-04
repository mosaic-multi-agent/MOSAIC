package resultstore_test

// Shared builders for reports whose runs carry observed subject models. Used
// by the refusal and observed-model filing tests.

import (
	"encoding/json"
	"testing"
	"time"
)

const (
	selectedModel = "github-copilot/claude-sonnet-4.6"
	otherModel    = "github-copilot/gpt-5"
)

// runSpec describes one repetition inside a built report.
type runSpec struct {
	selected     string   // subject_model
	observed     []string // observed_subject_models (ignored when omitObserved)
	omitObserved bool     // leave the observed field out entirely
}

// verifiedRun is a repetition whose observed model equals the selected model.
func verifiedRun() runSpec {
	return runSpec{selected: selectedModel, observed: []string{selectedModel}}
}

// buildReport returns report JSON with one test per argument, each holding the
// given repetitions. Version, suite, harness and start time are fixed so that
// filing paths are predictable.
func buildReport(t *testing.T, tests ...[]runSpec) []byte {
	t.Helper()
	var wireTests []any
	for i, runs := range tests {
		var wireRuns []any
		for _, r := range runs {
			run := map[string]any{
				"subject_version":    "v3.0.0",
				"subject_model":      r.selected,
				"harness_id":         "claude-code",
				"verdict":            "pass",
				"duration_ms":        1000,
				"cost":               map[string]any{"total_usd": 0.01, "attribution": "direct"},
				"termination_reason": "completed",
				"conditions":         []any{},
				"test_version":       1,
			}
			if !r.omitObserved {
				run["observed_subject_models"] = nonNil(r.observed)
				run["model_verification"] = "verified"
			}
			wireRuns = append(wireRuns, run)
		}
		wireTests = append(wireTests, map[string]any{
			"test_name":   "observed-test",
			"test_id":     i + 1,
			"description": "fixture test",
			"layer":       "orchestrator",
			"aggregate": map[string]any{
				"verdict": "pass", "counted": len(runs), "passed": len(runs), "pass_rate": 1.0,
				"infrastructure_failure": false,
				"total_cost":             map[string]any{"total_usd": 0.01, "attribution": "direct"},
			},
			"runs": wireRuns,
		})
	}
	report := map[string]any{
		"schema_version":          "1",
		"suite_id":                "observed-suite",
		"started_at":              time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"finished_at":             time.Date(2026, 10, 1, 12, 5, 0, 0, time.UTC).Format(time.RFC3339),
		"tests":                   wireTests,
		"counts":                  map[string]int{"pass": 1},
		"total_cost":              map[string]any{"total_usd": 0.01, "attribution": "direct"},
		"infrastructure_failures": 0,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("buildReport: %v", err)
	}
	return data
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
