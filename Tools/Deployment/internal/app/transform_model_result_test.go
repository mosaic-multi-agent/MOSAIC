package app_test

// transform_model_result_test.go specifies how the model answers of a whole-deployment
// transform are offered and reported: options come from the target harness's models, an
// offered option is written, the question title tells the groups apart, and the result echoes
// the applied mapping (TargetModel when one model was applied, AppliedModelMap per group).

import (
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// trModelCatalogModule is a harness module whose descriptor declares the given target models.
type trModelCatalogModule struct {
	domain.HarnessModule
	desc domain.HarnessDescriptor
}

func (m trModelCatalogModule) Descriptor() *domain.HarnessDescriptor { return &m.desc }

// trTargetModelIDs makes the harness declare ids as its models.
func trTargetModelIDs(harnessID string, ids ...string) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := d.Registry.(*stubRegistry)
		base := reg.modules[harnessID]
		desc := *base.Descriptor()
		desc.Models.IDs = ids
		reg.modules[harnessID] = trModelCatalogModule{HarnessModule: base, desc: desc}
	}
}

func TestTransformModelResult_QuestionOptionsAreTheTargetHarnessModels(t *testing.T) {
	ids := []string{"model-x", "model-y"}
	r := trNew(t, nil, hClaude, hOpen, trTargetModelIDs(hOpen, ids...))
	r.seed("subagent")

	r.mustRun(r.request())

	qs := r.spy.modelQuestions()
	if len(qs) != 1 {
		t.Fatalf("model questions = %d, want 1", len(qs))
	}
	var got []string
	for _, o := range qs[0].Options {
		got = append(got, o.ID)
		if o.Label == "" {
			t.Errorf("option %q has no label", o.ID)
		}
	}
	if !slices.Equal(got, ids) {
		t.Errorf("options = %q, want the target models %q", got, ids)
	}
}

func TestTransformModelResult_AnsweredOption_IsWrittenToTheAgent(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QTransformTargetModel, mSonnet, "model-y").Build()
	r := trNew(t, script, hClaude, hOpen, trTargetModelIDs(hOpen, "model-x", "model-y"))
	r.seed("subagent")

	r.mustRun(r.request())

	trAssertModel(t, r, "subagent", "model-y")
}

func TestTransformModelResult_QuestionTitleTellsTheGroupsApart(t *testing.T) {
	r := trModelRun(t, nil)

	r.mustRun(r.request())

	titles := map[string]string{}
	for _, q := range r.spy.modelQuestions() {
		titles[q.Subject] = q.Title
	}
	if titles[mUnset] == "" || titles[mSonnet] == "" {
		t.Fatalf("missing titles: %+v", titles)
	}
	if titles[mUnset] == titles[mSonnet] {
		t.Errorf("the unset group and a named group share the title %q", titles[mSonnet])
	}
	if !strings.Contains(titles[mSonnet], mSonnet) {
		t.Errorf("title %q does not name the source model %q", titles[mSonnet], mSonnet)
	}
}

func TestTransformModelResult_OneAppliedModel_IsEchoedAsTargetModelAndPerGroup(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.TargetModel = "pre/answered"

	res := r.mustRun(req)

	if res.TargetModel != "pre/answered" {
		t.Errorf("TargetModel = %q, want %q", res.TargetModel, "pre/answered")
	}
	for _, g := range trModelGroups {
		if got, ok := res.AppliedModelMap[g]; !ok || got != "pre/answered" {
			t.Errorf("AppliedModelMap[%q] = %q (present %v), want %q", g, got, ok, "pre/answered")
		}
	}
}

func TestTransformModelResult_SeveralAppliedModels_LeaveTargetModelEmptyAndMapEachGroup(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.ModelMap = app.TransformModelMapping{mSonnet: "pre/a", mOpus: "pre/b"}
	req.SkipAll = map[domain.QuestionID]bool{domain.QTransformTargetModel: true}

	res := r.mustRun(req)

	if res.TargetModel != "" {
		t.Errorf("TargetModel = %q, want empty when several models were applied", res.TargetModel)
	}
	if res.AppliedModelMap[mSonnet] != "pre/a" || res.AppliedModelMap[mOpus] != "pre/b" {
		t.Errorf("AppliedModelMap = %v, want sonnet->pre/a and opus->pre/b", res.AppliedModelMap)
	}
}

func TestTransformModelResult_GroupResolvedToNothing_IsRecordedWithAnEmptyTarget(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.ModelMap = app.TransformModelMapping{mSonnet: ""}
	req.SkipAll = map[domain.QuestionID]bool{domain.QTransformTargetModel: true}

	res := r.mustRun(req)

	if got, ok := res.AppliedModelMap[mSonnet]; !ok || got != "" {
		t.Errorf("AppliedModelMap[sonnet] = %q (present %v), want present and empty", got, ok)
	}
}
