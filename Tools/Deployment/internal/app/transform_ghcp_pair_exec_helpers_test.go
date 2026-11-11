package app_test

// transform_ghcp_pair_exec_helpers_test.go holds the shared builders of the real-run tests of a
// transform between the two GHCP targets: a workspace with a recorded source deployment (agents,
// hook files, an edited registration file), unrecorded user files, a skill and an unrelated
// manifest entry, plus failure injection for the manifest store.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	// trPairUserAgent is an X.agent.md the manifest does not record.
	trPairUserAgent = ".github/agents/mine.agent.md"
	// trPairUserNote is a plain markdown file in the agents folder that the manifest does not record.
	trPairUserNote = ".github/agents/notes.md"
	// trPairUserSkill is a user skill in the shared skills folder.
	trPairUserSkill = ".github/skills/user-skill/SKILL.md"
	// trPairBesideAgent is the content of an X.agent.md the manifest does not record, beside a legacy X.md.
	trPairBesideAgent = "an X.agent.md the manifest does not record\n"
	// trPairEditedLogger is a user-edited copy of a hook file the target variant also writes.
	trPairEditedLogger = "#!/bin/sh\n# edited by the user\n"
	// trPairUnrelatedPath is the file of a manifest entry that belongs to neither deployment.
	trPairUnrelatedPath = ".github/skills/other-skill/SKILL.md"
)

// trPairExecCase is one shape of recorded source deployment.
type trPairExecCase struct {
	name     string
	agentRel string // the recorded agent file
	beside   bool   // an unrecorded X.agent.md exists beside a legacy X.md
}

// trPairExecCases are the three recorded deployments a pair transform handles.
var trPairExecCases = []trPairExecCase{
	{name: "recorded X.agent.md", agentRel: trPairInPlaceAgent},
	{name: "recorded legacy X.md", agentRel: trPairLegacyAgent},
	{name: "legacy X.md beside X.agent.md", agentRel: trPairLegacyAgent, beside: true},
}

// trPairDirections are the two directions of the GHCP pair.
var trPairDirections = [][2]string{{hCLI, hVSCode}, {hVSCode, hCLI}}

// trPairExecRun is a pair run over the recorded deployment of the case, with unrecorded user
// files, a skill, an edited shared hook file and an unrelated manifest entry.
func trPairExecRun(t *testing.T, src, tgt string, c trPairExecCase, opts ...func(*app.Deps)) *trRun {
	t.Helper()
	r, _ := trPairHookRunWithAgent(t, src, tgt, c.agentRel, opts...)
	if c.beside {
		trWrite(t, r.ws, trPairInPlaceAgent, trPairBesideAgent)
	}
	trWrite(t, r.ws, r.hooksDir(src)+"/logger.sh", trPairEditedLogger)
	trWrite(t, r.ws, trPairUserAgent, trFixture(t, "utility"))
	trWrite(t, r.ws, trPairUserNote, "my own notes\n")
	trWrite(t, r.ws, trPairUserSkill, "---\nname: user-skill\n---\nmine\n")
	r.seedSkill()
	trWrite(t, r.ws, trPairUnrelatedPath, "unrelated skill\n")
	trAppendEntry(t, r.ws, trPairUnrelatedEntry())
	return r
}

// trPairUnrelatedEntry is a manifest entry that belongs to neither deployment of the pair.
func trPairUnrelatedEntry() domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:    hOpen,
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: "other-skill"},
		TargetPath: trPairUnrelatedPath, Version: "9.9.9", ContentHash: manifest.Hash([]byte("unrelated skill\n")),
		DeployedAt: time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
	}
}

// trAppendEntry adds an entry to the workspace manifest.
func trAppendEntry(t *testing.T, ws string, e domain.ManifestEntry) {
	t.Helper()
	m := trLoadManifest(t, ws)
	if e.Harness == "" && len(m.Harnesses) > 0 {
		e.Harness = m.Harnesses[0].HarnessID
	}
	if _, ok := m.Section(e.Harness); !ok {
		m.Harnesses = append(m.Harnesses, domain.HarnessSection{HarnessID: e.Harness})
	}
	m.Entries = append(m.Entries, e)
	if err := manifest.NewStore().Save(ws, m); err != nil {
		t.Fatalf("save manifest: %v", err)
	}
}

// trWithoutBackups returns the snapshot without the files below .mosaic/backups.
func trWithoutBackups(snap map[string]string) map[string]string {
	out := map[string]string{}
	for rel, content := range snap {
		if !strings.HasPrefix(rel, ".mosaic/backups/") {
			out[rel] = content
		}
	}
	return out
}

// trBackupsOf returns the contents of every backup file of the workspace-relative path.
func trBackupsOf(t *testing.T, ws, rel string) []string {
	t.Helper()
	dir := filepath.Join(ws, ".mosaic", "backups", filepath.FromSlash(filepath.ToSlash(filepath.Dir(rel))))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(rel)+".") && strings.HasSuffix(e.Name(), ".bak") {
			b, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
			if rerr != nil {
				t.Fatalf("read backup %s: %v", e.Name(), rerr)
			}
			out = append(out, string(b))
		}
	}
	return out
}

// trAssertRecoverable fails the test unless a backup of rel holds want.
func trAssertRecoverable(t *testing.T, ws, rel, want string) {
	t.Helper()
	for _, got := range trBackupsOf(t, ws, rel) {
		if got == want {
			return
		}
	}
	t.Errorf("no backup of %s under .mosaic/backups holds the original bytes %q", rel, want)
}

// trFilesIn lists the file names directly in a workspace-relative folder.
func trFilesIn(t *testing.T, ws, rel string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(ws, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// errTrManifestSave is the error the failing manifest store returns from Save.
var errTrManifestSave = errors.New("manifest save failed")

// trSaveFailingStore loads like the real store and fails every Save. When seen is set, it first
// records the workspace as the executor left it at the moment of the save.
type trSaveFailingStore struct {
	manifest.Store
	t    *testing.T
	seen *map[string]string
}

func (s trSaveFailingStore) Save(ws string, _ domain.Manifest) error {
	if s.seen != nil {
		*s.seen = trSnapshot(s.t, ws)
	}
	return errTrManifestSave
}

// trFailManifestSave makes the manifest save of the run fail, planning and loading as usual.
// seen (may be nil) receives the workspace tree at the time of the failing save, which proves
// the removals and replacements ran before the failure.
func trFailManifestSave(t *testing.T, seen *map[string]string) func(*app.Deps) {
	return func(d *app.Deps) {
		store := trSaveFailingStore{Store: manifest.NewStore(), t: t, seen: seen}
		d.Manifest = store
		d.Executor = deploy.NewExecutor(store, d.Logger, d.Todo)
	}
}
