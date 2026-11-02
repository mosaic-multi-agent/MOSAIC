package app

// deployed_bytes_migration.go lets the deployed-bytes rule serve a migrating GHCP agent the
// bytes of its legacy X.md. The executor removes X.md before it renders content, so the bytes
// are captured here, read-only, when the rule is built (after the plan was shown and confirmed,
// before anything is written). Backups are never the read path.

import (
	"strings"

	"mosaic-deploy/internal/domain"
)

// withMigratedAgents returns the rule extended with the legacy bytes of every agent p migrates
// from X.md to X.agent.md: items the planner marked as renamed and items for which the
// migration planned the removal of their legacy file.
func (d deployedBytes) withMigratedAgents(p domain.Plan) deployedBytes {
	removed := make(map[string]bool, len(p.Removals))
	for _, r := range p.Removals {
		removed[strings.ToLower(r.Path)] = true
	}
	captured := map[string][]byte{}
	for _, item := range p.Items {
		if item.Ref.Kind != domain.ArtifactAgent || !strings.HasSuffix(item.TargetPath, agentFileExtAgent) {
			continue
		}
		legacy := item.RenamedFrom
		if legacy == "" {
			candidate := strings.TrimSuffix(item.TargetPath, agentFileExtAgent) + agentFileExtPlain
			if !removed[strings.ToLower(candidate)] {
				continue
			}
			legacy = candidate
		}
		if b := readDeployedFile(d.workspace, legacy); b != nil {
			captured[item.TargetPath] = b
		}
	}
	d.captured = captured
	return d
}

// keepingUnchangedMigrations wraps content so a migrating agent the planner found current (a
// rename with nothing stale) keeps its legacy bytes verbatim: the rename must not rewrite a
// file a plain update would have left untouched.
func (d deployedBytes) keepingUnchangedMigrations(
	p domain.Plan,
	content func(domain.PlanItem) ([]byte, error),
) func(domain.PlanItem) ([]byte, error) {
	verbatim := map[string]bool{}
	for _, item := range p.Items {
		if item.RenamedFrom != "" && len(item.Stale) == 0 && d.captured[item.TargetPath] != nil {
			verbatim[item.TargetPath] = true
		}
	}
	if len(verbatim) == 0 {
		return content
	}
	return func(item domain.PlanItem) ([]byte, error) {
		if verbatim[item.TargetPath] {
			return d.captured[item.TargetPath], nil
		}
		return content(item)
	}
}
