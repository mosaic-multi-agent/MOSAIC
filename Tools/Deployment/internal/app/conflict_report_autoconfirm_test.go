package app_test

// conflict_report_autoconfirm_test.go verifies the non-interactive behaviour of the conflict
// report: with auto-confirm, each warning line is recorded as one catalog-shadowing gap in the
// run's TODO collector, info lines record nothing, a run without auto-confirm records nothing,
// and the run always completes with a successful outcome.

import (
	"context"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// crShadowGaps returns the catalog-shadowing gaps the spy collector recorded.
func crShadowGaps(spy *spyTodo) []domain.Gap {
	var out []domain.Gap
	for _, g := range spy.gaps {
		if g.Kind == domain.GapCatalogShadowing {
			out = append(out, g)
		}
	}
	return out
}

// TestAutoConfirm_UndeclaredOverride_RecordsOneGapAndCompletes verifies that an auto-confirmed
// run of each write operation records one gap per warning line, with the line's subject and
// detail, and still ends in a successful outcome.
func TestAutoConfirm_UndeclaredOverride_RecordsOneGapAndCompletes(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
			crWriteDeployed(t, ws, "orchestrator.md", "quick-fix")
			spy := &spyTodo{}
			deps.Todo = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			gaps := crShadowGaps(spy)
			if len(gaps) != 1 {
				t.Fatalf("catalog-shadowing gaps = %+v, want exactly one for the one warning line", gaps)
			}
			g := gaps[0]
			if g.Subject != "agent:"+crAgentKey {
				t.Errorf("Subject = %q, want %q", g.Subject, "agent:"+crAgentKey)
			}
			if g.Owner != "" || g.Fragment != "" {
				t.Errorf("Owner = %q, Fragment = %q, want both empty (run-level gap)", g.Owner, g.Fragment)
			}
			for _, frag := range []string{
				"from " + crUserID + " 1.0.0 shadows " + crDefaultID + " 1.0.0",
				"affects workflows: quick-fix",
				"undeclared override",
			} {
				if !strings.Contains(g.Detail, frag) {
					t.Errorf("Detail %q lacks %q", g.Detail, frag)
				}
			}
			if summary.Outcome != domain.OutcomeSuccess {
				t.Errorf("Outcome = %q, want %q: a conflict warning never changes the outcome", summary.Outcome, domain.OutcomeSuccess)
			}
		})
	}
}

// TestAutoConfirm_DeclaredOverride_RecordsNoGap verifies that an info line (an override the
// winner declared) adds no gap.
func TestAutoConfirm_DeclaredOverride_RecordsNoGap(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{declareAgent: true})
			spy := &spyTodo{}
			deps.Todo = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			crLineFor(t, summary.ConflictReport, crAgentItem()) // the line is reported
			if gaps := crShadowGaps(spy); len(gaps) != 0 {
				t.Errorf("catalog-shadowing gaps = %+v, want none for a declared override", gaps)
			}
		})
	}
}

// TestAutoConfirm_NotSet_RecordsNoGap verifies that a run the user confirmed interactively
// records no catalog-shadowing gap: the warning is shown through the report only.
func TestAutoConfirm_NotSet_RecordsNoGap(t *testing.T) {
	for _, op := range crOperations() {
		t.Run(op.name, func(t *testing.T) {
			deps, ws := crDeps(t, interactiontest.NewBuilder().AnswerReview(true).Build(), crStackOpts{})
			spy := &spyTodo{}
			deps.Todo = spy

			summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{})

			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
			crLineFor(t, summary.ConflictReport, crAgentItem())
			if gaps := crShadowGaps(spy); len(gaps) != 0 {
				t.Errorf("catalog-shadowing gaps = %+v, want none without auto-confirm", gaps)
			}
		})
	}
}

// TestAutoConfirm_TwoWarnings_GapsFollowReportOrder verifies one gap per warning line in report
// order: the orchestrator pair line comes before the agent line, and the pair's subject is the
// bare kind.
func TestAutoConfirm_TwoWarnings_GapsFollowReportOrder(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{orchestratorPair: true})
	spy := &spyTodo{}
	deps.Todo = spy
	op := crOperations()[0] // deploy

	_, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true})

	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	gaps := crShadowGaps(spy)
	if len(gaps) != 2 {
		t.Fatalf("catalog-shadowing gaps = %+v, want two", gaps)
	}
	if gaps[0].Subject != "orchestrator-pair" || gaps[1].Subject != "agent:"+crAgentKey {
		t.Errorf("subjects = %q, %q; want %q then %q", gaps[0].Subject, gaps[1].Subject, "orchestrator-pair", "agent:"+crAgentKey)
	}
}

// TestAutoConfirm_DryRunDeploy_StillRecordsGapAndCompletes verifies that a dry run with
// auto-confirm reports and records the warning and ends in a successful outcome.
func TestAutoConfirm_DryRunDeploy_StillRecordsGapAndCompletes(t *testing.T) {
	deps, ws := crDeps(t, interactiontest.NewBuilder().Build(), crStackOpts{})
	spy := &spyTodo{}
	deps.Todo = spy
	op := crOperations()[0] // deploy

	summary, err := op.run(context.Background(), app.New(deps), ws, crFlags{autoConfirm: true, dryRun: true})

	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if len(crShadowGaps(spy)) != 1 {
		t.Errorf("catalog-shadowing gaps = %+v, want one", crShadowGaps(spy))
	}
	if summary.Outcome != domain.OutcomeSuccess {
		t.Errorf("Outcome = %q, want %q", summary.Outcome, domain.OutcomeSuccess)
	}
}
