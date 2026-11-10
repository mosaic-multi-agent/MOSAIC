package app

// transform_run_build.go prepares and builds the agents of a transform run: it reads each
// source file, inspects it for the tools the target cannot map, asks the custom-tool
// questions and rebuilds every agent in memory before anything is written.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
)

// agentState is the working state of one planned agent during a run.
type agentState struct {
	// input is the rebuild input; Source is set once the file has been read.
	input RebuildInput
	// failReason is set when the file could not be read, inspected or rebuilt.
	failReason string
	built      bool
	result     RebuildResult
}

// active reports whether the agent is still part of the write set.
func (a *agentState) active() bool { return a.failReason == "" }

// isWrite reports whether the plan writes this agent.
func isWrite(a PlannedAgent) bool {
	return a.Action == PlanWriteCreate || a.Action == PlanWriteOverwrite
}

// catalogLookup is the catalog cross-check handed to the rebuild; nil when the service has no catalog.
func (r *transformRun) catalogLookup() CatalogAgentLookup {
	if r.s.deps.Catalog == nil {
		return nil
	}
	return r.s.deps.Catalog
}

// prepareAgents reads every agent the plan writes and inspects it. A file that cannot be read
// or inspected is marked failed and leaves the write set; it contributes no question.
func (r *transformRun) prepareAgents() {
	r.agents = make([]agentState, len(r.plan.Agents))
	r.inspections = make([]DeployedAgentInspection, len(r.plan.Agents))
	timestamp := r.s.now().UTC().Format(time.RFC3339)
	for i, pa := range r.plan.Agents {
		if !isWrite(pa) {
			continue
		}
		st := &r.agents[i]
		src, err := os.ReadFile(pa.SourcePath)
		if err != nil {
			st.failReason = fmt.Sprintf("cannot read file: %v", err)
			continue
		}
		st.input = RebuildInput{
			Source: src, FileName: pa.FileName, AgentKey: pa.Key,
			SourceModule: r.src, TargetModule: r.tgt, Catalog: r.catalogLookup(),
			Model:               r.modelSelection(pa.SourceModel),
			ToolMappingsVersion: r.toolMappingsVersion, Timestamp: timestamp,
		}
		r.setWriteOp(st, pa)
		insp, err := InspectDeployedAgent(st.input)
		if err != nil {
			st.failReason = err.Error()
			continue
		}
		r.inspections[i] = insp
	}
}

// setWriteOp sets the explicit operation of the rebuild from the planned write action and, on
// setWriteOp sets the explicit operation of the rebuild from the planned write action and, on
// overwrite, the destination file's raw bytes. The destination is read through the decode
// funnel with the target descriptor. A file that cannot be read or decoded is passed on
// without a decode result: the rebuild reports an undecodable destination, and an unreadable
// one leaves the prior bytes missing so the encoder reports it.
func (r *transformRun) setWriteOp(st *agentState, pa PlannedAgent) {
	if pa.Action != PlanWriteOverwrite {
		st.input.Op = agentformat.OpCreate
		return
	}
	st.input.Op = agentformat.OpUpdate
	dest := filepath.Join(r.workspace, filepath.FromSlash(pa.DestinationPath))
	if read := readDeployedArtifact(dest, domain.ArtifactAgent, r.tgt.Descriptor()); read.Present && read.ReadErr == nil {
		st.input.DestinationRaw = read.Raw
	}
}

// resolveCustomTools asks QCustomTool once per generic tool the target cannot map, over the
// agents that will be written. An answer names the server for every agent; a skip leaves the
// tool unmapped. A cancelled or failed question aborts the run.
func (r *transformRun) resolveCustomTools(ctx context.Context) error {
	r.customTools = map[string]string{}
	r.skippedTools = map[string]bool{}
	var order []string
	seen := map[string]bool{}
	for i := range r.agents {
		if !isWrite(r.plan.Agents[i]) || !r.agents[i].active() {
			continue
		}
		for _, generic := range r.inspections[i].Converted.Unmapped {
			if !seen[generic] {
				seen[generic] = true
				order = append(order, generic)
			}
		}
	}
	skippedAll := r.req.SkipAll[domain.QCustomTool]
	for _, generic := range order {
		if skippedAll {
			r.skippedTools[generic] = true
			continue
		}
		ans, err := r.s.deps.Interaction.AskText(ctx, domain.TextQuestion{
			Question: domain.Question{
				ID: domain.QCustomTool, Subject: generic,
				Title:        "Custom MCP server name for " + generic,
				Prompt:       "Enter the bare MCP server name that provides this tool -- not a specific tool name within the server.",
				Detail:       "Example: enter github, not github_get_me",
				AllowSkip:    true,
				AllowSkipAll: true,
			},
		})
		if err != nil || ans.Status == domain.Cancelled {
			return ErrSelectionCancelled
		}
		switch {
		case ans.Status == domain.SkippedAll:
			skippedAll = true
			r.skippedTools[generic] = true
		case ans.Status == domain.Answered && ans.Text != "":
			r.customTools[generic] = ans.Text
		default:
			r.skippedTools[generic] = true
		}
	}
	return nil
}

// buildAgents rebuilds every agent still in the write set. A build error marks that file
// failed and leaves the others untouched.
func (r *transformRun) buildAgents() {
	for i := range r.agents {
		st := &r.agents[i]
		if !isWrite(r.plan.Agents[i]) || !st.active() {
			continue
		}
		st.input.CustomTools = r.customTools
		if len(r.skippedTools) > 0 {
			st.input.SkippedTools = r.skippedTools
		}
		res, err := RebuildDeployedAgent(st.input)
		if err != nil {
			st.failReason = err.Error()
			if errors.Is(err, agentformat.ErrMalformedDeployed) {
				st.failReason = fmt.Sprintf("existing destination %s: %v", filepath.ToSlash(r.plan.Agents[i].DestinationPath), err)
			}
			continue
		}
		st.built, st.result = true, res
	}
}

// writeCount is the number of agents, skills and hook bundles that will be written.
func (r *transformRun) writeCount() int {
	n := r.skillWrites() + r.hookWrites()
	for i := range r.agents {
		if isWrite(r.plan.Agents[i]) && r.agents[i].active() {
			n++
		}
	}
	return n
}
