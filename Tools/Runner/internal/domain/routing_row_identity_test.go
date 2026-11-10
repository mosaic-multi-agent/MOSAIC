package domain_test

// Row identity accessors on RoutingRow: Block() and BareAgentID(), exercised
// on hand-built literals (no parsing involved).

import (
	"testing"

	"mosaic-run/internal/domain"
)

func TestRoutingRowBlock_ByPhaseShape(t *testing.T) {
	tests := []struct {
		name      string
		phase     domain.PhaseParsed
		wantBlock domain.BlockName
		wantOK    bool
	}{
		{
			name:      "blocks set and staged gives the outermost block",
			phase:     domain.PhaseParsed{Name: "EXECUTION", IsStaged: true, Blocks: []domain.BlockName{"Iteration", "Stage"}},
			wantBlock: "Iteration",
			wantOK:    true,
		},
		{
			name:      "blocks set wins even when IsStaged is false",
			phase:     domain.PhaseParsed{Name: "EXECUTION", IsStaged: false, Blocks: []domain.BlockName{"Sprint"}},
			wantBlock: "Sprint",
			wantOK:    true,
		},
		{
			name:      "neither blocks nor IsStaged is outside every block",
			phase:     domain.PhaseParsed{Name: "PLANNING"},
			wantBlock: "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := domain.RoutingRow{Phase: tt.phase.Name, PhaseParsed: tt.phase}

			gotBlock, gotOK := row.Block()

			if gotBlock != tt.wantBlock || gotOK != tt.wantOK {
				t.Errorf("Block(): want (%q, %v), got (%q, %v)", tt.wantBlock, tt.wantOK, gotBlock, gotOK)
			}
		})
	}
}

func TestRoutingRowBareAgentID_ByCells(t *testing.T) {
	tests := []struct {
		name string
		row  domain.RoutingRow
		want string
	}{
		{
			name: "agent id set is returned",
			row:  domain.RoutingRow{Agent: "planner", AgentID: "planner"},
			want: "planner",
		},
		{
			name: "qualified row returns the bare id, not the whole cell",
			row:  domain.RoutingRow{Agent: "build-review(tests)", AgentID: "build-review", Qualifier: "tests"},
			want: "build-review",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.row.BareAgentID(); got != tt.want {
				t.Errorf("BareAgentID(): want %q, got %q", tt.want, got)
			}
		})
	}
}
