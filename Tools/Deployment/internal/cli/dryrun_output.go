package cli

import (
	"fmt"
	"io"

	"mosaic-deploy/internal/domain"
)

// writeDryRunReport prints the plan a dry run reviewed: the dry-run statement, the workspace,
// one line per item with its action and reason, then the removals and replacements. It reports
// whether it printed; a summary without a dry-run plan is left to the regular outcome output.
func writeDryRunReport(out io.Writer, summary domain.RunSummary) bool {
	plan := summary.DryRunPlan
	if plan == nil {
		return false
	}
	fmt.Fprintf(out, "dry run: nothing was written\nworkspace: %s\nplan:\n", summary.WorkspacePath)
	for _, item := range plan.Items {
		fmt.Fprintf(out, "  %s %s%s\n", item.Action, item.TargetPath, dryRunReason(item.Reason))
	}
	if len(plan.Removals) > 0 {
		fmt.Fprintln(out, "removals:")
		for _, r := range plan.Removals {
			fmt.Fprintf(out, "  remove %s%s\n", r.Path, dryRunReason(r.Reason))
		}
	}
	if len(plan.Replacements) > 0 {
		fmt.Fprintln(out, "replacements:")
		for _, r := range plan.Replacements {
			fmt.Fprintf(out, "  replace %s%s\n", r.Path, dryRunReason(r.Reason))
		}
	}
	return true
}

// dryRunReason is the " - <reason>" suffix of a plan line, empty when there is no reason.
func dryRunReason(reason string) string {
	if reason == "" {
		return ""
	}
	return " - " + reason
}
