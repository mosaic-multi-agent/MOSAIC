package app

// transform_run_result.go builds the result of a transform run from the plan and the builds.

import (
	"path/filepath"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// newerInjectionsMessage tells the user that content owned by the target harness or the catalog
// may be newer than the source.
const newerInjectionsMessage = "Hooks and harness injections come from the target harness and the catalog and may be newer than the source."

// agentOutcome is the outcome of one planned agent.
func (r *transformRun) agentOutcome(i int) TransformFileOutcome {
	pa, st := r.plan.Agents[i], r.agents[i]
	out := TransformFileOutcome{
		Kind: domain.ArtifactAgent, Key: pa.Key, SourcePath: pa.SourcePath,
		DestinationPath: filepath.ToSlash(pa.DestinationPath), Action: pa.Action,
	}
	switch {
	case pa.Action == PlanSkip:
		out.Status, out.Skip, out.Reason = StatusSkipped, pa.Skip, pa.Reason
	case !st.active():
		out.Status, out.Reason = StatusFailed, st.failReason
	default:
		out.Status = StatusTransformed
		res := st.result
		out.Role, out.Model = res.Role, r.groupModels[pa.SourceModel]
		out.Tools, out.DroppedTools = res.Tools, res.Dropped
		out.UnmappedTools, out.DroppedFields = res.UnmappedTools, res.DroppedFields
		out.CustomTools = r.customToolServers(i)
	}
	return out
}

// customToolServers lists the servers kept as custom tools in a rebuilt agent, each once.
func (r *transformRun) customToolServers(i int) []string {
	var servers []string
	for _, c := range r.agents[i].result.Conversions {
		if c.Server != "" {
			servers = append(servers, c.Server)
		}
	}
	for _, generic := range r.inspections[i].Converted.Unmapped {
		if server, ok := r.customTools[generic]; ok {
			servers = append(servers, server)
		}
	}
	return uniqueStrings(servers)
}

// result builds the report of the run: the outcome of every planned item, the counts, the
// model summary and the notes. todoPath is empty when no TODO file was written.
func (r *transformRun) result(todoPath string) TransformHarnessResult {
	res := r.identifiers()
	res.DryRun = r.req.DryRun
	res.TodoPath = todoPath
	res.Files = []TransformFileOutcome{}
	for i := range r.plan.Agents {
		out := r.agentOutcome(i)
		if out.Status == StatusTransformed && out.Model == "" {
			res.UnresolvedModels = append(res.UnresolvedModels, out.Key)
		}
		res.add(out)
	}
	for i := range r.plan.Skills {
		res.add(r.skillOutcome(i))
	}
	for i := range r.plan.Hooks {
		res.add(r.hookOutcome(i))
	}
	for _, x := range r.plan.Removals {
		res.Removals = append(res.Removals, newFileChange(x.Ref, x.Path, x.Reason))
	}
	for _, x := range r.plan.Replacements {
		res.Replacements = append(res.Replacements, newFileChange(x.Ref, x.Path, x.Reason))
	}
	res.DroppedEntries = r.droppedEntryResults()
	res.Notes = append([]TransformNote(nil), r.plan.Notes...)
	if res.Transformed > 0 {
		res.Notes = append(res.Notes, TransformNote{Kind: NoteNewerTargetContent, Message: newerInjectionsMessage})
	}
	r.addModelSummary(&res)
	return res
}

// newFileChange is the result entry of one planned removal or replacement.
func newFileChange(ref domain.ArtifactRef, p, reason string) TransformFileChange {
	return TransformFileChange{Kind: ref.Kind, Key: ref.Key, Path: filepath.ToSlash(p), Reason: reason}
}

// recordBackups sets the backup path of every removal and replacement the executor backed up.
func (res *TransformHarnessResult) recordBackups(exec deploy.ExecResult) {
	for _, rec := range exec.Removals {
		setBackup(res.Removals, rec.Removal.Path, rec.BackupPath)
	}
	for _, rec := range exec.Replacements {
		setBackup(res.Replacements, rec.Replacement.Path, rec.BackupPath)
	}
}

// setBackup sets the backup path of the change at p.
func setBackup(changes []TransformFileChange, p, backup string) {
	for i := range changes {
		if changes[i].Path == filepath.ToSlash(p) {
			changes[i].BackupPath = backup
		}
	}
}

// add appends an outcome to the result and counts it by status.
func (res *TransformHarnessResult) add(out TransformFileOutcome) {
	res.Files = append(res.Files, out)
	switch out.Status {
	case StatusTransformed:
		res.Transformed++
	case StatusSkipped:
		res.Skipped++
	case StatusFailed:
		res.Failed++
	}
}

// addModelSummary sets AppliedModelMap and TargetModel from the resolved groups.
func (r *transformRun) addModelSummary(res *TransformHarnessResult) {
	if len(r.groupModels) == 0 {
		return
	}
	res.AppliedModelMap = r.groupModels
	distinct := map[string]bool{}
	for _, m := range r.groupModels {
		if m != "" {
			distinct[m] = true
		}
	}
	if len(distinct) == 1 {
		for m := range distinct {
			res.TargetModel = m
		}
	}
}

// identifiers is the result of a run that stopped early: the harness ids and workspace known
// so far, and where transform looked once the plan exists.
func (r *transformRun) identifiers() TransformHarnessResult {
	return TransformHarnessResult{
		SourceHarnessID: r.srcID, TargetHarnessID: r.tgtID, Workspace: r.workspace,
		SourceAgentsDir: r.plan.SourceAgentsDir,
	}
}
