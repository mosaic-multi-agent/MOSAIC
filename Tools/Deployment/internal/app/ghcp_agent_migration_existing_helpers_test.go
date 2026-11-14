package app_test

// ghcp_agent_migration_existing_helpers_test.go holds the builders and plan assertions shared by
// the tests of an X.agent.md that already exists beside a recorded, present X.md: a modern GHCP
// deployment with a legacy copy of every agent recorded next to it, the states the existing
// X.agent.md can be in (unchanged, outdated, edited, conflicting) and the plan the migration must
// leave for each. All fixtures come from the tool-local parity catalog.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	// gmeMigrationReasonPrefix starts the reason the migration adds to an agent item.
	gmeMigrationReasonPrefix = "migrates from the legacy file "
	// gmeEditReason is the reason of an update caused by an edit of Deploy-owned content.
	gmeEditReason = "Deploy-owned content was edited locally"
	// gmeEditedLine is the Deploy-owned body text the edited state adds.
	gmeEditedLine = "A line the user added to the Deploy-owned part of the agent."
)

// gmeState is one state of an X.agent.md beside a recorded, present X.md, and the plan item
// the migration has to leave for it.
type gmeState struct {
	name    string
	prepare func(t *testing.T, ws, key string)
	// action is the planned action the item has without the migration.
	action domain.PlanAction
	// backup is the item's own BackupBeforeWrite.
	backup bool
	// replaced is true when the plan lists a replacement of the existing X.agent.md: an update
	// that writes over an existing file without a backup of its own.
	replaced bool
	// ownReason is a text one of the item's own reasons (before the migration reason) holds.
	ownReason string
}

// gmeStates returns the states an existing X.agent.md can be in.
func gmeStates() []gmeState {
	return []gmeState{
		{name: "unchanged", prepare: func(*testing.T, string, string) {}, action: domain.ActionUnchanged},
		{name: "outdated", prepare: gmeMakeOutdated, action: domain.ActionUpdate, replaced: true, ownReason: "changed from"},
		{name: "edited", prepare: gmeMakeEdited, action: domain.ActionUpdate, backup: true, ownReason: gmeEditReason},
		{name: "unrecorded", prepare: gmeMakeUnrecorded, action: domain.ActionConflict},
		{name: "recorded unreadable", prepare: gmeMakeUnreadable, action: domain.ActionConflict},
	}
}

// gmeWorkspace deploys the parity sample to a fresh workspace and records a present legacy copy
// X.md of every agent next to its recorded X.agent.md.
func gmeWorkspace(t *testing.T, harness string) string {
	t.Helper()
	ws := parityDeploy(t, harness)
	for _, a := range parityAgents {
		content := string(gamRead(t, ws, gamRel(a.key, ".agent.md")))
		gamWrite(t, ws, gamRel(a.key, ".md"), content)
		gamRecordAgentAt(t, ws, a.key, gamRel(a.key, ".md"), content)
	}
	return ws
}

// gmeRecordContent re-records the content hash of the entry at rel.
func gmeRecordContent(t *testing.T, ws, rel, content string) {
	t.Helper()
	m := gamManifest(t, ws)
	for i, e := range m.Entries {
		if e.TargetPath == rel {
			m.Entries[i].ContentHash = manifest.Hash([]byte(content))
		}
	}
	gamSaveManifest(t, ws, m)
}

// gmeMakeOutdated ages the version of the existing X.agent.md, as a file deployed by an older
// catalog, and records the aged file.
func gmeMakeOutdated(t *testing.T, ws, key string) {
	t.Helper()
	rel := gamRel(key, ".agent.md")
	aged := gamOlderVersion(t, key, string(gamRead(t, ws, rel)))
	gamWrite(t, ws, rel, aged)
	gmeRecordContent(t, ws, rel, aged)
}

// gmeMakeEdited adds a line of Deploy-owned body text (outside every region) to the existing
// X.agent.md and leaves the record as the deploy wrote it.
func gmeMakeEdited(t *testing.T, ws, key string) {
	t.Helper()
	rel := gamRel(key, ".agent.md")
	doc := string(gamRead(t, ws, rel))
	const open, closing = "---\n", "\n---\n"
	i := strings.Index(doc[len(open):], closing)
	if !strings.HasPrefix(doc, open) || i < 0 {
		t.Fatalf("%s: deployed file has no frontmatter to place an edit after", key)
	}
	at := len(open) + i + len(closing)
	gamWrite(t, ws, rel, doc[:at]+gmeEditedLine+"\n"+doc[at:])
}

// gmeMakeUnrecorded replaces the existing X.agent.md by a file the manifest does not record.
func gmeMakeUnrecorded(t *testing.T, ws, key string) {
	t.Helper()
	rel := gamRel(key, ".agent.md")
	gamWrite(t, ws, rel, "stale user copy\n")
	gamDropEntry(t, ws, key, rel)
}

// gmeMakeUnreadable replaces the existing X.agent.md by a recorded file that is no agent.
func gmeMakeUnreadable(t *testing.T, ws, key string) {
	t.Helper()
	rel := gamRel(key, ".agent.md")
	gamWrite(t, ws, rel, "recorded copy\n")
	gmeRecordContent(t, ws, rel, "recorded copy\n")
}

// gmeRemoval returns the removal of a workspace-relative path.
func gmeRemoval(p domain.Plan, rel string) (domain.Removal, bool) {
	for _, r := range p.Removals {
		if strings.ReplaceAll(r.Path, "\\", "/") == rel {
			return r, true
		}
	}
	return domain.Removal{}, false
}

