package app_test

// custom_tool_list_gaps_test.go shows that the unmapped-tool reporting of the deploy-family
// flows discriminates per generic tool: when an agent holds two unmapped generic tools and only
// one is answered with a list, the answered one is reported nowhere while the skipped one is
// still reported, and each generic keeps its own entries.

import (
	"slices"
	"testing"

	"mosaic-deploy/internal/domain"
)

// twoGenericAgents has the first agent declare both generic tools and the second only "bash".
func twoGenericAgents() []domain.Agent {
	return []domain.Agent{toolAgent("test-runner", "bash", "grep"), toolAgent("second-runner", "bash")}
}

func TestCustomToolQuestion_AnsweredListLeavesNoGap_WhileSkippedGenericIsStillReported(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newListFlowRunWith(t, twoGenericAgents(), []string{"bash", "grep"},
				map[string]string{"bash": listFlowAnswer})

			summary := flow.run(t, r)

			assertToolGapReporting(t, r.customToolRun, []string{"bash"}, []string{"grep"})
			assertNotInSummary(t, summary, "bash")
			assertEveryAgentGets(t, r, listFlowEntries)
		})
	}
}

func TestCustomToolQuestion_TwoGenericsAnsweredWithDifferentLists_KeepTheirOwnEntries(t *testing.T) {
	for _, flow := range listFlows {
		t.Run(flow.name, func(t *testing.T) {
			r := newListFlowRunWith(t, twoGenericAgents(), []string{"bash", "grep"},
				map[string]string{"bash": listFlowAnswer, "grep": "search/find, other"})

			flow.run(t, r)

			wantGrep := []domain.MCPEntry{{Server: "search", Tool: "find"}, {Server: "other"}}
			got := r.entriesFor("grep")["test-runner"]
			if len(got) == 0 {
				t.Fatal("agent test-runner received no entries for grep")
			}
			for _, entries := range got {
				if !slices.Equal(entries, wantGrep) {
					t.Errorf("grep entries = %+v, want %+v", entries, wantGrep)
				}
			}
			assertEveryAgentGets(t, r, listFlowEntries)
			assertToolGapReporting(t, r.customToolRun, []string{"bash", "grep"}, nil)
		})
	}
}
