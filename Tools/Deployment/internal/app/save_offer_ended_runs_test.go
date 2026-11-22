package app_test

// save_offer_ended_runs_test.go specifies the save offer in runs that end without writing: a dry
// run, a plan the user declines or cancels at review, and a transform cancelled by a later
// question. The offer is still made, an accepted save stays in the user config with its notice,
// and a later call on the same service does not ask for the saved generic.

import (
	"context"
	"errors"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// reviewOverride answers the plan review with a fixed answer when one is set; every other call
// goes to the wrapped interaction.
type reviewOverride struct {
	domain.Interaction
	answer *domain.ConfirmAnswer
}

func (o *reviewOverride) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	if o.answer != nil {
		return *o.answer, nil
	}
	return o.Interaction.Review(ctx, p)
}

var (
	reviewDeclined  = &domain.ConfirmAnswer{Status: domain.Answered, Confirm: false}
	reviewCancelled = &domain.ConfirmAnswer{Status: domain.Cancelled}
)

// endMode is how a run ends. The zero value is a confirmed run.
type endMode struct {
	name    string
	dryRun  bool
	review  *domain.ConfirmAnswer // nil: the scripted review (confirmed)
	wantErr error                 // the error the run returns
}

// autoConfirm reports whether the request confirms the plan up front (only a confirmed run does).
func (m endMode) autoConfirm() bool { return !m.dryRun && m.review == nil }

var endModes = []endMode{
	{name: "dry run", dryRun: true},
	{name: "plan declined", review: reviewDeclined, wantErr: app.ErrPlanNotConfirmed},
	{name: "plan cancelled", review: reviewCancelled, wantErr: app.ErrPlanNotConfirmed},
}

// endFlows are the deploy-family flows driven on a service with the given ending.
var endFlows = []struct {
	name string
	run  func(svc app.Service, dir string, m endMode) error
}{
	{"deploy", func(svc app.Service, dir string, m endMode) error {
		_, err := svc.DeployNew(context.Background(), app.DeployRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, Scope: domain.ScopeProject,
			WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels,
			AutoConfirmPlan: m.autoConfirm(), DryRun: m.dryRun,
		})
		return err
	}},
	{"deploy agents", func(svc app.Service, dir string, m endMode) error {
		_, err := svc.DeployAgents(context.Background(), app.DeployAgentsRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, Scope: domain.ScopeProject,
			SubagentIDs: []string{"test-runner", "second-runner"}, UtilityAgentIDs: []string{},
			InfrastructureAgentIDs: []string{}, StandaloneAgentIDs: []string{},
			SkipAll: listSkipModels, AutoConfirmPlan: m.autoConfirm(), DryRun: m.dryRun,
		})
		return err
	}},
	{"change workflows", func(svc app.Service, dir string, m endMode) error {
		_, err := svc.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: offerHarnessID, WorkspacePath: dir, WorkflowIDs: []string{"quick-fix"},
			SkipAll: listSkipModels, AutoConfirmPlan: m.autoConfirm(), DryRun: m.dryRun,
		})
		return err
	}},
}

// newEndedRun is an offer run whose plan review answers with the mode's review and whose registry
// is rebuilt from the saved entries (as in a TUI process that returns to the mode screen).
func newEndedRun(t *testing.T, m endMode, o offerSetup) (*offerRun, *rebuildState, *reviewOverride, app.Service) {
	t.Helper()
	r := newOfferRun(t, listFlowAnswer, o)
	override := &reviewOverride{Interaction: r.deps.Interaction, answer: m.review}
	r.deps.Interaction = override
	state := &rebuildState{}
	r.deps.Registry = newRebuildRegistry(state, r.buildModules)
	return r, state, override, app.New(r.deps)
}

