package app_test

// harness_region_rebuild_test.go specifies that the deployed-agent rebuild fills the two
// orchestrator harness regions like deploy and render do: each region carries its own file's
// plain version, an emptied region keeps its tag only when its file exists, and an orchestrator
// source without the second marker gets one warning in the rebuild result with the
// orchestrator text left out of the shared region. The target module serves explicit, distinct
// versions so the tests do not depend on the real descriptor values.

import (
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// rebuildRegionModule wraps a real built-in module, replacing the injection versions its
// descriptor reports and the per-region content it serves.
type rebuildRegionModule struct {
	domain.HarnessModule
	orchText    string
	orchOK      bool
	orchVersion string
}

func (m *rebuildRegionModule) Descriptor() *domain.HarnessDescriptor {
	d := *m.HarnessModule.Descriptor()
	d.InjectionsVersion = stampSharedVersion
	d.OrchestratorInjectionsVersion = m.orchVersion
	return &d
}

func (m *rebuildRegionModule) Injection(req domain.InjectionRequest) (string, bool) {
	switch req.Name {
	case docformat.HarnessRegionName:
		return stampSharedText, true
	case docformat.OrchestratorHarnessRegionName:
		if req.Role == domain.RoleOrchestrator {
			return m.orchText, m.orchOK
		}
	}
	return "", false
}

// withSecondHarnessRegion adds a stale OrchestratorHarnessConstraints region after the source's
// HarnessConstraints region, the shape of an orchestrator deployed with both markers.
var withSecondHarnessRegion = withSourceEdit(replaceText("</HarnessConstraints>",
	"</HarnessConstraints>\n\n<OrchestratorHarnessConstraints type=\"managed\" version=\"0.1.0\">\nSTALE-ORCH-TEXT\n</OrchestratorHarnessConstraints>"))

// rebuildOrchestrator rebuilds the Claude Code fixture as the main orchestrator for the stub
// target and returns the result and the parsed output.
func rebuildOrchestrator(t *testing.T, mod *rebuildRegionModule, secondMarker bool) (string, []domain.Gap) {
	t.Helper()
	opts := []rbOpt{withFileName("orchestrator.md"), withoutRoleStamp}
	if secondMarker {
		opts = append(opts, withSecondHarnessRegion)
	}
	in := rbInput(t, claudeToClaude, false, opts...)
	in.TargetModule = mod
	res, out := rbRebuild(t, in)
	return out, res.Gaps
}

func TestRebuild_OrchestratorWithBothMarkers_EachRegionTaggedWithItsFileVersion(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}

	out, gaps := rebuildOrchestrator(t, mod, true)

	assertRegionStamps(t, "rebuild", []byte(out))
	if n := countGaps(gaps, domain.GapUndeliveredOrchestratorInjection); n != 0 {
		t.Errorf("source with both markers must give no undelivered-orchestrator-injection gap, got %d in %+v", n, gaps)
	}
	if strings.Contains(out, "STALE-ORCH-TEXT") {
		t.Errorf("stale deployed orchestrator region text survived the rebuild:\n%s", out)
	}
}

func TestRebuild_OrchestratorEmptyOrchestratorFile_SecondRegionEmptyAndTagged(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: "", orchOK: true, orchVersion: stampOrchVersion}

	out, _ := rebuildOrchestrator(t, mod, true)

	doc, err := docformat.Parse([]byte(out))
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	orch, ok := doc.Body().Deployed(docformat.OrchestratorHarnessRegionName)
	if !ok {
		t.Fatalf("OrchestratorHarnessConstraints region absent:\n%s", out)
	}
	if strings.TrimSpace(string(orch.Content())) != "" || strings.Contains(out, "STALE-ORCH-TEXT") {
		t.Errorf("OrchestratorHarnessConstraints content = %q, want empty", orch.Content())
	}
	if got := orch.Version(); got != stampOrchVersion {
		t.Errorf("empty region version = %q, want %q", got, stampOrchVersion)
	}
}

func TestRebuild_OrchestratorNoOrchestratorFile_SecondRegionEmptyAndUntagged(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: "", orchOK: false, orchVersion: ""}

	out, _ := rebuildOrchestrator(t, mod, true)

	doc, err := docformat.Parse([]byte(out))
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	orch, ok := doc.Body().Deployed(docformat.OrchestratorHarnessRegionName)
	if !ok {
		t.Fatalf("OrchestratorHarnessConstraints region absent:\n%s", out)
	}
	if got := orch.Version(); got != "" {
		t.Errorf("region without an orchestrator file has version %q, want no version attribute", got)
	}
	shared, ok := doc.Body().Deployed(docformat.HarnessRegionName)
	if !ok || shared.Version() != stampSharedVersion {
		t.Errorf("HarnessConstraints must carry %q regardless of the missing file:\n%s", stampSharedVersion, out)
	}
}

func TestRebuild_OrchestratorSourceWithoutSecondMarker_WarnsOnceAndLeavesTextOut(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: rbModule(t, hClaude), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}

	out, gaps := rebuildOrchestrator(t, mod, false)

	assertOneUndeliveredWarning(t, "orchestrator", gaps)
	if strings.Contains(out, stampOrchText) {
		t.Errorf("orchestrator text was merged into the output although the source has no region for it:\n%s", out)
	}
	if !strings.Contains(out, stampSharedText) {
		t.Errorf("shared text missing from the rebuilt HarnessConstraints:\n%s", out)
	}
}
