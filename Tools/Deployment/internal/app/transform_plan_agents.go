package app

// transform_plan_agents.go decides the destination and action of every agent file of the
// source agents folder: skip precedence, destination naming and the both-forms warning.

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/domain"
)

// slashJoin joins a folder and a name into a slash path.
func slashJoin(dir, name string) string {
	return path.Join(filepath.ToSlash(dir), name)
}

// sameSlashPath compares two workspace-relative paths after cleaning, ignoring case.
func sameSlashPath(a, b string) bool {
	return strings.EqualFold(path.Clean(filepath.ToSlash(a)), path.Clean(filepath.ToSlash(b)))
}

// planAgents fills plan.Agents, plan.SourceAgentsDir and the agent-related notes.
func (s *service) planAgents(in transformPlanInput, plan *TransformPlan) error {
	srcDesc, tgtDesc := in.Source.Descriptor(), in.Target.Descriptor()
	relDir := srcDesc.Paths.Agents.Project
	plan.SourceAgentsDir = filepath.Join(in.Workspace, filepath.FromSlash(relDir))
	files, sourceFiles, err := listSourceAgentFiles(plan.SourceAgentsDir, relDir, in.Source, inputIsGhcpPair(in))
	if err != nil {
		return err
	}
	if inputIsGhcpPair(in) {
		return s.planPairAgents(in, relDir, files, plan)
	}

	keeper := duplicateKeyKeepers(files, srcDesc.Extensions[domain.ArtifactAgent])
	sameFolder := sameSlashPath(relDir, tgtDesc.Paths.Agents.Project)
	srcExt, tgtExt := srcDesc.Extensions[domain.ArtifactAgent], tgtDesc.Extensions[domain.ArtifactAgent]

	recognised := 0
	for _, f := range files {
		a := PlannedAgent{SourcePath: f.path, FileName: f.name}
		switch {
		case !f.recognised:
			a.Action, a.Skip, a.Reason = PlanSkip, SkipNotAgent, "not a MOSAIC agent: "+f.reason
			plan.Agents = append(plan.Agents, a)
			continue
		}
		recognised++
		a.Key = agentkey.FromFileName(f.name)
		a.SourceModel = f.model
		switch {
		case sameFolder && tgtExt != srcExt && fileAgentExt(f.name) == tgtExt:
			a.Action, a.Skip = PlanSkip, SkipTargetExtension
			a.Reason = "name already has the target extension " + tgtExt + " in the shared agents folder " + relDir
		case keeper[a.Key] != f.name:
			a.Action, a.Skip = PlanSkip, SkipDuplicateKey
			a.Reason = "another file (" + keeper[a.Key] + ") is the source of agent " + a.Key
		default:
			s.planAgentDestination(in, sourceFiles, &a)
		}
		plan.Agents = append(plan.Agents, a)
	}

	if recognised == 0 {
		plan.Notes = append(plan.Notes, TransformNote{
			Kind:    NoteNoAgentsFound,
			Message: "no MOSAIC agents found in " + plan.SourceAgentsDir,
			Paths:   []string{plan.SourceAgentsDir},
		})
	}
	plan.Notes = append(plan.Notes, bothFormsNotes(in.Workspace, plan.Agents, nil)...)
	return nil
}

// duplicateKeyKeepers maps every agent key to the file that stays the source of it: the file
// whose extension equals srcExt, otherwise the first by name. files are in name order.
func duplicateKeyKeepers(files []sourceAgentFile, srcExt string) map[string]string {
	keeper := map[string]string{}
	for _, f := range files {
		if !f.recognised {
			continue
		}
		key := agentkey.FromFileName(f.name)
		cur, seen := keeper[key]
		if !seen || (fileAgentExt(cur) != srcExt && fileAgentExt(f.name) == srcExt) {
			keeper[key] = f.name
		}
	}
	return keeper
}

