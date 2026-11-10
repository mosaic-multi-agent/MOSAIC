package report_test

import (
	"encoding/json"
	"testing"
)

// TestRenderText_ShowsNoCost asserts the terminal report carries no cost
// line or currency amount for a result whose runs all have costs.
func TestRenderText_ShowsNoCost(t *testing.T) {
	// Arrange
	result := fixtureFullResult()

	// Act
	text, err := renderText(t, result)

	// Assert
	if err != nil {
		t.Fatalf("RenderText returned an error: %v", err)
	}
	assertNoCostInText(t, text)
}

// TestRenderJSON_KeepsCostFields asserts the machine-readable report still
// carries the suite total cost and every run's cost unchanged.
func TestRenderJSON_KeepsCostFields(t *testing.T) {
	// Arrange
	result := fixtureFullResult()

	// Act
	out, err := renderJSON(t, result)

	// Assert
	if err != nil {
		t.Fatalf("RenderJSON returned an error: %v", err)
	}
	var doc struct {
		TotalCost *struct {
			TotalUSD float64 `json:"total_usd"`
		} `json:"total_cost"`
		Tests []struct {
			Runs []struct {
				Cost *struct {
					TotalUSD float64 `json:"total_usd"`
				} `json:"cost"`
			} `json:"runs"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decoding machine-readable report: %v", err)
	}
	if doc.TotalCost == nil || doc.TotalCost.TotalUSD != result.TotalCost.TotalUSD {
		t.Errorf("total_cost = %+v, want total_usd %v", doc.TotalCost, result.TotalCost.TotalUSD)
	}
	runs := 0
	for _, tc := range doc.Tests {
		for _, r := range tc.Runs {
			runs++
			if r.Cost == nil {
				t.Errorf("a run lost its cost field")
			}
		}
	}
	if runs == 0 {
		t.Fatal("fixture produced no runs to check")
	}
}
