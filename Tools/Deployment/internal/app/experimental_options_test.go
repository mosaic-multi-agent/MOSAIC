package app_test

// experimental_options_test.go tests that the two question pickers that choose a harness (the
// harness question and the transform target-harness question) still offer an experimental
// harness, mark it with the experimental label, carry its reason in the option description,
// and leave every other option and every disabled rule exactly as they were.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

var (
	optPlainRef = domain.HarnessRef{
		ID: expPlainID, DisplayName: "Plain Harness", Tier: domain.TierBuiltin, Usable: true,
	}
	optExpRef = domain.HarnessRef{
		ID: expHarnessID, DisplayName: "Preview Harness", Tier: domain.TierBuiltin, Usable: true,
		ExperimentalReason: expReason,
	}
	optExpUnusableRef = domain.HarnessRef{
		ID: expHarnessID2, DisplayName: "Broken Preview", Tier: domain.TierBuiltin, Usable: false,
		UnusableReason: "descriptor failed to load", ExperimentalReason: expReason2,
	}
)

// assertExperimentalOption checks the label and description of an option for an experimental
// ref, and that it is not disabled.
func assertExperimentalOption(t *testing.T, o domain.Option, ref domain.HarnessRef) {
	t.Helper()
	if want := ref.DisplayName + " [" + domain.ExperimentalLabel + "]"; o.Label != want {
		t.Errorf("experimental option label = %q, want %q", o.Label, want)
	}
	if o.Description != ref.ExperimentalReason {
		t.Errorf("experimental option description = %q, want the reason %q", o.Description, ref.ExperimentalReason)
	}
	if o.ID != ref.ID {
		t.Errorf("experimental option id = %q, want %q", o.ID, ref.ID)
	}
}

// assertUnchangedOption checks an option equals what a harness without the declaration gets.
func assertUnchangedOption(t *testing.T, o domain.Option, ref domain.HarnessRef) {
	t.Helper()
	want := domain.Option{ID: ref.ID, Label: ref.DisplayName, Disabled: !ref.Usable, DisabledReason: ref.UnusableReason}
	if o != want {
		t.Errorf("option for %q = %+v, want %+v", ref.ID, o, want)
	}
}

// harnessQuestionFor asks the harness question through DeployNew over a registry holding refs
// and returns the question the interaction port received.
func harnessQuestionFor(t *testing.T, refs ...domain.HarnessRef) domain.ChoiceQuestion {
	t.Helper()
	rec := newRecordingInteraction(map[domain.QuestionID]string{domain.QHarness: refs[0].ID})
	deps, ws := newBaseDeps(t, interactiontest.NewBuilder().Build())
	deps.Registry = &stubRegistry{list: refs}
	deps.Interaction = rec
	svc := app.New(deps)

	_, _ = svc.DeployNew(context.Background(), app.DeployRequest{WorkspacePath: ws, Scope: domain.ScopeProject})

	return rec.question(t, domain.QHarness)
}

func TestHarnessQuestion_ExperimentalHarness_OfferedAndMarked(t *testing.T) {
	// Arrange / Act
	q := harnessQuestionFor(t, optPlainRef, optExpRef)

	// Assert
	assertExperimentalOption(t, optionByID(t, q, optExpRef.ID), optExpRef)
	if o := optionByID(t, q, optExpRef.ID); o.Disabled || o.DisabledReason != "" {
		t.Errorf("experimental option disabled = %v (%q), want selectable", o.Disabled, o.DisabledReason)
	}
}

func TestHarnessQuestion_NonExperimentalHarness_OptionUnchanged(t *testing.T) {
	// Arrange / Act
	q := harnessQuestionFor(t, optPlainRef, optExpRef)

	// Assert
	assertUnchangedOption(t, optionByID(t, q, optPlainRef.ID), optPlainRef)
	if len(q.Options) != 2 {
		t.Errorf("len(Options) = %d, want 2 (one per registry harness)", len(q.Options))
	}
}

func TestHarnessQuestion_UnusableExperimentalHarness_StaysDisabledWithUnusableReason(t *testing.T) {
	// Arrange / Act
	q := harnessQuestionFor(t, optPlainRef, optExpUnusableRef)

	// Assert
	o := optionByID(t, q, optExpUnusableRef.ID)
	assertExperimentalOption(t, o, optExpUnusableRef)
	if !o.Disabled || o.DisabledReason != optExpUnusableRef.UnusableReason {
		t.Errorf("unusable experimental option = disabled %v reason %q, want disabled with %q",
			o.Disabled, o.DisabledReason, optExpUnusableRef.UnusableReason)
	}
}