func TestSaveOffer_EndedRuns_Deploy_OfferIsMadeAndAcceptedSaveIsKept(t *testing.T) {
	for _, flow := range endFlows {
		for _, m := range endModes {
			t.Run(flow.name+"/"+m.name, func(t *testing.T) {
				r, _, _, svc := newEndedRun(t, m, offerSetup{})

				err := flow.run(svc, r.workDir, m)

				if !errors.Is(err, m.wantErr) || (m.wantErr == nil && err != nil) {
					t.Fatalf("run error = %v, want %v", err, m.wantErr)
				}
				if len(r.offer.offers) != 1 {
					t.Fatalf("offers = %d, want the offer to be made once", len(r.offer.offers))
				}
				if len(r.user.puts) != 1 {
					t.Fatalf("saved %d entries, want the accepted one to stay: %+v", len(r.user.puts), r.user.puts)
				}
				got := r.user.puts[0]
				if got.harnessID != offerHarnessID || !equalMapping(got.mapping, offerMapping("bash", offerListNames...)) {
					t.Errorf("saved %q %+v, want the answer for %q", got.harnessID, got.mapping, offerHarnessID)
				}
				assertSaveAnnounced(t, r.notices(), "bash", offerHarnessID)
			})
		}
	}
}

func assertSaveAnnounced(t *testing.T, notices []domain.Notice, words ...string) {
	t.Helper()
	for _, n := range noticesAt(notices, domain.NoticeInfo) {
		if noticeMentions(n, words...) {
			return
		}
	}
	t.Errorf("no info notice names %v: %+v", words, notices)
}

func TestSaveOffer_EndedRuns_Deploy_DeclinedOfferSavesNothing(t *testing.T) {
	for _, flow := range endFlows {
		for _, m := range endModes {
			t.Run(flow.name+"/"+m.name, func(t *testing.T) {
				r, _, _, svc := newEndedRun(t, m, offerSetup{answer: declineOffer})

				_ = flow.run(svc, r.workDir, m)

				if len(r.offer.offers) != 1 {
					t.Fatalf("offers = %d, want one", len(r.offer.offers))
				}
				if len(r.user.puts) != 0 {
					t.Errorf("saved %+v although the offer was declined", r.user.puts)
				}
			})
		}
	}
}

func TestSaveOffer_EndedRuns_Deploy_LaterCallOnTheSameServiceDoesNotAskAgain(t *testing.T) {
	for _, flow := range endFlows {
		for _, m := range endModes {
			t.Run(flow.name+"/"+m.name, func(t *testing.T) {
				r, state, override, svc := newEndedRun(t, m, offerSetup{})
				_ = flow.run(svc, r.workDir, m)
				override.answer = nil // the user confirms the next plan

				err := flow.run(svc, r.workDir, endMode{})

				if err != nil {
					t.Fatalf("later run: %v", err)
				}
				if got := r.spy.Stub.CountAsked(domain.QCustomTool, "bash"); got != 1 {
					t.Errorf("custom tool question asked %d times over two calls, want once", got)
				}
				if len(r.offer.offers) != 1 {
					t.Errorf("offers = %d, want the one of the first call", len(r.offer.offers))
				}
				if state.rebuilds != 1 {
					t.Errorf("registry rebuilt %d times, want once after the save", state.rebuilds)
				}
			})
		}
	}
}

// cancelSecondAsk answers the first custom tool question with the list answer and cancels the
// second, as a user who aborts a later question of the transform.
type cancelSecondAsk struct {
	domain.Interaction
	asked int
}

func (c *cancelSecondAsk) AskText(ctx context.Context, q domain.TextQuestion) (domain.TextAnswer, error) {
	if q.ID != domain.QCustomTool {
		return c.Interaction.AskText(ctx, q)
	}
	c.asked++
	if c.asked == 1 {
		return domain.TextAnswer{Status: domain.Answered, Text: listFlowAnswer}, nil
	}
	return domain.TextAnswer{Status: domain.Cancelled}, nil
}

// trEnded is a transform offer run whose plan review and later questions can end the run.
func trEnded(t *testing.T, review *domain.ConfirmAnswer, extra ...func(*app.Deps)) *trOffer {
	t.Helper()
	wrap := func(d *app.Deps) { d.Interaction = &reviewOverride{Interaction: d.Interaction, answer: review} }
	return newTrOffer(t, listFlowAnswer, offerSetup{}, append([]func(*app.Deps){wrap}, extra...)...)
}

