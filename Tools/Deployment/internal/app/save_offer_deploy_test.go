package app_test

// save_offer_deploy_test.go specifies the save offer of the deploy-family flows (deploy, deploy
// agents, change workflows): after a custom tool answer given in the run, the user is offered to
// remember it; accepting saves the tool_destinations entry that reproduces the answer for the
// run's harness, anything else saves nothing, a failed save is a warning, and the run's output
// for the tool is the same whichever way the offer goes.

import (
	"context"
	"errors"
	"slices"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

func TestSaveOffer_Deploy_AcceptedAnswerIsSavedForTheRunsHarness(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newOfferRun(t, listFlowAnswer, offerSetup{})

			flow.run(t, r.listFlowRun)

			if len(r.user.puts) != 1 {
				t.Fatalf("saved %d entries, want exactly one: %+v", len(r.user.puts), r.user.puts)
			}
			got := r.user.puts[0]
			if got.harnessID != offerHarnessID {
				t.Errorf("saved for harness %q, want %q", got.harnessID, offerHarnessID)
			}
			if want := offerMapping("bash", offerListNames...); !equalMapping(got.mapping, want) {
				t.Errorf("saved mapping = %+v, want %+v", got.mapping, want)
			}
		})
	}
}

func equalMapping(a, b domain.ToolMapping) bool {
	if a.Generic != b.Generic || len(a.Destinations) != len(b.Destinations) {
		return false
	}
	for i := range a.Destinations {
		x, y := a.Destinations[i], b.Destinations[i]
		if x.Kind != y.Kind || x.Field != y.Field || x.Format != y.Format || x.Separator != y.Separator ||
			!slices.Equal(x.Names, y.Names) {
			return false
		}
	}
	return true
}

func TestSaveOffer_Deploy_OfferIsOneConfirmationForTheGenericNamingTheAnswerAndTheHarness(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newOfferRun(t, listFlowAnswer, offerSetup{})

			flow.run(t, r.listFlowRun)

			if len(r.offer.offers) != 1 {
				t.Fatalf("offers = %d, want one for the generic tool both agents declare", len(r.offer.offers))
			}
			q := r.offer.offers[0]
			if q.ID != domain.QSaveCustomTool || q.Subject != "bash" {
				t.Errorf("offer = %+v, want QSaveCustomTool with subject bash", q)
			}
			if !noticeMentions(domain.Notice{Message: q.Prompt}, "tool_destinations", offerHarnessID) {
				t.Errorf("prompt %q does not name tool_destinations and the harness", q.Prompt)
			}
			if !noticeMentions(domain.Notice{Message: q.Detail}, "github/get_pr, jira", r.user.path) {
				t.Errorf("detail %q does not show the answer and the config path", q.Detail)
			}
		})
	}
}

func TestSaveOffer_Deploy_AcceptedSave_IsAnnouncedAsInfo(t *testing.T) {
	r := newOfferRun(t, listFlowAnswer, offerSetup{})

	listFlows[0].run(t, r.listFlowRun)

	infos := noticesAt(r.notices(), domain.NoticeInfo)
	found := false
	for _, n := range infos {
		if noticeMentions(n, "bash", offerHarnessID) {
			found = true
		}
	}
	if !found {
		t.Errorf("no info notice names the remembered tool and harness: %+v", infos)
	}
}

func TestSaveOffer_Deploy_NotAccepted_SavesNothingAndTheRunIsUnchanged(t *testing.T) {
	answers := map[string]func(domain.Question) (domain.ConfirmAnswer, error){
		"declined": declineOffer, "skipped": skipOffer, "cancelled": cancelOffer, "failed": failOffer,
	}
	for _, flow := range listFlows {
		reference := newOfferRun(t, listFlowAnswer, offerSetup{})
		want := reference.outcome(flow.run(t, reference.listFlowRun))
		for name, answer := range answers {
			t.Run(flow.name+"/"+name, func(t *testing.T) {
				r := newOfferRun(t, listFlowAnswer, offerSetup{answer: answer})

				summary := flow.run(t, r.listFlowRun)

				if len(r.offer.offers) != 1 {
					t.Fatalf("offers = %d, want the offer to be made once", len(r.offer.offers))
				}
				if len(r.user.puts) != 0 {
					t.Errorf("saved %+v although the offer was not accepted", r.user.puts)
				}
				if w := noticesAt(r.notices(), domain.NoticeWarning); len(w) != 0 {
					t.Errorf("a refused offer raised warnings: %+v", w)
				}
				assertSameOutcome(t, r.outcome(summary), want)
			})
		}
	}
}

func TestSaveOffer_Deploy_RunOutputIsTheSameWhetherTheOfferIsAcceptedOrDeclined(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			accepted := newOfferRun(t, listFlowAnswer, offerSetup{answer: acceptOffer})
			declined := newOfferRun(t, listFlowAnswer, offerSetup{answer: declineOffer})

			acceptedOutcome := accepted.outcome(flow.run(t, accepted.listFlowRun))
			declinedOutcome := declined.outcome(flow.run(t, declined.listFlowRun))

			if len(accepted.user.puts) != 1 || len(declined.offer.offers) != 1 {
				t.Fatalf("the offer was not made and accepted/declined as scripted (puts=%d, offers=%d)",
					len(accepted.user.puts), len(declined.offer.offers))
			}
			assertSameOutcome(t, declinedOutcome, acceptedOutcome)
			assertEveryAgentGets(t, accepted.listFlowRun, listFlowEntries)
		})
	}
}

