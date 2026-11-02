package app_test

// transform_ghcp_pair_exec_test.go specifies the end state of a real transform between the two
// GHCP targets, in both directions, over a workspace with recorded X.agent.md agents, recorded
// legacy X.md agents, and an X.agent.md beside a legacy X.md: the workspace holds only the
// target deployment, every removed or replaced file can be recovered from .mosaic/backups, and
// the manifest and TODO file describe the target.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
)

// trPairEach runs the body once per direction and recorded-deployment shape.
func trPairEach(t *testing.T, body func(t *testing.T, r *trRun, c trPairExecCase)) {
	t.Helper()
	for _, c := range trPairExecCases {
		for _, pair := range trPairDirections {
			t.Run(c.name+" "+pair[0]+"->"+pair[1], func(t *testing.T) {
				body(t, trPairExecRun(t, pair[0], pair[1], c), c)
			})
		}
	}
}

func TestTransformGhcpPair_RealRunLeavesOneTargetAgentFileAndNoLegacyFile(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		source := r.read(c.agentRel)

		r.mustRun(r.request())

		got := trFilesIn(t, r.ws, ".github/agents")
		want := []string{"mine.agent.md", "notes.md", "worker.agent.md"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("agents folder = %v, want %v: one X.agent.md per transformed agent, no legacy X.md", got, want)
		}
		written := r.read(trPairInPlaceAgent)
		if written == source || written == trPairBesideAgent {
			t.Errorf("%s still holds the source content", trPairInPlaceAgent)
		}
		if !trHasKey(written, "mosaic_harness_version") {
			t.Errorf("%s does not carry the harness stamp:\n%s", trPairInPlaceAgent, written)
		}
		if !strings.Contains(written, "You are the **CodeReviewer** agent.") {
			t.Errorf("%s lost the agent body:\n%s", trPairInPlaceAgent, written)
		}
	})
}

func TestTransformGhcpPair_RealRunInstallsTargetHookFilesAndRegistrationAndDropsSourceOnlyFiles(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		hooks := r.hooksDir(r.tgt)

		r.mustRun(r.request())

		if got, want := r.read(hooks+"/logger.sh"), trReadTestdata(t, "hooks/flat/logger.sh"); got != want {
			t.Errorf("%s/logger.sh holds %q, want the target variant file %q", hooks, got, want)
		}
		if r.exists(hooks + "/" + trPairSourceOnlyFile) {
			t.Errorf("source-variant-only hook file %s/%s is still there", hooks, trPairSourceOnlyFile)
		}
		if got, want := r.read(trPairRegistrationFile), "{\"hooks\":[\"logger\"]}\n"; got != want {
			t.Errorf("registration file holds %q, want the target fragment %q (the user edit is replaced)", got, want)
		}
	})
}

func TestTransformGhcpPair_RealRunLeavesUnrecordedUserFilesAndSkillsUnchanged(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		kept := map[string]string{
			trPairUserAgent:     r.read(trPairUserAgent),
			trPairUserNote:      r.read(trPairUserNote),
			trPairUserSkill:     r.read(trPairUserSkill),
			trPairUnrelatedPath: r.read(trPairUnrelatedPath),
		}
		for rel, content := range r.snapshot() {
			if strings.HasPrefix(rel, ".github/skills/") {
				kept[rel] = content
			}
		}

		r.mustRun(r.request())

		for rel, want := range kept {
			if got := r.read(rel); got != want {
				t.Errorf("%s changed: got %q, want %q", rel, got, want)
			}
			if n := len(trBackupsOf(t, r.ws, rel)); n != 0 {
				t.Errorf("%s has %d backups, want none for a file that is neither removed nor replaced", rel, n)
			}
		}
	})
}

func TestTransformGhcpPair_RealRunLeavesTheManifestNamingTheTarget(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		r.mustRun(r.request())

		m := trLoadManifest(t, r.ws)
		if m.HarnessID != r.tgt {
			t.Errorf("harness_id = %q, want the target %q", m.HarnessID, r.tgt)
		}
		agents := trEntryByRef(m, domain.ArtifactAgent, "worker")
		if len(agents) != 1 || agents[0].TargetPath != trPairInPlaceAgent {
			t.Fatalf("agent entries = %+v, want exactly one at %s", agents, trPairInPlaceAgent)
		}
		if want := manifest.Hash([]byte(r.read(trPairInPlaceAgent))); agents[0].ContentHash != want {
			t.Errorf("agent content hash = %q, want the hash of the written bytes %q", agents[0].ContentHash, want)
		}
		if _, ok := trEntryAt(m, trPairLegacyAgent); ok {
			t.Errorf("manifest still has an entry at the removed %s", trPairLegacyAgent)
		}
		if len(trEntryByRef(m, domain.ArtifactHook, trPairHookKey)) != 1 {
			t.Errorf("manifest hook entries = %+v, want one for %s", trEntryByRef(m, domain.ArtifactHook, trPairHookKey), trPairHookKey)
		}
		got, ok := trEntryAt(m, trPairUnrelatedPath)
		want := trPairUnrelatedEntry()
		if !ok || got.Ref != want.Ref || got.Version != want.Version || got.ContentHash != want.ContentHash {
			t.Errorf("unrelated entry = %+v (found %v), want it kept as %+v", got, ok, want)
		}
	})
}

func TestTransformGhcpPair_RealRunTodoFileHasNoRegistrationGapAndNoBothFormsStep(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		res := r.mustRun(r.request())

		todo := trReadTodo(t, res)
		for _, stale := range []string{trPairRegistrationFile, "Register the logger hook", "worker.md"} {
			if strings.Contains(todo, stale) {
				t.Errorf("TODO file still mentions %q although the run replaced it:\n%s", stale, todo)
			}
		}
		if n := trNotes(res, "both-forms"); len(n) != 0 {
			t.Errorf("notes = %+v, want no both-forms note: the legacy file was migrated", n)
		}
	})
}

func TestTransformGhcpPair_RealRunBacksUpEveryRemovedAndReplacedFileByteForByte(t *testing.T) {
	trPairEach(t, func(t *testing.T, r *trRun, c trPairExecCase) {
		hooks := r.hooksDir(r.src)
		original := map[string]string{
			c.agentRel:                         r.read(c.agentRel),
			hooks + "/logger.sh":               r.read(hooks + "/logger.sh"),
			hooks + "/" + trPairSourceOnlyFile: r.read(hooks + "/" + trPairSourceOnlyFile),
			trPairRegistrationFile:             r.read(trPairRegistrationFile),
		}
		if c.beside {
			original[trPairInPlaceAgent] = trPairBesideAgent
		}

		res := r.mustRun(r.request())

		for rel, want := range original {
			trAssertRecoverable(t, r.ws, rel, want)
		}
		for _, ch := range append(append([]app.TransformFileChange{}, res.Removals...), res.Replacements...) {
			if _, ok := original[ch.Path]; !ok {
				t.Errorf("result reports %s as removed or replaced, but no such file was part of the source deployment", ch.Path)
			}
			trAssertBackedUp(t, ch, original[ch.Path])
		}
	})
}
