package app_test

// transform_manifest_test.go specifies what the whole-deployment transform records in the
// workspace manifest (target entries with the carried versions and real content hashes; the
// source harness's entries and files untouched) and that repeating a transform changes nothing
// for every harness pair outside GHCP CLI / VS Code GHCP.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trCarriedVersions is the mosaic_version each agent fixture carries; harness-only has none.
var trCarriedVersions = map[string]string{
	"subagent": "2.3.0", "orchestrator": "3.0.0", "orchestrator-script": "1.0.0",
	"utility": "1.1.0", "standalone": "1.0.0", "harness-only": "",
}

func TestTransformManifest_RecordsTargetEntriesWithCarriedVersionsAndRealHashes(t *testing.T) {
	r := trModelRun(t, nil)
	req := r.request()
	req.TargetModel = "pre/answered"

	r.mustRun(req)

	m := trLoadManifest(t, r.ws)
	if m.HarnessID != hOpen {
		t.Errorf("manifest harness = %q, want the target %q", m.HarnessID, hOpen)
	}
	for _, key := range trAgentFixtures {
		e, ok := trEntryAt(m, r.dest(key))
		if !ok {
			t.Errorf("no manifest entry at %s", r.dest(key))
			continue
		}
		want := domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}
		if e.Ref != want {
			t.Errorf("%s: ref = %+v, want %+v", key, e.Ref, want)
		}
		if e.Version != trCarriedVersions[key] {
			t.Errorf("%s: version = %q, want the carried %q", key, e.Version, trCarriedVersions[key])
		}
		if wantHash := manifest.Hash([]byte(r.read(r.dest(key)))); e.ContentHash != wantHash {
			t.Errorf("%s: content hash = %q, want the hash of the written bytes %q", key, e.ContentHash, wantHash)
		}
		if e.HarnessVersion == "" {
			t.Errorf("%s: harness version is empty", key)
		}
	}
}

func TestTransformManifest_SourceHarnessEntriesAreUnchanged(t *testing.T) {
	r := trModelRun(t, nil)
	var seeded []domain.ManifestEntry
	for _, key := range []string{"subagent", "utility"} {
		rel := r.agentsDir(hClaude) + "/" + key + r.agentExt(hClaude)
		seeded = append(seeded, domain.ManifestEntry{
			Ref:         domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key},
			TargetPath:  rel,
			Version:     trCarriedVersions[key],
			ContentHash: manifest.Hash([]byte(r.read(rel))),
			DeployedAt:  time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
		})
	}
	err := manifest.NewStore().Save(r.ws, domain.Manifest{
		SchemaVersion: manifest.SchemaVersion, HarnessID: hClaude, Entries: seeded, ToolVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	r.mustRun(r.request())

	m := trLoadManifest(t, r.ws)
	for _, want := range seeded {
		got, ok := trEntryAt(m, want.TargetPath)
		if !ok {
			t.Errorf("source entry at %s was removed", want.TargetPath)
			continue
		}
		if got.Ref != want.Ref || got.Version != want.Version || got.ContentHash != want.ContentHash {
			t.Errorf("source entry at %s changed: got %+v, want %+v", want.TargetPath, got, want)
		}
	}
}

func TestTransformManifest_SourceFilesAreUnchangedByARealRun(t *testing.T) {
	r := trModelRun(t, nil)
	src := map[string]string{}
	for _, key := range trAgentFixtures {
		rel := r.agentsDir(hClaude) + "/" + key + ".md"
		src[rel] = r.read(rel)
	}

	r.mustRun(r.request())

	for rel, want := range src {
		if got := r.read(rel); got != want {
			t.Errorf("source file %s was modified", rel)
		}
	}
}

func TestTransformManifest_RepeatWithoutOverwrite_ChangesNothingAndAsksNothing(t *testing.T) {
	r := trModelRun(t, nil)
	r.mustRun(r.request())
	after := r.snapshot()
	asked := len(r.spy.choices) + len(r.spy.reviews)

	res := r.mustRun(r.request())

	if diff := trSameTree(after, r.snapshot()); diff != "" {
		t.Errorf("repeat changed the workspace (manifest and TODO file included): %s", diff)
	}
	if got := len(r.spy.choices) + len(r.spy.reviews); got != asked {
		t.Errorf("repeat asked %d more questions, want none", got-asked)
	}
	if res.Transformed != 0 || res.TodoPath != "" {
		t.Errorf("repeat Transformed=%d TodoPath=%q, want 0 and empty", res.Transformed, res.TodoPath)
	}
	for _, key := range trAgentFixtures {
		if f := trFile(t, res, key); f.Status != app.StatusSkipped || f.Skip != app.SkipDestinationExists {
			t.Errorf("repeat: %s status %q skip %q, want skipped/destination-exists", key, f.Status, f.Skip)
		}
	}
}

// trRepeatPairs are the harness pairs outside GHCP CLI / VS Code GHCP.
var trRepeatPairs = []struct{ name, src, tgt string }{
	{"claude-to-opencode", hClaude, hOpen},
	{"opencode-to-claude", hOpen, hClaude},
	{"claude-to-ghcp-cli", hClaude, hCLI},
	{"opencode-to-vscode-ghcp", hOpen, hVSCode},
	{"opencode-to-ghcp-cli", hOpen, hCLI},
	{"claude-to-vscode-ghcp", hClaude, hVSCode},
}

// trAgentFolderFiles returns the snapshot restricted to the source and target agents folders.
func trAgentFolderFiles(r *trRun) map[string]string {
	out := map[string]string{}
	for rel, content := range r.snapshot() {
		for _, h := range []string{r.src, r.tgt} {
			if strings.HasPrefix(rel, r.agentsDir(h)+"/") {
				out[rel] = content
			}
		}
	}
	return out
}

// trManifestWithoutTimes returns the manifest entries with the deploy time cleared.
func trManifestWithoutTimes(t *testing.T, ws string) []domain.ManifestEntry {
	t.Helper()
	entries := append([]domain.ManifestEntry(nil), trLoadManifest(t, ws).Entries...)
	for i := range entries {
		entries[i].DeployedAt = time.Time{}
	}
	return entries
}

func TestTransformManifest_Repeat_ChangesNothingForEveryPairOutsideTheGhcpPair(t *testing.T) {
	for _, p := range trRepeatPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trNew(t, nil, p.src, p.tgt)
			r.seed(trAgentFixtures...)
			req := r.request()
			req.TargetModel = "pre/answered"
			r.mustRun(req)
			after := r.snapshot()

			r.mustRun(req)

			if diff := trSameTree(after, r.snapshot()); diff != "" {
				t.Errorf("repeating the transform changed the workspace: %s", diff)
			}
		})
	}
}