func TestSaveOffer_Deploy_FailedSave_IsAWarningAndTheRunSucceeds(t *testing.T) {
	failures := map[string]offerSetup{
		"lock":  {lockErr: errors.New("lock timeout")},
		"write": {putErr: errors.New("disk full")},
	}
	for _, flow := range listFlows {
		reference := newOfferRun(t, listFlowAnswer, offerSetup{})
		want := reference.outcome(flow.run(t, reference.listFlowRun))
		for name, setup := range failures {
			t.Run(flow.name+"/"+name, func(t *testing.T) {
				r := newOfferRun(t, listFlowAnswer, setup)

				summary := flow.run(t, r.listFlowRun) // fails the test on a run error

				warnings := noticesAt(r.notices(), domain.NoticeWarning)
				found := false
				for _, n := range warnings {
					if noticeMentions(n, "bash", r.user.path) {
						found = true
					}
				}
				if !found {
					t.Errorf("no warning names the tool and the config path: %+v", warnings)
				}
				assertSameOutcome(t, r.outcome(summary), want)
			})
		}
	}
}

func TestSaveOffer_Deploy_OfferComesRightAfterEachAnswerBeforeTheNextQuestion(t *testing.T) {
	agents := []domain.Agent{toolAgent("test-runner", "bash", "web-search")}
	r := newOfferRunFor(t, agents, []string{"bash", "web-search"},
		map[string]string{"bash": "github/get_pr", "web-search": "jira"}, offerSetup{})

	listFlows[0].run(t, r.listFlowRun)

	want := []string{"ask:bash", "offer:bash", "ask:web-search", "offer:web-search"}
	if !slices.Equal(r.offer.order, want) {
		t.Errorf("order of questions = %v, want %v", r.offer.order, want)
	}
	if len(r.user.puts) != 2 {
		t.Fatalf("saved %d entries, want one per answered tool", len(r.user.puts))
	}
	if r.user.puts[0].mapping.Generic != "bash" || r.user.puts[1].mapping.Generic != "web-search" {
		t.Errorf("saved generics = %q, %q; want bash then web-search",
			r.user.puts[0].mapping.Generic, r.user.puts[1].mapping.Generic)
	}
}

// The cases below pin where no offer is made. Each pairs with the accepted-answer test above,
// which makes the same run and does get the offer.

func TestSaveOffer_Deploy_NoOfferWhenNothingWasAnsweredInTheRun(t *testing.T) {
	cases := map[string]string{"blank answer": "   ", "refused text": "github/a/b"}
	for name, answer := range cases {
		t.Run(name, func(t *testing.T) {
			r := newOfferRun(t, answer, offerSetup{})

			listFlows[0].run(t, r.listFlowRun)

			if len(r.offer.offers) != 0 || len(r.user.puts) != 0 {
				t.Errorf("offers=%d puts=%d, want none for an answer that produced no entries",
					len(r.offer.offers), len(r.user.puts))
			}
		})
	}
}

func TestSaveOffer_Deploy_NoOfferForAPreAnsweredGeneric(t *testing.T) {
	r := newOfferRun(t, "unused", offerSetup{})

	_, err := app.New(r.deps).DeployNew(context.Background(), app.DeployRequest{
		HarnessID: offerHarnessID, WorkspacePath: r.workDir, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"quick-fix"}, SkipAll: listSkipModels, AutoConfirmPlan: true,
		CustomTools: domain.CustomToolAnswers{"bash": listFlowEntries},
	})

	if err != nil {
		t.Fatalf("DeployNew: %v", err)
	}
	if len(r.offer.offers) != 0 || len(r.user.puts) != 0 {
		t.Errorf("offers=%d puts=%d, want none for a pre-answer", len(r.offer.offers), len(r.user.puts))
	}
}

func TestSaveOffer_Deploy_OfferOnlyForHarnessesWhoseConfigEntriesApply(t *testing.T) {
	cases := []struct {
		name  string
		setup offerSetup
		offer bool
	}{
		{"builtin with MCP forms", offerSetup{tier: domain.TierBuiltin}, true},
		{"descriptor with MCP forms", offerSetup{tier: domain.TierDescriptor}, true},
		{"external", offerSetup{tier: domain.TierExternal}, false},
		{"builtin without MCP forms", offerSetup{tier: domain.TierBuiltin, noForms: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newOfferRun(t, listFlowAnswer, c.setup)

			listFlows[0].run(t, r.listFlowRun)

			if got := len(r.offer.offers) == 1; got != c.offer {
				t.Errorf("offer made = %v (%d offers), want %v", got, len(r.offer.offers), c.offer)
			}
			if got := len(r.user.puts) == 1; got != c.offer {
				t.Errorf("entry saved = %v (%d puts), want %v", got, len(r.user.puts), c.offer)
			}
		})
	}
}

func TestSaveOffer_Deploy_MixedAnswersSaveOnlyTheAcceptedTool(t *testing.T) {
	agents := []domain.Agent{toolAgent("test-runner", "bash", "web-search")}
	acceptBashOnly := func(q domain.Question) (domain.ConfirmAnswer, error) {
		if q.Subject == "bash" {
			return acceptOffer(q)
		}
		return declineOffer(q)
	}
	r := newOfferRunFor(t, agents, []string{"bash", "web-search"},
		map[string]string{"bash": "github/get_pr", "web-search": "jira"}, offerSetup{answer: acceptBashOnly})

	listFlows[0].run(t, r.listFlowRun)

	if len(r.offer.offers) != 2 {
		t.Fatalf("offers = %d, want one per answered tool", len(r.offer.offers))
	}
	if len(r.user.puts) != 1 || r.user.puts[0].mapping.Generic != "bash" {
		t.Errorf("saved %+v, want only the accepted tool bash", r.user.puts)
	}
}
