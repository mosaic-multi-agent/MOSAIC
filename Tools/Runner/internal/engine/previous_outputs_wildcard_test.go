package engine_test

// A wildcard entry in the routing row's Output column is injected unchanged
// into the route-back inputs, live and on resume alike: the engine expands no
// wildcard, the session does at dispatch.

import (
	"testing"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

const wildcardOutputReviewContent = v216Columns +
	`| PLANNING | agent-a | FALSE | agent-b | - | - | design.md |
| PLANNING | agent-b | FALSE | COMPLETE | agent-a | design.md | reviews/*.md, review.md |
`

func TestNext_RouteBack_WildcardOutputOfReviewer_IsInjectedUnchanged(t *testing.T) {
	aw := mustParseAndAdmitV216(t, wildcardOutputReviewContent, "wildcard-review", "2.0")
	responses := map[string]*domain.ProtocolResponse{
		"live":   cnaResponse("agent-b#2"),
		"resume": nil,
	}
	for name, resp := range responses {
		t.Run(name, func(t *testing.T) {
			dec := engine.Next(engine.NextInput{
				Workflow:     aw,
				State:        stateAfterRow("PLANNING", "", "agent-b#2", domain.StatusCOMPLETED_NEEDS_ACTION, 2, 2),
				LastResponse: resp,
				Agents:       newTestAgents("agent-a", "agent-b"),
				Seq:          2,
				Now:          fixedNow,
				Mode:         domain.ExecutionModeAutoReview,
			})

			step := requireDispatch(t, dec)

			inputs := step.Request.InputArtifacts
			if !containsPath(inputs, "reviews/*.md") || !containsPath(inputs, "review.md") {
				t.Errorf("want the reviewer's Output entries unchanged in the route-back inputs, got %v", inputs)
			}
		})
	}
}
