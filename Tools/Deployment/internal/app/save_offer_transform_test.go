package app_test

// save_offer_transform_test.go specifies the save offer of the whole-deployment transform: after
// a custom tool answer, the user is offered to remember it for the TARGET harness (never the
// source); accepting saves the entry that reproduces the answer for the target, anything else
// saves nothing and never cancels the transform, a failed save is a warning, and the written
// agent is the same whichever way the offer goes.

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

// trOffer is a transform of the subagent fixture (source claude-code, target opencode, which
// cannot map trUnmappedGeneric) whose user config is a spy and whose save offer follows o.
type trOffer struct {
	*trRun
	user  *spyUserConfig
	offer *offerSpy
}

func newTrOffer(t *testing.T, answer string, o offerSetup, extra ...func(*app.Deps)) *trOffer {
	t.Helper()
	script := interactiontest.NewBuilder().AnswerText(domain.QCustomTool, trUnmappedGeneric, answer).Build()
	user := &spyUserConfig{
		path:    filepath.Join(t.TempDir(), "user-config.yaml"),
		lockErr: o.lockErr, putErr: o.putErr,
	}
	spy := &offerSpy{answer: o.answer}
	wire := func(d *app.Deps) {
		d.UserConfig = user
		spy.Interaction = d.Interaction
		d.Interaction = spy
	}
	opts := append([]func(*app.Deps){trUnmapGeneric(hOpen, trUnmappedGeneric), wire}, extra...)
	r := trNew(t, script, hClaude, hOpen, opts...)
	r.seed("subagent")
	return &trOffer{trRun: r, user: user, offer: spy}
}

// savedMappingFor is the entry that reproduces the answer on the target harness: the destinations
// of the target module's own custom resolution of the generic.
func (r *trOffer) savedMappingFor(answer string) domain.ToolMapping {
	r.t.Helper()
	entries, err := customtool.Parse(answer)
	if err != nil {
		r.t.Fatalf("parse %q: %v", answer, err)
	}
	mod, err := r.deps.Registry.Resolve(hOpen)
	if err != nil {
		r.t.Fatalf("resolve %s: %v", hOpen, err)
	}
	res, err := mod.Tools(domain.ToolRequest{
		AgentKey: "subagent", Generic: []string{trUnmappedGeneric},
		CustomNames: domain.CustomToolAnswers{trUnmappedGeneric: entries},
	})
	if err != nil || len(res.Resolutions) != 1 {
		r.t.Fatalf("Tools: %v (%d resolutions)", err, len(res.Resolutions))
	}
	m, err := config.CustomToolMapping(res.Resolutions[0])
	if err != nil {
		r.t.Fatalf("CustomToolMapping: %v", err)
	}
	return m
}

// result is what the transform produced for the subagent, independent of the offer.
type trOfferResult struct {
	Content       string
	Status        app.TransformFileStatus
	CustomTools   []string
	UnmappedTools []string
}

func (r *trOffer) result(res app.TransformHarnessResult) trOfferResult {
	f := trFile(r.t, res, "subagent")
	return trOfferResult{
		Content: r.read(r.dest("subagent")), Status: f.Status,
		CustomTools: f.CustomTools, UnmappedTools: f.UnmappedTools,
	}
}

func TestSaveOffer_Transform_AcceptedAnswerIsSavedForTheTargetHarnessOnly(t *testing.T) {
	r := newTrOffer(t, listFlowAnswer, offerSetup{})

	r.mustRun(r.request())

	if len(r.user.puts) != 1 {
		t.Fatalf("saved %d entries, want exactly one: %+v", len(r.user.puts), r.user.puts)
	}
	got := r.user.puts[0]
	if got.harnessID != hOpen {
		t.Errorf("saved for harness %q, want the target %q (the source is %q)", got.harnessID, hOpen, hClaude)
	}
	if want := r.savedMappingFor(listFlowAnswer); !reflect.DeepEqual(got.mapping, want) {
		t.Errorf("saved mapping = %+v, want %+v", got.mapping, want)
	}
}

