package opencodev2

import (
	"context"

	"mosaic-agent-test/internal/domain"
)

// CheckEnvironment implements domain.HarnessAdapter. The harness runs no
// interpreter, so none is declared. The check is scope-based only: it reports
// any non-sandbox scope that could rewrite the intercepted call's input. A
// missing executable is reported by the launcher at spawn.
func (a *Adapter) CheckEnvironment(ctx context.Context) (domain.EnvironmentReport, error) {
	var report domain.EnvironmentReport

	findings, err := a.InspectScopes(ctx)
	if err != nil {
		return report, err
	}
	report.Scopes = findings
	for _, f := range findings {
		if f.RewritesInput && !f.Neutralized {
			report.Problems = append(report.Problems, domain.EnvironmentProblem{
				Kind:   domain.ProblemCompetingRewriter,
				Detail: f.Detail,
			})
		}
	}
	return report, nil
}
