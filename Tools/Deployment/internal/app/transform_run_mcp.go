package app

// transform_run_mcp.go asks how the ambiguous MCP tool names of a transform run split into
// server and tool, for the names that the tool mappings and the declared servers of the source
// harness leave open.

import (
	"context"

	"mosaic-deploy/internal/domain"
)

// openCodeHarnessID is the descriptor id of the harness whose configuration declares MCP servers.
const openCodeHarnessID = "opencode"

// openCodeDeclaredServers returns the servers OpenCode's configuration declares when the source
// harness is OpenCode; any other source has none.
func (r *transformRun) openCodeDeclaredServers() []string {
	if r.src == nil || r.src.Descriptor().ID != openCodeHarnessID {
		return nil
	}
	return readOpenCodeDeclaredServers(r.workspace)
}

// resolveMCPSplits asks QMCPNameSplit once per distinct ambiguous name over the agents that will
// be written. A pre-answer suppresses the question; an answer names the server for every agent;
// a skip leaves the name unmapped. A cancelled or failed question aborts the run.
func (r *transformRun) resolveMCPSplits(ctx context.Context) error {
	r.mcpSplits = map[string]string{}
	for name, server := range r.req.MCPSplits {
		r.mcpSplits[name] = server
	}
	skippedAll := r.req.SkipAll[domain.QMCPNameSplit]
	asked := map[string]bool{}
	for i := range r.agents {
		if !isWrite(r.plan.Agents[i]) || !r.agents[i].active() {
			continue
		}
		for _, amb := range r.inspections[i].Converted.Ambiguous {
			if asked[amb.Source] {
				continue
			}
			asked[amb.Source] = true
			if _, preAnswered := r.req.MCPSplits[amb.Source]; preAnswered || skippedAll {
				continue
			}
			server, status, err := r.askMCPSplit(ctx, amb)
			if err != nil {
				return err
			}
			if status == domain.SkippedAll {
				skippedAll = true
			}
			if server != "" {
				r.mcpSplits[amb.Source] = server
			}
		}
	}
	return nil
}

// askMCPSplit asks the split question of one name and returns the chosen server, "" when the
// question was skipped.
func (r *transformRun) askMCPSplit(ctx context.Context, amb AmbiguousName) (string, domain.AnswerStatus, error) {
	options := make([]domain.Option, len(amb.Splits))
	for i, split := range amb.Splits {
		options[i] = domain.Option{ID: split.Server, Label: split.Server + " / " + split.Tool}
	}
	ans, err := r.s.deps.Interaction.SelectOne(ctx, domain.ChoiceQuestion{
		Question: domain.Question{
			ID: domain.QMCPNameSplit, Subject: amb.Source,
			Title:           "Which MCP server provides " + amb.Source + "?",
			Prompt:          "The name can be read as more than one server and tool. Choose the server.",
			AllowSkip:       true,
			AllowSkipAll:    true,
			DefaultOptionID: options[0].ID,
		},
		Options: options,
	})
	if err != nil || ans.Status == domain.Cancelled {
		return "", ans.Status, ErrSelectionCancelled
	}
	if ans.Status == domain.Answered && ans.OptionID != "" {
		return ans.OptionID, ans.Status, nil
	}
	return "", ans.Status, nil
}
