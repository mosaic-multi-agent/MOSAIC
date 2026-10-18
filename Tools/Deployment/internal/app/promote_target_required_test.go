package app

// promote_target_required_test.go covers the error returned when a new agent is promoted into
// a stack of several layers and no target catalog is chosen: it must stay matchable as the
// target-required sentinel and list the catalog ids of the stack, highest layer first.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// targetAnswerInteraction answers every choice question with a fixed answer and error.
type targetAnswerInteraction struct {
	domain.Interaction
	answer domain.ChoiceAnswer
	err    error
}

func (i *targetAnswerInteraction) SelectOne(context.Context, domain.ChoiceQuestion) (domain.ChoiceAnswer, error) {
	return i.answer, i.err
}

func targetRequiredLayers() []domain.CatalogLayer {
	return []domain.CatalogLayer{
		{Identity: domain.CatalogIdentity{ID: "team-catalog"}, Root: "/r/team"},
		{Identity: domain.CatalogIdentity{ID: "mid-catalog"}, Root: "/r/mid"},
		{Identity: domain.CatalogIdentity{ID: "mosaic-default"}, Root: "/r/default"},
	}
}

// assertListsIDsHighestFirst fails unless msg names every layer id in stack order.
func assertListsIDsHighestFirst(t *testing.T, msg string, layers []domain.CatalogLayer) {
	t.Helper()
	from := 0
	for _, l := range layers {
		idx := strings.Index(msg[from:], l.Identity.ID)
		if idx < 0 {
			t.Fatalf("error %q does not list catalog id %q after position %d (highest first)", msg, l.Identity.ID, from)
		}
		from += idx + len(l.Identity.ID)
	}
}

func TestAskPromoteTargetLayer_Unanswered_ListsStackIDsHighestFirst(t *testing.T) {
	statuses := map[string]domain.AnswerStatus{
		"skipped":   domain.SkippedOne,
		"cancelled": domain.Cancelled,
	}
	for name, status := range statuses {
		t.Run(name, func(t *testing.T) {
			// Arrange
			layers := targetRequiredLayers()
			s := &service{deps: Deps{Interaction: &targetAnswerInteraction{answer: domain.ChoiceAnswer{Status: status}}}}

			// Act
			_, err := s.askPromoteTargetLayer(context.Background(), layers, "/src/agent.md")

			// Assert
			if !errors.Is(err, errPromoteTargetRequired) {
				t.Fatalf("err = %v, want it to match errPromoteTargetRequired", err)
			}
			assertListsIDsHighestFirst(t, err.Error(), layers)
		})
	}
}

func TestAskPromoteTargetLayer_InteractionError_ListsStackIDsAndKeepsCause(t *testing.T) {
	// Arrange
	layers := targetRequiredLayers()
	cause := errors.New("terminal closed")
	s := &service{deps: Deps{Interaction: &targetAnswerInteraction{err: cause}}}

	// Act
	_, err := s.askPromoteTargetLayer(context.Background(), layers, "/src/agent.md")

	// Assert
	if !errors.Is(err, errPromoteTargetRequired) || !errors.Is(err, cause) {
		t.Fatalf("err = %v, want it to match errPromoteTargetRequired and the interaction cause", err)
	}
	assertListsIDsHighestFirst(t, err.Error(), layers)
}

func TestAskPromoteTargetLayer_AnswerNamingUnknownLayer_IsNotTheTargetRequiredError(t *testing.T) {
	// Arrange
	s := &service{deps: Deps{Interaction: &targetAnswerInteraction{
		answer: domain.ChoiceAnswer{Status: domain.Answered, OptionID: "nope"},
	}}}

	// Act
	_, err := s.askPromoteTargetLayer(context.Background(), targetRequiredLayers(), "/src/agent.md")

	// Assert
	if err == nil || errors.Is(err, errPromoteTargetRequired) {
		t.Fatalf("err = %v, want the unchanged not-an-active-layer error", err)
	}
}
