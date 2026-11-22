package app_test

// promote_category_question_test.go covers the category question of single-agent promote: it
// offers no skip, and a category that is cancelled, skipped or errored still ends the run
// with ErrPromoteCategoryRequired and writes nothing.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// categoryQuestionSpy answers QPromoteCategory with a fixed answer (or error) and records the
// questions it was asked. Every other question is forwarded to the embedded stub.
type categoryQuestionSpy struct {
	*interactiontest.Stub
	answer domain.ChoiceAnswer
	err    error
	asked  []domain.ChoiceQuestion
}

func (c *categoryQuestionSpy) SelectOne(ctx context.Context, q domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	if q.ID == domain.QPromoteCategory {
		c.asked = append(c.asked, q)
		return c.answer, c.err
	}
	return c.Stub.SelectOne(ctx, q)
}

func newCategoryQuestionSpy(answer domain.ChoiceAnswer, err error) *categoryQuestionSpy {
	return &categoryQuestionSpy{Stub: interactiontest.NewBuilder().Build(), answer: answer, err: err}
}

// newCategoryPromote returns a service on a plain catalog, an eligible source file (no
// pre-supplied category) and the catalog root that receives the promoted file.
func newCategoryPromote(t *testing.T, ix domain.Interaction) (app.Service, app.PromoteRequest, string) {
	t.Helper()
	mosaicRoot := writeMosaicRoot(t)
	deps, _ := newPromoteDeps(t, interactiontest.NewBuilder().Build(), mosaicRoot)
	deps.Interaction = ix
	src := writeEligibleSourceFile(t, t.TempDir(), plAgentKey+".md")
	req := app.PromoteRequest{FilePath: src, HarnessID: "stub-harness"}
	return app.New(deps), req, filepath.Join(mosaicRoot, "Catalog")
}

// TestPromote_CategoryQuestion_OffersNoSkip verifies QPromoteCategory is asked without a skip
// option, so the frontend shows no "None" row.
func TestPromote_CategoryQuestion_OffersNoSkip(t *testing.T) {
	// Arrange
	spy := newCategoryQuestionSpy(domain.ChoiceAnswer{Status: domain.Answered, OptionID: "Audit"}, nil)
	svc, req, catRoot := newCategoryPromote(t, spy)

	// Act
	result, err := svc.Promote(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if len(spy.asked) != 1 {
		t.Fatalf("category question asked %d times, want 1", len(spy.asked))
	}
	if spy.asked[0].AllowSkip {
		t.Error("category question has AllowSkip set; it must offer no skip option")
	}
	if spy.asked[0].AllowSkipAll {
		t.Error("category question has AllowSkipAll set; it must offer no skip option")
	}
	if want := filepath.Join(catRoot, "Subagents", "Audit", plAgentKey+".md"); result.DestinationPath != want {
		t.Errorf("DestinationPath = %q, want %q", result.DestinationPath, want)
	}
}

// TestPromote_CategoryQuestion_NotAnswered_RequiresCategoryAndWritesNothing verifies a
// cancelled or unanswered category question ends the run with ErrPromoteCategoryRequired and
// writes nothing.
func TestPromote_CategoryQuestion_NotAnswered_RequiresCategoryAndWritesNothing(t *testing.T) {
	statuses := map[string]domain.AnswerStatus{
		"cancelled":   domain.Cancelled,
		"skipped":     domain.SkippedOne,
		"skipped all": domain.SkippedAll,
	}
	for name, status := range statuses {
		t.Run(name, func(t *testing.T) {
			// Arrange
			spy := newCategoryQuestionSpy(domain.ChoiceAnswer{Status: status}, nil)
			svc, req, catRoot := newCategoryPromote(t, spy)

			// Act
			result, err := svc.Promote(context.Background(), req)

			// Assert
			if !errors.Is(err, app.ErrPromoteCategoryRequired) {
				t.Fatalf("error = %v, want one wrapping ErrPromoteCategoryRequired", err)
			}
			if result.DestinationPath != "" {
				t.Errorf("result carries destination %q despite the error", result.DestinationPath)
			}
			if _, statErr := os.Stat(filepath.Join(catRoot, "Subagents")); statErr == nil {
				t.Error("a Subagents directory was created although no category was chosen")
			}
		})
	}
}

// TestPromote_CategoryQuestion_InteractionError_RequiresCategoryAndWritesNothing verifies an
// error from the interaction port is reported as ErrPromoteCategoryRequired and writes nothing.
func TestPromote_CategoryQuestion_InteractionError_RequiresCategoryAndWritesNothing(t *testing.T) {
	// Arrange
	boom := errors.New("interaction failed")
	spy := newCategoryQuestionSpy(domain.ChoiceAnswer{}, boom)
	svc, req, catRoot := newCategoryPromote(t, spy)

	// Act
	_, err := svc.Promote(context.Background(), req)

	// Assert
	if !errors.Is(err, app.ErrPromoteCategoryRequired) {
		t.Fatalf("error = %v, want one wrapping ErrPromoteCategoryRequired", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("error = %v, want it to keep the interaction error", err)
	}
	if _, statErr := os.Stat(filepath.Join(catRoot, "Subagents")); statErr == nil {
		t.Error("a Subagents directory was created although no category was chosen")
	}
}