func TestSaveOffer_Transform_OfferIsOneConfirmationForTheGenericNamingTheAnswerAndTheTarget(t *testing.T) {
	r := newTrOffer(t, listFlowAnswer, offerSetup{})

	r.mustRun(r.request())

	if len(r.offer.offers) != 1 {
		t.Fatalf("offers = %d, want one", len(r.offer.offers))
	}
	q := r.offer.offers[0]
	if q.ID != domain.QSaveCustomTool || q.Subject != trUnmappedGeneric {
		t.Errorf("offer = %+v, want QSaveCustomTool with subject %s", q, trUnmappedGeneric)
	}
	if !noticeMentions(domain.Notice{Message: q.Prompt}, "tool_destinations", hOpen) {
		t.Errorf("prompt %q does not name tool_destinations and the target harness", q.Prompt)
	}
	if !noticeMentions(domain.Notice{Message: q.Detail}, "github/get_pr, jira", r.user.path) {
		t.Errorf("detail %q does not show the answer and the config path", q.Detail)
	}
}

func TestSaveOffer_Transform_NotAccepted_SavesNothingNeverCancelsAndWritesTheSameAgent(t *testing.T) {
	reference := newTrOffer(t, listFlowAnswer, offerSetup{})
	want := reference.result(reference.mustRun(reference.request()))
	answers := map[string]func(domain.Question) (domain.ConfirmAnswer, error){
		"declined": declineOffer, "skipped": skipOffer, "cancelled": cancelOffer, "failed": failOffer,
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			r := newTrOffer(t, listFlowAnswer, offerSetup{answer: answer})

			res := r.mustRun(r.request()) // a cancelled or failed offer must not abort the run

			if len(r.offer.offers) != 1 {
				t.Fatalf("offers = %d, want the offer to be made once", len(r.offer.offers))
			}
			if len(r.user.puts) != 0 {
				t.Errorf("saved %+v although the offer was not accepted", r.user.puts)
			}
			if w := noticesAt(r.spy.Stub.Notices(), domain.NoticeWarning); len(w) != 0 {
				t.Errorf("a refused offer raised warnings: %+v", w)
			}
			if got := r.result(res); !reflect.DeepEqual(got, want) {
				t.Errorf("result differs from the run with the offer accepted:\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

func TestSaveOffer_Transform_FailedSave_IsAWarningAndTheAgentIsStillWritten(t *testing.T) {
	reference := newTrOffer(t, listFlowAnswer, offerSetup{})
	want := reference.result(reference.mustRun(reference.request()))
	failures := map[string]offerSetup{
		"lock":  {lockErr: errors.New("lock timeout")},
		"write": {putErr: errors.New("disk full")},
	}
	for name, setup := range failures {
		t.Run(name, func(t *testing.T) {
			r := newTrOffer(t, listFlowAnswer, setup)

			res := r.mustRun(r.request())

			found := false
			for _, n := range noticesAt(r.spy.Stub.Notices(), domain.NoticeWarning) {
				if noticeMentions(n, trUnmappedGeneric, r.user.path) {
					found = true
				}
			}
			if !found {
				t.Errorf("no warning names the tool and the config path: %+v", r.spy.Stub.Notices())
			}
			if got := r.result(res); !reflect.DeepEqual(got, want) {
				t.Errorf("result differs from the run with the offer accepted:\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

func TestSaveOffer_Transform_AcceptedSave_IsAnnouncedAsInfo(t *testing.T) {
	r := newTrOffer(t, listFlowAnswer, offerSetup{})

	r.mustRun(r.request())

	found := false
	for _, n := range noticesAt(r.spy.Stub.Notices(), domain.NoticeInfo) {
		if noticeMentions(n, trUnmappedGeneric, hOpen) {
			found = true
		}
	}
	if !found {
		t.Errorf("no info notice names the remembered tool and the target: %+v", r.spy.Stub.Notices())
	}
}

func TestSaveOffer_Transform_NoOfferWhenTheToolWasSkipped(t *testing.T) {
	r := newTrOffer(t, "   ", offerSetup{}) // blank answer: skipped

	r.mustRun(r.request())

	if len(r.offer.offers) != 0 || len(r.user.puts) != 0 {
		t.Errorf("offers=%d puts=%d, want none when no entries were answered", len(r.offer.offers), len(r.user.puts))
	}
}
