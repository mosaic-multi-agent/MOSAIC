package app_test

// thirdformat_parity_steps_test.go holds the flow steps and the hand-edit helpers of the twin
// parity tests. A hand edit is made on the decoded canonical form and written back through the
// side's translator, so the same edit is made in either format.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

// Sentinels of the user content the tests plant into a deployed agent.
const (
	twinUserKey      = "user_key"
	twinUserValue    = "keep-me"
	twinProjectNote  = "Team rules."
	twinCustomNote   = "My notes."
	twinStaleVersion = "0.0.1"
)

func twinModels() map[string]string {
	m := map[string]string{}
	for _, a := range parityAgents {
		m[a.key] = parityModel
	}
	return m
}

// twinDeployNewStep runs Deploy workspace for the parity workflow, conflicts skipped.
func twinDeployNewStep(t *testing.T, side twinSide) (domain.RunSummary, error) {
	t.Helper()
	return side.Service.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: twinHarnessID, WorkspacePath: side.Workspace, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: twinModels(), AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
}

// twinDeployAgentsStep runs Deploy agents for the one sample subagent, conflicts skipped.
func twinDeployAgentsStep(t *testing.T, side twinSide) (domain.RunSummary, error) {
	t.Helper()
	return side.Service.DeployAgents(context.Background(), app.DeployAgentsRequest{
		HarnessID: twinHarnessID, WorkspacePath: side.Workspace, Scope: domain.ScopeProject,
		SubagentIDs: []string{owRunner}, UtilityAgentIDs: []string{}, InfrastructureAgentIDs: []string{},
		StandaloneAgentIDs: []string{}, AgentModels: twinModels(), AutoConfirmPlan: true,
		ConflictDefault: domain.DecisionSkip})
}