// With Overwrite the executor runs again, so the deploy time and TODO file may legitimately
// change; the agent files and the manifest entries (deploy time aside) must not.
func TestTransformManifest_RepeatWithOverwrite_KeepsAgentFilesAndEntries(t *testing.T) {
	for _, p := range trRepeatPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trNew(t, nil, p.src, p.tgt)
			r.seed(trAgentFixtures...)
			req := r.request()
			req.TargetModel = "pre/answered"
			req.Overwrite = true
			r.mustRun(req)
			files, entries := trAgentFolderFiles(r), trManifestWithoutTimes(t, r.ws)

			r.mustRun(req)

			if diff := trSameTree(files, trAgentFolderFiles(r)); diff != "" {
				t.Errorf("repeating with Overwrite changed an agent file: %s", diff)
			}
			if !reflect.DeepEqual(entries, trManifestWithoutTimes(t, r.ws)) {
				t.Errorf("repeating with Overwrite changed the manifest entries (deploy time aside)")
			}
		})
	}
}

func TestTransformManifest_Repeat_NeverChangesAnEarlierModel(t *testing.T) {
	for _, p := range trRepeatPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trNew(t, nil, p.src, p.tgt)
			r.seed(trAgentFixtures...)
			first := r.request()
			first.TargetModel = "pre/answered"
			r.mustRun(first)
			second := r.request()
			second.TargetModel = "other/changed"

			r.mustRun(second)

			for _, key := range trAgentFixtures {
				trAssertModel(t, r, key, "pre/answered")
				if strings.Contains(r.read(r.dest(key)), "other/changed") {
					t.Errorf("agent %q was rewritten with the second run's model", key)
				}
			}
		})
	}
}

func TestTransformManifest_Repeat_NeverDoublesAnExtension(t *testing.T) {
	for _, p := range trRepeatPairs {
		t.Run(p.name, func(t *testing.T) {
			r := trNew(t, nil, p.src, p.tgt)
			r.seed(trAgentFixtures...)
			req := r.request()
			req.TargetModel = "pre/answered"
			req.Overwrite = true
			r.mustRun(req)
			r.mustRun(req)

			entries, err := os.ReadDir(filepath.Join(r.ws, filepath.FromSlash(r.agentsDir(p.tgt))))
			if err != nil {
				t.Fatalf("read target folder: %v", err)
			}
			ext := r.agentExt(p.tgt)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ext+ext) || strings.Contains(e.Name(), ".agent.agent") {
					t.Errorf("target file %q has a doubled extension", e.Name())
				}
			}
		})
	}
}