func TestSaveOffer_EndedRuns_Transform_AcceptedSaveIsKeptWhenThePlanIsNotConfirmed(t *testing.T) {
	cases := map[string]*domain.ConfirmAnswer{"declined": reviewDeclined, "cancelled": reviewCancelled}
	for name, review := range cases {
		t.Run(name, func(t *testing.T) {
			r := trEnded(t, review)
			req := r.request()
			req.AutoConfirmPlan = false

			_, err := r.run(req)

			if !errors.Is(err, app.ErrPlanNotConfirmed) {
				t.Fatalf("error = %v, want ErrPlanNotConfirmed", err)
			}
			if len(r.offer.offers) != 1 || len(r.user.puts) != 1 || r.user.puts[0].harnessID != hOpen {
				t.Fatalf("offers=%d puts=%+v, want one offer and one entry saved for %s",
					len(r.offer.offers), r.user.puts, hOpen)
			}
			assertSaveAnnounced(t, r.spy.Stub.Notices(), trUnmappedGeneric, hOpen)
		})
	}
}

func TestSaveOffer_EndedRuns_Transform_DryRunStillOffersAndKeepsTheSave(t *testing.T) {
	r := trEnded(t, nil)
	req := r.request()
	req.AutoConfirmPlan = false
	req.DryRun = true

	r.mustRun(req)

	if len(r.offer.offers) != 1 || len(r.user.puts) != 1 || r.user.puts[0].harnessID != hOpen {
		t.Fatalf("offers=%d puts=%+v, want one offer and one entry saved for %s",
			len(r.offer.offers), r.user.puts, hOpen)
	}
	assertSaveAnnounced(t, r.spy.Stub.Notices(), trUnmappedGeneric, hOpen)
}

func TestSaveOffer_EndedRuns_Transform_SaveIsKeptWhenALaterQuestionCancelsTheRun(t *testing.T) {
	// Both generics of the fixture agent's tools are unmapped on the target; the second question is cancelled.
	var ask *cancelSecondAsk
	cancelLater := func(d *app.Deps) {
		ask = &cancelSecondAsk{Interaction: d.Interaction}
		d.Interaction = ask
	}
	r := newTrOffer(t, listFlowAnswer, offerSetup{},
		trUnmapGeneric(hOpen, "file_read"), cancelLater)

	_, err := r.run(r.request())

	if !errors.Is(err, app.ErrSelectionCancelled) {
		t.Fatalf("error = %v, want ErrSelectionCancelled", err)
	}
	if len(r.offer.offers) != 1 || len(r.user.puts) != 1 {
		t.Errorf("offers=%d puts=%+v, want the first answer offered and its entry kept after the cancel",
			len(r.offer.offers), r.user.puts)
	}
	if ask.asked != 2 {
		t.Errorf("custom tool questions asked = %d, want 2 (the second one cancelled)", ask.asked)
	}
}

func TestSaveOffer_EndedRuns_Transform_LaterCallOnTheSameServiceDoesNotAskAgain(t *testing.T) {
	cases := map[string]*domain.ConfirmAnswer{"declined": reviewDeclined, "cancelled": reviewCancelled}
	for name, review := range cases {
		t.Run(name, func(t *testing.T) {
			state := &rebuildState{}
			var override *reviewOverride
			capture := func(d *app.Deps) { override = d.Interaction.(*reviewOverride) }
			r := trEnded(t, review, trRebuildable(t, state), capture)
			first := r.request()
			first.AutoConfirmPlan = false
			_, _ = r.run(first)
			override.answer = nil // the user confirms the next plan

			r.mustRun(r.request())

			if got := r.spy.Stub.CountAsked(domain.QCustomTool, trUnmappedGeneric); got != 1 {
				t.Errorf("custom tool question asked %d times over two transforms, want once", got)
			}
			if state.rebuilds != 1 {
				t.Errorf("registry rebuilt %d times, want once after the save", state.rebuilds)
			}
		})
	}
}
