package transform_test

// harness_region_warning_test.go specifies the missing-region warning (orchestrator content
// with no region to receive it) and the behaviour with an older external module that serves
// merged content under HarnessConstraints only.

import (
	"bytes"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

func TestMissingRegionWarning_OrchestratorSourceWithoutSecondMarker_WarnsAndTransforms(t *testing.T) {
	for _, key := range orchestratorKeys {
		t.Run(key, func(t *testing.T) {
			mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

			doc, result := regionApply(t, mod, sourceSingleMarker, key, domain.RoleOrchestrator, "")

			gaps := undeliveredGaps(result.Report.Gaps)
			if len(gaps) != 1 {
				t.Fatalf("got %d undelivered-orchestrator-injection gaps, want exactly 1: %+v", len(gaps), result.Report.Gaps)
			}
			g := gaps[0]
			if g.Subject != docformat.OrchestratorHarnessRegionName {
				t.Errorf("gap Subject = %q, want %q", g.Subject, docformat.OrchestratorHarnessRegionName)
			}
			if g.Owner != key {
				t.Errorf("gap Owner = %q, want the agent key %q", g.Owner, key)
			}
			for _, want := range []string{key, docformat.OrchestratorHarnessRegionName, "HarnessInjectionsOrchestrator.md"} {
				if !strings.Contains(g.Detail, want) {
					t.Errorf("gap Detail %q does not name %q", g.Detail, want)
				}
			}
			// The warning must identify the undelivered content itself, not only its file:
			// an excerpt of the text that had no region to receive it.
			if !strings.Contains(g.Detail, "ORCH_REGION_TEXT") {
				t.Errorf("gap Detail %q does not identify the undelivered content (excerpt of %q)", g.Detail, orchRegionText)
			}
			shared := mustRegion(t, doc, docformat.HarnessRegionName)
			if !bytes.Contains(shared.Content(), []byte(sharedRegionText)) {
				t.Errorf("HarnessConstraints lost the shared text: %q", shared.Content())
			}
			if bytes.Contains(result.Output, []byte(orchRegionText)) {
				t.Error("orchestrator text was merged into the output although no region exists for it")
			}
		})
	}
}

func TestMissingRegionWarning_NoWarning_WhenOrchestratorFileHasNoContent(t *testing.T) {
	cases := map[string]domain.HarnessModule{
		"empty file":      newSplitModule(t, sharedRegionText, "", true),
		"no file at all":  newSplitModule(t, sharedRegionText, "", false),
		"merged-only mod": newMergedOnlyModule(t, sharedRegionText, ""),
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			_, result := regionApply(t, mod, sourceSingleMarker, "orchestrator", domain.RoleOrchestrator, "")

			assertNoUndeliveredGap(t, result.Report.Gaps)
		})
	}
}

func TestMissingRegionWarning_NoWarning_WhenSourceHasSecondMarker(t *testing.T) {
	mod := newSplitModule(t, sharedRegionText, orchRegionText, true)

	_, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, "")

	assertNoUndeliveredGap(t, result.Report.Gaps)
}

func TestMissingRegionWarning_NoWarning_ForSubagentWithoutSecondMarker(t *testing.T) {
	// A module that would hand out orchestrator text for any role must not trigger the
	// warning for a subagent: the condition is orchestrator-role only.
	mod := &regionModule{HarnessModule: newFixtureModule(t), inject: func(req domain.InjectionRequest) (string, bool) {
		if req.Name == docformat.OrchestratorHarnessRegionName {
			return orchRegionText, true
		}
		return sharedRegionText, true
	}}

	_, result := regionApply(t, mod, sourceSingleMarker, "worker", domain.RoleSubagent, "")

	assertNoUndeliveredGap(t, result.Report.Gaps)
}

func TestMergedOnlyModule_OrchestratorSource_NothingLostOrDuplicated_SecondRegionEmpty(t *testing.T) {
	mod := newMergedOnlyModule(t, sharedRegionText, orchRegionText)

	doc, result := regionApply(t, mod, orchestratorSourceBothMarkers, "orchestrator", domain.RoleOrchestrator, "")

	for _, text := range []string{sharedRegionText, orchRegionText} {
		if n := strings.Count(string(result.Output), text); n != 1 {
			t.Errorf("text %q appears %d times, want exactly once", text, n)
		}
	}
	shared := mustRegion(t, doc, docformat.HarnessRegionName)
	if !bytes.Contains(shared.Content(), []byte(sharedRegionText)) || !bytes.Contains(shared.Content(), []byte(orchRegionText)) {
		t.Errorf("HarnessConstraints = %q, want the merged content the module serves", shared.Content())
	}
	second := mustRegion(t, doc, docformat.OrchestratorHarnessRegionName)
	if strings.TrimSpace(string(second.Content())) != "" {
		t.Errorf("OrchestratorHarnessConstraints = %q, want empty", second.Content())
	}
	if got := shared.Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
	}
	if got := second.Version(); got != orchRegionVersion {
		t.Errorf("empty second region version = %q, want %q like every other harness region", got, orchRegionVersion)
	}
	assertNoUndeliveredGap(t, result.Report.Gaps)
}

func TestMergedOnlyModule_SourceWithoutSecondMarker_NothingLost(t *testing.T) {
	mod := newMergedOnlyModule(t, sharedRegionText, orchRegionText)

	doc, result := regionApply(t, mod, sourceSingleMarker, "orchestrator", domain.RoleOrchestrator, "")

	shared := mustRegion(t, doc, docformat.HarnessRegionName)
	if !bytes.Contains(shared.Content(), []byte(orchRegionText)) {
		t.Errorf("merged orchestrator text was lost: %q", shared.Content())
	}
	if n := strings.Count(string(result.Output), orchRegionText); n != 1 {
		t.Errorf("orchestrator text appears %d times, want exactly once", n)
	}
	if got := shared.Version(); got != sharedRegionVersion {
		t.Errorf("HarnessConstraints version = %q, want %q", got, sharedRegionVersion)
	}
}
