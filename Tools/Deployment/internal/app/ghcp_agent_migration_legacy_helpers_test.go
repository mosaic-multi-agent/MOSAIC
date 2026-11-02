package app_test

// ghcp_agent_migration_legacy_helpers_test.go extends the legacy-workspace builder of the GHCP
// migration tests with the state a long-lived workspace carries: sentinels in project and
// custom regions, hand-set tools, infrastructure declarations, an agent whose manifest record
// is missing and an agent whose file is older than the catalog. It also provides a reviewing
// Interaction that keeps the plan it was shown and the agent folder as it was at that moment.
// All fixtures come from the tool-local parity catalog.

import (
	"context"
	"os"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	gamProjectSentinel = "LEGACY-PROJECT-REGION-SENTINEL"
	gamCustomSentinel  = "LEGACY-CUSTOM-REGION-SENTINEL"
	gamInfraSentinel   = "LEGACY-INFRASTRUCTURE-DECLARATION-SENTINEL"
	gamHandSetTools    = "tools: ['hand-set-legacy-tool']"
)

// gamLegacyOpts shapes the legacy workspace.
type gamLegacyOpts struct {
	unrecorded []string // agent keys whose X.md stays on disk but has no manifest entry
	stale      []string // agent keys whose X.md declares an older version than the catalog
}

func (o gamLegacyOpts) has(list []string, key string) bool {
	for _, k := range list {
		if k == key {
			return true
		}
	}
	return false
}

// gamPlantedLegacyWorkspace is gamLegacyWorkspace with user state planted in every agent file
// (project-region and custom-region sentinels, hand-set tools, an infrastructure declaration in
// the orchestrators) and the manifest hashes re-recorded, so each file is still the file the
// manifest describes. It returns the workspace and the legacy bytes of each agent by key.
func gamPlantedLegacyWorkspace(t *testing.T, harness string, o gamLegacyOpts) (string, map[string][]byte) {
	t.Helper()
	ws, _ := gamLegacyWorkspace(t, harness)
	m := gamManifest(t, ws)
	for _, a := range parityAgents {
		rel := gamRel(a.key, ".md")
		content := gamPlantState(string(gamRead(t, ws, rel)))
		if o.has(o.stale, a.key) {
			content = gamOlderVersion(t, a.key, content)
		}
		gamWrite(t, ws, rel, content)
		for i, e := range m.Entries {
			if e.Ref.Kind == domain.ArtifactAgent && e.Ref.Key == a.key {
				m.Entries[i].ContentHash = manifest.Hash([]byte(content))
			}
		}
	}
	kept := m.Entries[:0]
	for _, e := range m.Entries {
		if e.Ref.Kind == domain.ArtifactAgent && o.has(o.unrecorded, e.Ref.Key) {
			continue
		}
		kept = append(kept, e)
	}
	m.Entries = kept
	gamSaveManifest(t, ws, m)
	legacy := map[string][]byte{}
	for _, a := range parityAgents {
		legacy[a.key] = gamRead(t, ws, gamRel(a.key, ".md"))
	}
	return ws, legacy
}

// gamPlantState puts the user state into a deployed agent document.
func gamPlantState(doc string) string {
	for _, tag := range []string{`<CodebaseContext type="project">`, `<ContextLimits type="project">`} {
		doc = strings.Replace(doc, tag+"\n", tag+"\n"+gamProjectSentinel+"\n", 1)
	}
	doc = strings.Replace(doc, `<InfrastructureAgents type="managed">`+"\n",
		`<InfrastructureAgents type="managed">`+"\n"+gamInfraSentinel+"\n", 1)
	lines := strings.Split(doc, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "tools:") {
			lines[i] = gamHandSetTools
			break
		}
	}
	doc = strings.Join(lines, "\n")
	return strings.TrimRight(doc, "\n") + "\n\n" + `<UserNotes type="custom">` + "\n" +
		gamCustomSentinel + "\n" + `</UserNotes>` + "\n"
}

// gamOlderVersion rewrites the frontmatter version of a deployed agent to an older one, so the
// agent compares as stale against the catalog.
func gamOlderVersion(t *testing.T, key, doc string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "mosaic_version:") {
			lines[i] = "mosaic_version: 0.0.1"
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("%s: deployed file has no version line to age", key)
	return doc
}

// gamReviewCapture is an Interaction that keeps every plan it is asked to review together with
// the agent folder as it was at that moment, and records all questions.
type gamReviewCapture struct {
	*interactiontest.Stub
	ws     string
	plans  []domain.Plan
	folder [][]string // agent folder file names at each review
}

func (c *gamReviewCapture) Review(ctx context.Context, p domain.Plan) (domain.ConfirmAnswer, error) {
	c.plans = append(c.plans, p)
	var names []string
	if files, err := os.ReadDir(gamAbs(c.ws, gamAgentsDir)); err == nil {
		for _, f := range files {
			names = append(names, f.Name())
		}
	}
	c.folder = append(c.folder, names)
	return c.Stub.Review(ctx, p)
}

// gamCaptureDeps replaces the service Interaction with a gamReviewCapture over a plain stub.
func gamCaptureDeps(c *gamReviewCapture) func(*app.Deps) {
	return func(d *app.Deps) { d.Interaction = c }
}

func gamNewReviewCapture(ws string) *gamReviewCapture {
	return &gamReviewCapture{Stub: interactiontest.NewBuilder().Build(), ws: ws}
}
