package app

import (
	"context"

	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

// customToolQuestion builds the question that asks which MCP entries provide a generic tool
// the target cannot map. Deploy flows and transform share it, so both teach the same syntax
// and enforce the same rules.
func customToolQuestion(generic string) domain.TextQuestion {
	return domain.TextQuestion{
		Question: domain.Question{
			ID: domain.QCustomTool, Subject: generic,
			Title: "Custom MCP tools for " + generic,
			Prompt: `Enter one or more MCP entries, separated by commas. ` +
				`"server" grants every tool of that server; "server/tool" grants a single tool.`,
			Detail:       "Example: github/get_pr, github/list_comments, jira",
			AllowSkip:    true,
			AllowSkipAll: true,
		},
		Validate: customtool.Validate,
	}
}

// customToolEntries reads an answered custom tool text into entries. A blank or unparseable
// text gives none, which callers treat as a skip.
func customToolEntries(text string) []domain.MCPEntry {
	entries, err := customtool.Parse(text)
	if err != nil {
		return nil
	}
	return entries
}

// resolveCustomTools discovers every generic tool with no harness mapping across the given
// agents, asks QCustomTool once per distinct unmapped tool (honouring pre-answers, skip, and
// skip-all), and returns the resolved custom entries plus the set of tools the user skipped.
// Every skipped tool produces a GapUnmappedTool entry (AC18.5). Each answered tool is offered
// for saving on harnessID right after its answer.
func (s *service) resolveCustomTools(
	ctx context.Context,
	harnessID string,
	preAnswered domain.CustomToolAnswers,
	skipAll map[domain.QuestionID]bool,
	module domain.HarnessModule,
	agents []domain.Agent,
) (domain.CustomToolAnswers, map[string]bool) {
	customNames := make(domain.CustomToolAnswers, len(preAnswered))
	for k, v := range preAnswered {
		customNames[k] = v
	}
	skipped := make(map[string]bool)

	var unmappedOrder []string
	declaredBy := make(map[string]domain.Agent) // first agent declaring each unmapped tool
	seen := make(map[string]bool)
	for _, agent := range agents {
		if len(agent.Tools) == 0 {
			continue
		}
		result, err := module.Tools(domain.ToolRequest{
			AgentKey: agent.Key, Generic: agent.Tools, Placeholder: agent.ToolsPlaceholder,
		})
		if err != nil {
			continue
		}
		for _, res := range result.Resolutions {
			if res.Outcome == domain.ToolUnmapped && !seen[res.Generic] {
				seen[res.Generic] = true
				declaredBy[res.Generic] = agent
				unmappedOrder = append(unmappedOrder, res.Generic)
			}
		}
	}

	offers := s.newSaveOffer(harnessID, module)
	defer offers.finish(ctx)

	skippedAll := skipAll[domain.QCustomTool]
	for _, toolName := range unmappedOrder {
		if _, ok := customNames[toolName]; ok {
			continue // pre-answered
		}
		if skippedAll {
			skipped[toolName] = true
			continue
		}
		ans, err := s.deps.Interaction.AskText(ctx, customToolQuestion(toolName))
		if err != nil {
			skipped[toolName] = true
			continue
		}
		switch ans.Status {
		case domain.SkippedAll:
			skippedAll = true
			skipped[toolName] = true
		case domain.Answered:
			if entries := customToolEntries(ans.Text); len(entries) > 0 {
				customNames[toolName] = entries
				agent := declaredBy[toolName]
				offers.offer(ctx, toolName, agent.Key, agent.ToolsPlaceholder, entries)
			} else {
				skipped[toolName] = true
			}
		default:
			skipped[toolName] = true
		}
	}

	// Re-resolve with the final custom-name and skip maps so the harness module records the
	// propagated custom names (used by content generation during deployment).
	for _, agent := range agents {
		if len(agent.Tools) == 0 {
			continue
		}
		_, _ = module.Tools(domain.ToolRequest{
			AgentKey: agent.Key, Generic: agent.Tools, Placeholder: agent.ToolsPlaceholder,
			CustomNames: customNames, SkippedTools: skipped,
		})
	}

	// Unmapped-tool gaps are emitted solely by plan.Build, which names the owning agent and
	// also fires in the update flow. Emitting them here would produce duplicates.

	return customNames, skipped
}
