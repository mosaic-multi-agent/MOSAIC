package app

// transform_plan_skills.go plans skills: each folder of the source skills folder is one skill
// that is copied unchanged, as a whole, to the target skills folder.

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// planSkills fills plan.Skills.
func (s *service) planSkills(in transformPlanInput, plan *TransformPlan) error {
	srcDesc := in.Source.Descriptor()
	relDir := srcDesc.Paths.Skills.Project
	if relDir == "" {
		return nil
	}
	root := filepath.Join(in.Workspace, filepath.FromSlash(relDir))
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sourceFiles, err := allWorkspaceFiles(root, relDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sk, err := s.planSkill(in, root, relDir, e.Name(), sourceFiles)
		if err != nil {
			return err
		}
		plan.Skills = append(plan.Skills, sk)
	}
	return nil
}

// allWorkspaceFiles returns the lower-cased workspace-relative slash paths of every regular
// file below root, which sits at relDir.
func allWorkspaceFiles(root, relDir string) (map[string]bool, error) {
	out := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[strings.ToLower(slashJoin(relDir, filepath.ToSlash(rel)))] = true
		return nil
	})
	return out, err
}

// planSkill plans one skill folder.
func (s *service) planSkill(in transformPlanInput, root, relDir, key string, sourceFiles map[string]bool) (PlannedSkill, error) {
	srcDesc, tgtDesc := in.Source.Descriptor(), in.Target.Descriptor()
	dir := filepath.Join(root, key)
	sk := PlannedSkill{
		Key:            key,
		SourceDir:      dir,
		DestinationDir: slashJoin(tgtDesc.Paths.Skills.Project, key),
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		sk.Files = append(sk.Files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return sk, err
	}
	sk.Version = skillVersion(filepath.Join(dir, "SKILL.md"))

	if !srcDesc.Paths.Skills.Supported || !tgtDesc.Paths.Skills.Supported {
		sk.Action, sk.Skip = PlanSkip, SkipUnsupported
		sk.Reason = "skills are not supported by " + unsupportedSkillHarness(srcDesc, tgtDesc)
		return sk, nil
	}
	exists := false
	for _, f := range sk.Files {
		dest := slashJoin(sk.DestinationDir, f)
		if sourceFiles[strings.ToLower(path.Clean(dest))] {
			sk.Action, sk.Skip = PlanSkip, SkipDestinationIsSource
			sk.Reason = "destination " + dest + " is a source file and is never written"
			return sk, nil
		}
		exists = exists || pathExists(filepath.Join(in.Workspace, filepath.FromSlash(dest)))
	}
	switch {
	case exists && !in.Overwrite:
		sk.Action, sk.Skip = PlanSkip, SkipDestinationExists
		sk.Reason = "skill files already exist in " + sk.DestinationDir + " and are kept unchanged"
	case exists:
		sk.Action = PlanWriteOverwrite
	default:
		sk.Action = PlanWriteCreate
	}
	return sk, nil
}

// unsupportedSkillHarness names the harness that has no skills folder.
func unsupportedSkillHarness(src, tgt *domain.HarnessDescriptor) string {
	if !src.Paths.Skills.Supported {
		return "the source harness " + src.ID
	}
	return "the target harness " + tgt.ID
}

// skillVersion reads mosaic_version (else legacy version) from a SKILL.md; empty when absent.
func skillVersion(skillFile string) string {
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return ""
	}
	doc, err := ParseGenericSource(data)
	if err != nil {
		return ""
	}
	field, ok := agentfields.ByGeneric("version")
	if !ok {
		return ""
	}
	fm := doc.Frontmatter()
	for _, key := range agentfields.ReadOrder(field) {
		if v, found := fm.Get(key); found && v.Kind == domain.KindScalar && v.Scalar != "" {
			return v.Scalar
		}
	}
	return ""
}
