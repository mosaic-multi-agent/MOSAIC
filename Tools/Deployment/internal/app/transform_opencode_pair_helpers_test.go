package app_test

// transform_opencode_pair_helpers_test.go holds the shared builders of the real-run tests of a
// transform between the two OpenCode targets (opencode and opencode-v2): a service whose
// registry holds the real built-in modules of both targets, a tool-local hook bundle that
// declares both variants (with different file lists, so source-only files exist in each
// direction), and a workspace holding a recorded source deployment (an agent, a skill, hook
// files) beside unrecorded user files and an unrelated manifest entry. Nothing here reads the
// repository Catalog or its logger bundle.

import (
	"path"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

const (
	// ocHookKey is the bundle whose variants write different file sets into .opencode/plugins.
	ocHookKey     = "oc-logger"
	ocHookVersion = "3.0.0"

	ocPluginsDir = ".opencode/plugins"
	ocAgentsDir  = ".opencode/agents"

	// ocWorker is the recorded source agent; it sits at the same path for both targets.
	ocWorker = ".opencode/agents/worker.md"
	// ocUserPlugin and ocUserNote are files no manifest entry records.
	ocUserPlugin = ".opencode/plugins/user-plugin.ts"
	ocUserNote   = ".opencode/agents/notes.md"
	// ocUnrelatedPath is the file of a manifest entry that belongs to neither OpenCode target.
	ocUnrelatedPath = ".claude/agents/other.md"
)

// ocVariantFiles are the files (testdata source, deployed name) each OpenCode variant writes.
// The entry file has the same name in both; each variant also writes one helper the other lacks.
var ocVariantFiles = map[string][][2]string{
	hOpen:   {{"hooks/oc/v1/plugin.ts", "mosaic-logger.ts"}, {"hooks/oc/v1/lib/v1-helper.ts", "lib/v1-helper.ts"}},
	hOpenV2: {{"hooks/oc/v2/plugin.ts", "mosaic-logger.ts"}, {"hooks/oc/v2/lib/host.ts", "lib/host.ts"}},
}

// ocOnlyHelper is the helper file only the variant of the harness writes.
var ocOnlyHelper = map[string]string{hOpen: "lib/v1-helper.ts", hOpenV2: "lib/host.ts"}

// ocToolsKey is the frontmatter key each target's agents carry for their permissions.
var ocToolsKey = map[string]string{hOpen: "permission", hOpenV2: "permissions"}

// ocDirections are the two directions of the OpenCode pair.
var ocDirections = [][2]string{{hOpen, hOpenV2}, {hOpenV2, hOpen}}

// ocBundle is the hook bundle with a variant for each of the given harness ids.
func ocBundle(t *testing.T, variantsOf ...string) domain.HookBundle {
	t.Helper()
	b := domain.HookBundle{Key: ocHookKey, Version: ocHookVersion, Variants: map[string]domain.HookVariant{}}
	for _, id := range variantsOf {
		v := domain.HookVariant{HarnessID: id, Supported: true}
		for _, f := range ocVariantFiles[id] {
			v.Files = append(v.Files, domain.HookFile{SourcePath: trAbs(t, f[0]), TargetName: f[1]})
		}
		b.Variants[id] = v
	}
	return b
}

// ocWithRegistry gives the run a registry holding the real modules of every harness the
// transform tests use, the OpenCode V2 module included.
func ocWithRegistry(t *testing.T) func(*app.Deps) {
	return func(d *app.Deps) {
		reg := &stubRegistry{modules: map[string]domain.HarnessModule{}}
		for _, id := range []string{hClaude, hOpen, hOpenV2, hCLI, hVSCode} {
			m := rbModule(t, id)
			reg.list = append(reg.list, m.Ref())
			reg.modules[id] = m
		}
		d.Registry = reg
	}
}

// ocPairRun is a pair run whose catalog knows one hook bundle with the given variants and
// whose workspace holds the recorded source deployment of src: the agent ocWorker, the review
// skill, the source variant's hook files, all recorded under src; plus unrecorded user files and
// an unrelated entry of another harness.
func ocPairRun(t *testing.T, src, tgt string, variantsOf []string, opts ...func(*app.Deps)) *trRun {
	t.Helper()
	cat := newMinimalCatalog()
	cat.hooks = []domain.HookBundle{ocBundle(t, variantsOf...)}
	all := append([]func(*app.Deps){func(d *app.Deps) { d.Catalog = cat }, ocWithRegistry(t)}, opts...)
	r := trNew(t, nil, src, tgt, all...)

	trWrite(t, r.ws, ocWorker, trFixture(t, "subagent"))
	trSaveManifest(t, r.ws, src, ocWorker)
	for rel, content := range r.seedSkill() {
		p := r.skillsDir(src) + "/" + trSkillKey + "/" + rel
		trAppendEntry(t, r.ws, domain.ManifestEntry{
			Harness: src, Ref: domain.ArtifactRef{Kind: domain.ArtifactSkill, Key: trSkillKey},
			TargetPath: p, Version: trSkillVersion, ContentHash: manifest.Hash([]byte(content)),
		})
	}
	for _, f := range ocVariantFiles[src] {
		content := trReadTestdata(t, f[0])
		p := ocPluginsDir + "/" + f[1]
		trWrite(t, r.ws, p, content)
		trAppendEntry(t, r.ws, domain.ManifestEntry{
			Harness: src, Ref: domain.ArtifactRef{Kind: domain.ArtifactHook, Key: ocHookKey},
			TargetPath: p, Version: ocHookVersion, ContentHash: manifest.Hash([]byte(content)),
		})
	}
	trWrite(t, r.ws, ocUserPlugin, "export default { id: \"user-plugin\" }\n")
	trWrite(t, r.ws, ocUserNote, "my own notes\n")
	trWrite(t, r.ws, ocUnrelatedPath, "unrelated agent\n")
	trAppendEntry(t, r.ws, ocUnrelatedEntry())
	return r
}

// ocUnrelatedEntry is a manifest entry that belongs to neither OpenCode target.
func ocUnrelatedEntry() domain.ManifestEntry {
	return domain.ManifestEntry{
		Harness:    hClaude,
		Ref:        domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: "other"},
		TargetPath: ocUnrelatedPath, Version: "9.9.9", ContentHash: manifest.Hash([]byte("unrelated agent\n")),
		DeployedAt: time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
	}
}