// twinUpdateStep runs Update, conflicts skipped.
func twinUpdateStep(t *testing.T, side twinSide) (domain.RunSummary, error) {
	t.Helper()
	return side.Service.Update(context.Background(), app.UpdateRequest{
		HarnessID: twinHarnessID, WorkspacePath: side.Workspace, AgentModels: twinModels(),
		AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
}

// twinUpdateWorkflowsStep returns the Update workflows step for the parity workflow with the
// given conflict decision.
func twinUpdateWorkflowsStep(decision domain.ConflictDecision) twinStep {
	return func(t *testing.T, side twinSide) (domain.RunSummary, error) {
		t.Helper()
		return side.Service.UpdateWorkflows(context.Background(), app.WorkflowUpdateRequest{
			HarnessID: twinHarnessID, WorkspacePath: side.Workspace, WorkflowIDs: []string{"parity-flow"},
			AutoConfirmPlan: true, ConflictDefault: decision})
	}
}

// twinDeployed deploys the sample catalog to both sides and fails the test on an error.
func twinDeployed(t *testing.T) twinPair {
	t.Helper()
	p := newTestTwinPair(t)
	runTwinStep(t, p, twinDeployNewStep)
	return p
}

// twinSides lists both sides of a pair.
func twinSides(p twinPair) []twinSide { return []twinSide{p.Test, p.Markdown} }

// twinAgentRel is the workspace-relative path of the deployed agent key of a side.
func twinAgentRel(t *testing.T, side twinSide, key string) string {
	t.Helper()
	rel, err := side.Module.TargetPath(domain.TargetPathRequest{Kind: domain.ArtifactAgent, Key: key, Scope: domain.ScopeProject})
	if err != nil {
		t.Fatalf("twin target path of %s: %v", key, err)
	}
	return filepath.ToSlash(rel)
}

func twinTranslator(t *testing.T, side twinSide) agentformat.Translator {
	t.Helper()
	tr, err := agentformat.LookupString(side.Module.Descriptor().AgentFormatID)
	if err != nil {
		t.Fatalf("twin format does not resolve: %v", err)
	}
	return tr
}

// readTwinAgent returns the decoded canonical bytes of the deployed agent key.
func readTwinAgent(t *testing.T, side twinSide, key string) []byte {
	t.Helper()
	raw := gamRead(t, side.Workspace, twinAgentRel(t, side, key))
	canonical, _, err := twinTranslator(t, side).Decode(raw, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
	if err != nil {
		t.Fatalf("decode deployed %s on the %s twin: %v", key, side.name(), err)
	}
	return canonical
}

// editTwinAgent applies edit to the decoded canonical form of the deployed agent key on side and
// writes it back through the side's translator (an update with the current bytes as prior).
func editTwinAgent(t *testing.T, side twinSide, key string, edit func(canonical []byte) []byte) {
	t.Helper()
	rel := twinAgentRel(t, side, key)
	prior := gamRead(t, side.Workspace, rel)
	canonical := readTwinAgent(t, side, key)
	out, _, err := twinTranslator(t, side).Encode(edit(canonical), agentformat.ArtifactContext{
		AgentKey: key, Kind: domain.ArtifactAgent, Op: agentformat.OpUpdate, PriorDeployed: prior})
	if err != nil {
		t.Fatalf("encode edited %s on the %s twin: %v", key, side.name(), err)
	}
	gamWrite(t, side.Workspace, rel, string(out))
}

// writeTwinAgent creates the agent key from canonical bytes through the side's translator.
func writeTwinAgent(t *testing.T, side twinSide, key string, canonical []byte) {
	t.Helper()
	out, _, err := twinTranslator(t, side).Encode(canonical, agentformat.ArtifactContext{
		AgentKey: key, Kind: domain.ArtifactAgent, Op: agentformat.OpCreate})
	if err != nil {
		t.Fatalf("encode new %s on the %s twin: %v", key, side.name(), err)
	}
	gamWrite(t, side.Workspace, twinAgentRel(t, side, key), string(out))
}

// corruptTwinAgent replaces the deployed agent key with bytes its format cannot use as an agent
// document: invalid JSON for the test format, a file without frontmatter for Markdown.
func corruptTwinAgent(t *testing.T, side twinSide, key string) {
	t.Helper()
	content := "no frontmatter in this file\n"
	if side.Format == twinTestFormat {
		content = "{ this is not valid json"
	}
	gamWrite(t, side.Workspace, twinAgentRel(t, side, key), content)
}

// twinReRecord records the current file of an agent in the manifest (hash and owned record), so
// the file is again the file the manifest describes.
func twinReRecord(t *testing.T, side twinSide, key string) {
	t.Helper()
	rel := twinAgentRel(t, side, key)
	rec, err := transform.OwnedContentOf(domain.ArtifactAgent, readTwinAgent(t, side, key))
	if err != nil {
		t.Fatalf("owned record of %s: %v", key, err)
	}
	m := gamManifest(t, side.Workspace)
	for i, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == key && filepath.ToSlash(e.TargetPath) == rel {
			m.Entries[i].ContentHash = manifest.Hash(gamRead(t, side.Workspace, rel))
			m.Entries[i].Owned = &rec
		}
	}
	gamSaveManifest(t, side.Workspace, m)
}

// twinSetField returns an edit that sets one frontmatter key of canonical bytes (adding it
// after the opening delimiter when absent).
func twinSetField(field, value string) func([]byte) []byte {
	return func(canonical []byte) []byte {
		lines := strings.Split(string(canonical), "\n")
		for i, l := range lines {
			if i > 0 && strings.HasPrefix(l, field+":") {
				lines[i] = field + ": " + value
				return []byte(strings.Join(lines, "\n"))
			}
		}
		lines = append(lines[:1], append([]string{field + ": " + value}, lines[1:]...)...)
		return []byte(strings.Join(lines, "\n"))
	}
}

// twinBodyEdit lifts a string edit of the canonical document into a byte edit.
func twinBodyEdit(edit func(string) string) func([]byte) []byte {
	return func(canonical []byte) []byte { return []byte(edit(string(canonical))) }
}

// twinChain applies the edits in order.
func twinChain(edits ...func([]byte) []byte) func([]byte) []byte {
	return func(b []byte) []byte {
		for _, e := range edits {
			b = e(b)
		}
		return b
	}
}

// twinPlantUserContent is the edit that adds a user-owned frontmatter key, a project region
// line and a custom region to the canonical document.
func twinPlantUserContent(t *testing.T) func([]byte) []byte {
	return twinChain(twinBodyEdit(owProjectEdit(t)), twinBodyEdit(owCustomEdit(t)), twinSetField(twinUserKey, twinUserValue))
}

// twinAgeVersion is the edit that makes the deployed agent compare as stale.
func twinAgeVersion() func([]byte) []byte { return twinSetField("mosaic_version", twinStaleVersion) }

// twinItem returns the plan item of the agent key in an observation.
func twinItem(t *testing.T, obs twinObservation, key string) twinPlanItem {
	t.Helper()
	for _, it := range obs.Items {
		if it.Ref.Kind == domain.ArtifactAgent && it.Ref.Key == key {
			return it
		}
	}
	t.Fatalf("plan has no item for agent %s", key)
	return twinPlanItem{}
}

// twinRequireUserContent fails unless the decoded agent still holds the planted user content.
func twinRequireUserContent(t *testing.T, label string, obs twinObservation, key string) {
	t.Helper()
	a, ok := obs.Agents[key]
	if !ok {
		t.Errorf("%s: agent %s is not deployed after the step", label, key)
		return
	}
	for _, want := range []string{twinProjectNote, twinCustomNote} {
		if !strings.Contains(a.Body, want) {
			t.Errorf("%s: agent %s lost the user content %q", label, key, want)
		}
	}
	if got := a.Frontmatter[twinUserKey]; got != twinUserValue {
		t.Errorf("%s: user-owned key %s = %q after the step; want %q", label, twinUserKey, got, twinUserValue)
	}
}

// twinRemoveField returns an edit that removes one frontmatter key of canonical bytes.
func twinRemoveField(field string) func([]byte) []byte {
	return func(canonical []byte) []byte {
		lines := strings.Split(string(canonical), "\n")
		out := make([]string, 0, len(lines))
		for i, l := range lines {
			if i > 0 && strings.HasPrefix(l, field+":") {
				continue
			}
			out = append(out, l)
		}
		return []byte(strings.Join(out, "\n"))
	}
}
