package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mosaic-run/internal/artifact"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/engine"
)

// buildInfraRequest builds the request of a trigger dispatch of agent.
//
// A commit-class agent whose matched triggers include ITERATION_END receives
// the ended iteration's own artifacts that exist in the run folder, and a task
// description naming the block and iteration ID. Every other dispatch carries
// no input artifacts and a description naming the matched triggers. Output
// artifacts stay empty and the human is never in the loop. Resolution never
// fails the dispatch: when the iteration or its artifacts cannot be
// determined, the request is built as for any other dispatch.
func buildInfraRequest(
	agent domain.DeclaredInfraAgent,
	matched []string,
	instanceID string,
	runID string,
	runFolder string,
	completedStep domain.CompletedStep,
	boundaries triggerBoundaries,
) domain.ProtocolRequest {
	req := domain.ProtocolRequest{
		AgentInstanceID: instanceID,
		RunID:           runID,
		TaskDescription: fmt.Sprintf("infrastructure agent dispatch: %s, triggered by %s",
			agent.Name, strings.Join(matched, ", ")),
	}
	if agent.Class != "commit" || !containsTrigger(matched, domain.InfraTriggerIterationEnd) {
		return req
	}
	block, iteration, ok := endedIteration(completedStep, boundaries)
	if !ok {
		return req
	}
	candidates, err := engine.IterationArtifactCandidates(boundaries.Workflow.Table, block, iteration)
	if err != nil {
		return req
	}
	req.InputArtifacts = existingIterationArtifacts(runFolder, runID, candidates)
	req.TaskDescription = fmt.Sprintf("infrastructure agent dispatch: %s, commit the artifacts of %s %s (triggered by %s)",
		agent.Name, block.Name, iteration, strings.Join(matched, ", "))
	return req
}

// endedIteration resolves the block enclosing the completed step's row and the
// iteration ID recorded for it (group suffix dropped).
func endedIteration(step domain.CompletedStep, b triggerBoundaries) (domain.AdmittedBlock, domain.IterationID, bool) {
	block, ok := b.Workflow.BlockOf(b.RowIndex)
	if !ok {
		return domain.AdmittedBlock{}, "", false
	}
	value, err := b.readValue(step.Stage)
	if err != nil || value.ID == "" {
		return domain.AdmittedBlock{}, "", false
	}
	return block, value.ID, true
}

// existingIterationArtifacts expands wildcard candidates against the run
// folder, keeps the concrete candidates that exist there, and returns them
// run-scoped, in candidate order, each path once.
func existingIterationArtifacts(runFolder, runID string, candidates []string) []string {
	prefix := domain.RunScopedFolder(runID) + "/"
	fsys := os.DirFS(filepath.Dir(runFolder))
	var found []string
	for _, c := range candidates {
		scoped := resolveToRunScoped([]string{c}, prefix)[0]
		if strings.Contains(c, "*") {
			found = append(found, artifact.ExpandWildcard(fsys, scoped)...)
			continue
		}
		rel := strings.TrimPrefix(scoped, prefix)
		if fileExists(filepath.Join(runFolder, filepath.FromSlash(rel))) {
			found = append(found, scoped)
		}
	}
	return domain.DedupArtifactPaths(runID, found)
}

// containsTrigger reports whether name is among triggers.
func containsTrigger(triggers []string, name string) bool {
	for _, t := range triggers {
		if t == name {
			return true
		}
	}
	return false
}
