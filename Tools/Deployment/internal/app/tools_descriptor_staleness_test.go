package app_test

// tools_descriptor_staleness_test.go verifies that a change of a harness's tool settings reaches
// the deployed agent on the next update whether it comes from the descriptor shipped with a
// release or from the configuration, also when the agent's catalog source did not change, and
// that an update without such a change leaves the agent alone. The agent is deployed with the
// descriptor of the release before, then updated with the descriptor of the release after.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

func TestUpdate_DescriptorDropsToolMapping_AgentUpdatedAndToolRemoved(t *testing.T) {
	env := newTdEnv(t)
	env.deploy(tdSpecBefore())
	if !strings.Contains(toolsBlock(env.written()), tdRemovedTool) {
		t.Fatalf("setup: the deployed tools line %q does not hold %s", toolsBlock(env.written()), tdRemovedTool)
	}

	run := env.update(tdSpecAfter())

	run.requireAction(t, domain.ActionUpdate)
	written := env.written()
	if strings.Contains(written, tdRemovedTool) {
		t.Errorf("the rewritten agent still mentions %s:\n%s", tdRemovedTool, written)
	}
	line := toolsBlock(written)
	for _, want := range []string{"Read", "Edit"} {
		if !strings.Contains(line, want) {
			t.Errorf("the rewritten tools line %q lost %s", line, want)
		}
	}
}

func TestUpdate_DescriptorDropsToolMapping_PlanNamesToolMappingsVersionChange(t *testing.T) {
	env := newTdEnv(t)
	env.deploy(tdSpecBefore())

	run := env.update(tdSpecAfter())

	item := run.requireAction(t, domain.ActionUpdate)
	if !strings.Contains(strings.Join(item.Reasons, "; "), tdReasonStamp) {
		t.Errorf("reasons %q do not name the tool_mappings_version change", item.Reasons)
	}
	if got := reasonWithPrefix(item, reasonRemovedPrefix); got != reasonRemovedPrefix+tdRemovedTool {
		t.Errorf("removed reason = %q, want %q; reasons: %q", got, reasonRemovedPrefix+tdRemovedTool, item.Reasons)
	}
}

func TestUpdate_DescriptorDropsToolMapping_AgentRecordsNewStampAndNextUpdateIsClean(t *testing.T) {
	env := newTdEnv(t)
	env.deploy(tdSpecBefore())
	before := tdStampOf(env.written())

	env.update(tdSpecAfter())
	after := tdStampOf(env.written())
	next := env.update(tdSpecAfter())

	if after == "" || after == before {
		t.Errorf("stamp after the update = %q, before = %q; want a new non-empty stamp", after, before)
	}
	next.requireAction(t, domain.ActionUnchanged)
}

// tdSettingChanges are descriptor changes that leave the rendered tools field of the agent as it
// is but change the tool settings of the harness.
var tdSettingChanges = map[string]func(*domain.ToolSpec){
	"unused disposition of a universe tool": func(s *domain.ToolSpec) { s.Universe[0].Unused = domain.Deny },
	"placeholder expansion":                 func(s *domain.ToolSpec) { s.PlaceholderExpansion = []string{"Read"} },
	"mcp name form":                         func(s *domain.ToolSpec) { s.MCP.ServerForm = "mcp__{server}__*" },
	"custom tool destinations": func(s *domain.ToolSpec) {
		s.CustomToolDestinations = []domain.ToolDestination{{Kind: domain.DestField, Field: "custom_tools"}}
	},
	"report unknown names": func(s *domain.ToolSpec) { s.ReportUnknownNames = true },
}

func TestUpdate_AnyDescriptorToolSettingChanges_MarksAgentUpdated(t *testing.T) {
	for name, change := range tdSettingChanges {
		t.Run(name, func(t *testing.T) {
			env := newTdEnv(t)
			env.deploy(tdSpecAfter())
			changed := tdSpecAfter()
			change(&changed)

			run := env.update(changed)

			item := run.requireAction(t, domain.ActionUpdate)
			if !strings.Contains(strings.Join(item.Reasons, "; "), tdReasonStamp) {
				t.Errorf("reasons %q do not name the tool_mappings_version change", item.Reasons)
			}
		})
	}
}

func TestUpdate_ConfigToolDestinationChanges_MarksAgentUpdatedWithSameKindOfReason(t *testing.T) {
	env := newTdEnv(t)
	env.deploy(tdSpecAfter())
	env.setUserDestinations(tdMain("user_interaction", "Ask"))

	run := env.update(tdSpecAfter())

	item := run.requireAction(t, domain.ActionUpdate)
	if !strings.Contains(strings.Join(item.Reasons, "; "), tdReasonStamp) {
		t.Errorf("reasons %q do not name the tool_mappings_version change for a config-caused change", item.Reasons)
	}
}

func TestUpdate_ConfigToolDestinationRemoved_MarksAgentUpdated(t *testing.T) {
	env := newTdEnv(t)
	env.setUserDestinations(tdMain("user_interaction", "Ask"))
	env.deploy(tdSpecAfter())
	env.setUserDestinations()

	run := env.update(tdSpecAfter())

	run.requireAction(t, domain.ActionUpdate)
}

func TestUpdate_NoChangeFromEitherSource_SecondUpdateLeavesAgentUnchanged(t *testing.T) {
	env := newTdEnv(t)
	env.setUserDestinations(tdMain("web", "fetch"))
	env.deploy(tdSpecAfter())
	deployed := env.written()

	first := env.update(tdSpecAfter())
	second := env.update(tdSpecAfter())

	first.requireAction(t, domain.ActionUnchanged)
	second.requireAction(t, domain.ActionUnchanged)
	if env.written() != deployed {
		t.Error("an update without a change of tool settings rewrote the agent")
	}
}

func TestUpdate_FirstUpdateAfterChange_ThenTwoConsecutiveUpdatesAreClean(t *testing.T) {
	env := newTdEnv(t)
	env.deploy(tdSpecBefore())

	env.update(tdSpecAfter())
	second := env.update(tdSpecAfter())
	third := env.update(tdSpecAfter())

	second.requireAction(t, domain.ActionUnchanged)
	third.requireAction(t, domain.ActionUnchanged)
}