// targetQuestionFor asks the transform target question for a transform whose source is
// transformSourceHarness over a registry of the stub source module plus the given refs, and
// returns the question the interaction port received.
func targetQuestionFor(t *testing.T, source domain.HarnessRef, refs ...domain.HarnessRef) domain.ChoiceQuestion {
	t.Helper()
	rec := newRecordingInteraction(map[domain.QuestionID]string{domain.QTransformTargetHarness: refs[0].ID})
	deps, _ := newTransformDeps(t, interactiontest.NewBuilder().Build())
	list := append([]domain.HarnessRef{source}, refs...)
	modules := map[string]domain.HarnessModule{}
	for _, ref := range list {
		modules[ref.ID] = &stubHarnessModule{ref: ref, descriptor: domain.HarnessDescriptor{
			ID: ref.ID, DisplayName: ref.DisplayName, ExperimentalReason: ref.ExperimentalReason,
		}}
	}
	deps.Registry = &stubRegistry{list: list, modules: modules}
	deps.Interaction = rec
	svc := app.New(deps)
	agentPath := writeTransformFile(t, t.TempDir(), "my-agent.src.md", matchingAgentBytes())
	req := newPreAnsweredTransformRequest(agentPath)
	req.SourceHarnessID = source.ID
	req.TargetHarnessID = ""

	_, _ = svc.TransformHarness(context.Background(), req)

	return rec.question(t, domain.QTransformTargetHarness)
}

func TestTransformTargetQuestion_ExperimentalHarness_OfferedAndMarked(t *testing.T) {
	// Arrange / Act
	q := targetQuestionFor(t, transformSourceHarness, optExpRef, optPlainRef)

	// Assert
	o := optionByID(t, q, optExpRef.ID)
	assertExperimentalOption(t, o, optExpRef)
	if o.Disabled || o.DisabledReason != "" {
		t.Errorf("experimental target option disabled = %v (%q), want selectable", o.Disabled, o.DisabledReason)
	}
}

func TestTransformTargetQuestion_NonExperimentalHarness_OptionUnchanged(t *testing.T) {
	// Arrange / Act
	q := targetQuestionFor(t, transformSourceHarness, optExpRef, optPlainRef)

	// Assert
	assertUnchangedOption(t, optionByID(t, q, optPlainRef.ID), optPlainRef)
	srcOpt := optionByID(t, q, transformSourceHarness.ID)
	if srcOpt.Label != transformSourceHarness.DisplayName || srcOpt.Description != "" || !srcOpt.Disabled {
		t.Errorf("non-experimental source option = %+v, want plain label, no description, disabled", srcOpt)
	}
}

func TestTransformTargetQuestion_ExperimentalSource_MarkedButStillDisabledAsSource(t *testing.T) {
	// Arrange: the transform source itself is experimental.
	src := transformSourceHarness
	src.ExperimentalReason = expReason

	// Act
	q := targetQuestionFor(t, src, optPlainRef)

	// Assert
	o := optionByID(t, q, src.ID)
	assertExperimentalOption(t, o, src)
	if !o.Disabled || o.DisabledReason == "" {
		t.Errorf("experimental source option = disabled %v reason %q, want disabled as the transform source",
			o.Disabled, o.DisabledReason)
	}
	plain := optionByID(t, q, optPlainRef.ID)
	assertUnchangedOption(t, plain, optPlainRef)
}

func TestTransformTargetQuestion_UnusableExperimentalHarness_StaysDisabledWithUnusableReason(t *testing.T) {
	// Arrange / Act
	q := targetQuestionFor(t, transformSourceHarness, optPlainRef, optExpUnusableRef)

	// Assert
	o := optionByID(t, q, optExpUnusableRef.ID)
	assertExperimentalOption(t, o, optExpUnusableRef)
	if !o.Disabled || o.DisabledReason != optExpUnusableRef.UnusableReason {
		t.Errorf("unusable experimental target option = disabled %v reason %q, want disabled with %q",
			o.Disabled, o.DisabledReason, optExpUnusableRef.UnusableReason)
	}
}
