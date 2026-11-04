package cost_test

// The log-analysis tool's total payload may carry additive fields for a run
// with unattributed usage (an "unattributed" object next to the committed
// ones). The cost provider must keep reading the committed fields unchanged:
// the money total already includes the unattributed share and is reported in
// full, as an attributed cost.

import (
	"context"
	"testing"

	"mosaic-agent-test/internal/cost"
	"mosaic-agent-test/internal/domain"
)

const totalWithUnattributed = `{
  "schema_version": "1",
  "run_id": "run-unattr",
  "provisional": false,
  "currency": "USD",
  "tokens": {"input": 6310, "cache_read": 4000, "cache_creation": 700, "output": 620},
  "money": {"state": "known", "amount": "10.21"},
  "complete": true,
  "unattributed": {
    "tokens": {"input": 1000, "cache_read": null, "cache_creation": null, "output": 100},
    "money": {"state": "known", "amount": "1.50"}
  }
}`

func TestCost_TotalWithAdditiveUnattributedField_ReportsTheFullTotalAsAttributed(t *testing.T) {
	provider := cost.New(cost.Options{
		ExecutablePath: "mosaic-log-analyzer",
		Invoke: func(ctx context.Context, path string, args []string, workingDir string) ([]byte, int, error) {
			return []byte(totalWithUnattributed), exitSuccess, nil
		},
	})

	report, err := provider.Cost(context.Background(), domain.CostQuery{LogRoot: "/sandbox/logs", RunID: "run-unattr"})

	if err != nil {
		t.Fatalf("Cost returned unexpected error: %v", err)
	}
	if report.Attribution != domain.AttributionAttributed {
		t.Errorf("Attribution = %q, want %q: the additive field must not change how the total is read", report.Attribution, domain.AttributionAttributed)
	}
	if report.TotalUSD != 10.21 {
		t.Errorf("TotalUSD = %v, want 10.21: the run total already includes the unattributed share", report.TotalUSD)
	}
}