// ocEach runs the body once per direction of the pair.
func ocEach(t *testing.T, body func(t *testing.T, r *trRun)) {
	t.Helper()
	for _, pair := range ocDirections {
		t.Run(pair[0]+"->"+pair[1], func(t *testing.T) {
			body(t, ocPairRun(t, pair[0], pair[1], []string{hOpen, hOpenV2}))
		})
	}
}

// ocRequest is r.request() for a pair named explicitly (round trips reverse the run's pair).
func ocRequest(r *trRun, src, tgt string) app.TransformHarnessRequest {
	req := r.request()
	req.SourceHarnessID, req.TargetHarnessID = src, tgt
	return req
}

// ocEntriesOf returns the manifest entries recorded for a harness.
func ocEntriesOf(m domain.Manifest, harnessID string) []domain.ManifestEntry {
	return m.ForHarness(harnessID).Entries
}

// ocHookPaths returns the target paths of the hook entries of a manifest view, sorted by path.
func ocHookPaths(m domain.Manifest) []string {
	var out []string
	for _, e := range ocEntriesOfKind(m, domain.ArtifactHook) {
		out = append(out, path.Clean(e.TargetPath))
	}
	return out
}

// ocEntriesOfKind returns the entries of one kind.
func ocEntriesOfKind(m domain.Manifest, kind domain.ArtifactKind) []domain.ManifestEntry {
	var out []domain.ManifestEntry
	for _, e := range m.Entries {
		if e.Ref.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}