// gmeLastReasonIsMigration reports whether the item's last reason is the migration reason naming
// the legacy file.
func gmeLastReasonIsMigration(it domain.PlanItem, legacyRel string) bool {
	if len(it.Reasons) == 0 {
		return false
	}
	last := it.Reasons[len(it.Reasons)-1]
	return strings.HasPrefix(last, gmeMigrationReasonPrefix) && strings.Contains(last, legacyRel)
}

// gmeAssertLegacyRemoval checks the plan removes the legacy file with its entry.
func gmeAssertLegacyRemoval(t *testing.T, p domain.Plan, key string) {
	t.Helper()
	legacyRel := gamRel(key, ".md")
	r, ok := gmeRemoval(p, legacyRel)
	switch {
	case !ok:
		t.Errorf("%s: plan does not remove the present legacy %s", key, legacyRel)
	case r.Ref.Kind != domain.ArtifactAgent || r.Ref.Key != key:
		t.Errorf("%s: removal of %s has ref %+v, want the agent's ref", key, legacyRel, r.Ref)
	case !r.DropEntry:
		t.Errorf("%s: removal of %s does not drop its manifest entry", key, legacyRel)
	case !strings.Contains(r.Reason, "replaced by"):
		t.Errorf("%s: removal reason %q does not say the legacy file is replaced", key, r.Reason)
	}
}

// gmeAssertPlanItem checks the migrated agent item keeps the action, conflict, backup marker and
// reasons it has without the migration, and carries the migration's removal and reason.
func gmeAssertPlanItem(t *testing.T, p domain.Plan, key string, st gmeState) {
	t.Helper()
	newRel, legacyRel := gamRel(key, ".agent.md"), gamRel(key, ".md")
	it, ok := gplAgentItem(p, key)
	if !ok {
		t.Errorf("%s: plan has no item for %s", key, newRel)
		return
	}
	if it.Action != st.action {
		t.Errorf("%s: action = %q, want %q (the state the file has without the migration)", key, it.Action, st.action)
	}
	if (it.Conflict != nil) != (st.action == domain.ActionConflict) {
		t.Errorf("%s: conflict = %+v for action %q", key, it.Conflict, it.Action)
	}
	if it.BackupBeforeWrite != st.backup {
		t.Errorf("%s: BackupBeforeWrite = %t, want %t", key, it.BackupBeforeWrite, st.backup)
	}
	if it.RenamedFrom != "" {
		t.Errorf("%s: RenamedFrom = %q, want none: %s exists and is the agent's state", key, it.RenamedFrom, newRel)
	}
	if !gmeLastReasonIsMigration(it, legacyRel) {
		t.Errorf("%s: reasons = %q, want the migration reason naming %s added last", key, it.Reasons, legacyRel)
	}
	gmeAssertOwnReasons(t, it, key, st)
	if got := drgReplacesPath(p, newRel); got != st.replaced {
		t.Errorf("%s: plan replaces %s = %t, want %t", key, newRel, got, st.replaced)
	}
	gmeAssertLegacyRemoval(t, p, key)
}

// gmeAssertOwnReasons checks the reasons before the migration reason are the item's own.
func gmeAssertOwnReasons(t *testing.T, it domain.PlanItem, key string, st gmeState) {
	t.Helper()
	own := it.Reasons
	if len(own) > 0 {
		own = own[:len(own)-1]
	}
	for _, r := range own {
		if strings.HasPrefix(r, gmeMigrationReasonPrefix) {
			t.Errorf("%s: reasons = %q, the migration reason is not last", key, it.Reasons)
		}
	}
	if st.action != domain.ActionUnchanged && len(own) == 0 {
		t.Errorf("%s: reasons = %q, want the item's own reason before the migration reason", key, it.Reasons)
	}
	if st.action == domain.ActionUnchanged && len(it.Stale) != 0 {
		t.Errorf("%s: stale = %v on an unchanged agent", key, it.Stale)
	}
	if st.ownReason == "" {
		return
	}
	for _, r := range own {
		if strings.Contains(r, st.ownReason) {
			return
		}
	}
	t.Errorf("%s: reasons = %q, want an own reason containing %q", key, it.Reasons, st.ownReason)
}

// gmeAssertOthersUnchanged checks the agents whose X.agent.md is up to date stay unchanged and
// still carry the removal of their legacy file.
func gmeAssertOthersUnchanged(t *testing.T, p domain.Plan, keys []string) {
	t.Helper()
	for _, key := range keys {
		gmeAssertPlanItem(t, p, key, gmeStates()[0])
	}
}

// gmeAssertReplacementsMeanWrites checks every replacement names an agent the plan updates
// without a backup of its own: a migration never turns an unchanged, conflicting or
// self-backed-up agent into a replacement.
func gmeAssertReplacementsMeanWrites(t *testing.T, p domain.Plan) {
	t.Helper()
	for _, r := range p.Replacements {
		path := strings.ReplaceAll(r.Path, "\\", "/")
		matched := false
		for _, it := range p.Items {
			if strings.ReplaceAll(it.TargetPath, "\\", "/") != path {
				continue
			}
			matched = true
			if it.Action != domain.ActionUpdate || it.BackupBeforeWrite {
				t.Errorf("replacement of %s although its item is %q (backup before write %t)", path, it.Action, it.BackupBeforeWrite)
			}
		}
		if !matched {
			t.Errorf("replacement of %s names no planned item", path)
		}
	}
}