// planAgentDestination resolves the destination of a kept source agent and its action.
func (s *service) planAgentDestination(in transformPlanInput, sourceFiles map[string]bool, a *PlannedAgent) {
	dest, err := in.Target.TargetPath(domain.TargetPathRequest{Kind: domain.ArtifactAgent, Key: a.Key, Scope: domain.ScopeProject})
	if err != nil {
		a.Action, a.Skip, a.Reason = PlanSkip, SkipUnsupported, "target harness cannot place this agent: "+err.Error()
		return
	}
	a.DestinationPath = dest
	if sourceFiles[strings.ToLower(path.Clean(filepath.ToSlash(dest)))] {
		a.Action, a.Skip = PlanSkip, SkipDestinationIsSource
		a.Reason = "destination " + filepath.ToSlash(dest) + " is a source file and is never written"
		return
	}
	exists := pathExists(filepath.Join(in.Workspace, filepath.FromSlash(dest)))
	switch {
	case exists && !in.Overwrite:
		a.Action, a.Skip = PlanSkip, SkipDestinationExists
		a.Reason = "destination " + filepath.ToSlash(dest) + " already exists and is kept unchanged"
	case exists:
		a.Action = PlanWriteOverwrite
	default:
		a.Action = PlanWriteCreate
	}
}

// pathExists reports whether p exists.
func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// bothFormsNotes warns, per destination folder and agent stem, when the result would hold
// both X.md and X.agent.md with at least one of them a planned write. Stems compare
// case-insensitively, as Runner's agent resolution does.
func bothFormsNotes(workspace string, agents []PlannedAgent, omit map[string]bool) []TransformNote {
	written := map[string]bool{} // lower-cased slash destination
	folders := map[string]bool{}
	for _, a := range agents {
		if a.Action == PlanWriteCreate || a.Action == PlanWriteOverwrite {
			d := filepath.ToSlash(a.DestinationPath)
			written[strings.ToLower(d)] = true
			folders[path.Dir(d)] = true
		}
	}
	var dirs []string
	for d := range folders {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var notes []TransformNote
	for _, dir := range dirs {
		names := map[string]string{} // lower-cased name -> actual name
		if entries, err := os.ReadDir(filepath.Join(workspace, filepath.FromSlash(dir))); err == nil {
			for _, e := range entries {
				if e.Type().IsRegular() && strings.HasSuffix(e.Name(), agentFileExtPlain) && !omit[strings.ToLower(dir+"/"+e.Name())] {
					names[strings.ToLower(e.Name())] = e.Name()
				}
			}
		}
		for _, a := range agents {
			if written[strings.ToLower(filepath.ToSlash(a.DestinationPath))] && path.Dir(filepath.ToSlash(a.DestinationPath)) == dir {
				names[strings.ToLower(path.Base(filepath.ToSlash(a.DestinationPath)))] = path.Base(filepath.ToSlash(a.DestinationPath))
			}
		}
		notes = append(notes, bothFormsInFolder(dir, names, written)...)
	}
	return notes
}

// bothFormsInFolder builds the notes of one folder from its candidate file names.
func bothFormsInFolder(dir string, names map[string]string, written map[string]bool) []TransformNote {
	type forms struct{ plain, agent []string }
	stems := map[string]*forms{}
	for lower, actual := range names {
		stem, isAgent := strings.TrimSuffix(lower, agentFileExtPlain), false
		if strings.HasSuffix(lower, agentFileExtAgent) {
			stem, isAgent = strings.TrimSuffix(lower, agentFileExtAgent), true
		}
		f := stems[stem]
		if f == nil {
			f = &forms{}
			stems[stem] = f
		}
		if isAgent {
			f.agent = append(f.agent, actual)
		} else {
			f.plain = append(f.plain, actual)
		}
	}
	var keys []string
	for k := range stems {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var notes []TransformNote
	for _, k := range keys {
		f := stems[k]
		if len(f.plain) == 0 || len(f.agent) == 0 {
			continue
		}
		all := append(append([]string(nil), f.plain...), f.agent...)
		sort.Strings(all)
		involved := false
		var paths []string
		for _, n := range all {
			p := dir + "/" + n
			paths = append(paths, p)
			involved = involved || written[strings.ToLower(p)]
		}
		if involved {
			notes = append(notes, TransformNote{
				Kind:    NoteBothForms,
				Message: "folder " + dir + " would hold both X.md and X.agent.md for one agent; Runner refuses that folder until one copy is removed: " + strings.Join(paths, ", "),
				Paths:   paths,
			})
		}
	}
	return notes
}
