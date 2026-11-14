package app_test

// owned_content_write_basis_test.go specifies that the Deploy-owned content record written to
// the manifest with an agent equals the record the probe computes from the written file, for a
// Markdown harness and for Codex, on every path that writes agents: create, update after a
// hand edit, conflict overwrite, backup-then-overwrite, dry run, transform/rebuild and the
// GHCP agent migration. The expected value is always the record of the canonical (decoded)
// form of the file, so a record taken over the raw bytes of a non-Markdown file fails.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/manifest"
	"mosaic-deploy/internal/transform"
)

func TestWriteBasis_CreateFlows_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	for _, h := range owHarnesses {
		for _, f := range owCreateFlows(t) {
			t.Run(h.name+"/"+f.name, func(t *testing.T) {
				spy := &owSpy{}

				if _, err := f.run(t, owService(t, spy), h, t.TempDir(), owFlowOpts{}); err != nil {
					t.Fatalf("%s: %v", f.name, err)
				}

				owAssertChannel(t, spy)
				// The workspace of the run is the one the spy's plan was built for.
				owAssertRecordsMatchDisk(t, h, spy.lastPlan(t).WorkspacePath, h.keysOf(f.writes))
			})
		}
	}
}

func TestWriteBasis_RewriteFlows_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	for _, h := range owHarnesses {
		for _, f := range owFlows() {
			t.Run(h.name+"/"+f.name, func(t *testing.T) {
				ws := owDeploy(t, h)
				for _, key := range h.keysOf(f.writes) {
					owEditBody(t, h, ws, key, owOutsideEdit(t))
				}
				spy := &owSpy{}

				if _, err := f.run(t, owService(t, spy), h, ws, owFlowOpts{}); err != nil {
					t.Fatalf("%s: %v", f.name, err)
				}

				if got := owPlanItem(t, spy.lastPlan(t), h.keysOf(f.writes)[0]).Action; got != domain.ActionUpdate {
					t.Fatalf("action of %s = %s; want %s so the update write path is exercised", h.keysOf(f.writes)[0], got, domain.ActionUpdate)
				}
				owAssertChannel(t, spy)
				owAssertRecordsMatchDisk(t, h, ws, h.keysOf(f.writes))
			})
		}
	}
}

// owConflictWorkspace deploys, drops the manifest and hand-edits the runner, so the next
// deploy finds a locally modified file no record vouches for.
func owConflictWorkspace(t *testing.T, h owHarness) string {
	t.Helper()
	ws := owDeploy(t, h)
	owEditBody(t, h, ws, owRunner, owOutsideEdit(t))
	if err := os.Remove(filepath.Join(ws, manifest.Dir, manifest.FileName)); err != nil {
		t.Fatalf("remove manifest: %v", err)
	}
	return ws
}

func TestWriteBasis_ConflictResolvedByWriting_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	decisions := []domain.ConflictDecision{domain.DecisionOverwrite, domain.DecisionBackupThenOverwrite}
	for _, h := range owHarnesses {
		for _, d := range decisions {
			t.Run(h.name+"/"+string(d), func(t *testing.T) {
				ws := owConflictWorkspace(t, h)
				spy := &owSpy{}

				_, err := owFlowNamed(t, "DeployNew").run(t, owService(t, spy), h, ws, owFlowOpts{conflict: d})

				if err != nil {
					t.Fatalf("DeployNew over a locally modified file: %v", err)
				}
				if got := owPlanItem(t, spy.lastPlan(t), owRunner).Action; got != domain.ActionConflict {
					t.Fatalf("action of %s = %s; want %s so the conflict write path is exercised", owRunner, got, domain.ActionConflict)
				}
				owAssertChannel(t, spy)
				owAssertRecordsMatchDisk(t, h, ws, []string{owRunner})
			})
		}
	}
}

func TestWriteBasis_DryRun_SimulatedRecordIsRecordOfCanonicalForm(t *testing.T) {
	for _, h := range owHarnesses {
		t.Run(h.name, func(t *testing.T) {
			spy := &owSpy{}

			_, err := owFlowNamed(t, "DeployNew").run(t, owService(t, spy), h, t.TempDir(), owFlowOpts{dry: true})

			if err != nil {
				t.Fatalf("dry-run DeployNew: %v", err)
			}
			owAssertChannel(t, spy)
			exec := spy.lastExec(t)
			for _, key := range h.keysOf(owAllKeys) {
				entry, ok := exec.res.Manifest.LookupAt(domain.ArtifactRef{Kind: domain.ArtifactAgent, Key: key}, h.rel(key))
				written, have := exec.contents[h.rel(key)]
				if !ok || !have {
					t.Fatalf("%s: simulated entry found = %v, content computed = %v", key, ok, have)
				}
				want := owCanonicalRecord(t, h, written)
				if entry.Owned == nil || entry.Owned.Hash != want.Hash || len(entry.Owned.Regions) != len(want.Regions) {
					t.Errorf("%s: simulated Owned = %+v; want the record of the canonical form %+v", key, entry.Owned, want)
				}
			}
		})
	}
}

func TestWriteBasis_TransformAndRebuild_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	pairs := []struct {
		name     string
		src, tgt owHarness
		key      string
		seed     func(t *testing.T, r *trRun)
	}{
		{name: "MarkdownToCodex", src: owHarnesses[0], tgt: owHarnesses[1], key: "code-reviewer",
			seed: func(t *testing.T, r *trRun) {
				r.put(hClaude, "code-reviewer", string(rbFixture(t, "claude-code-full.md")))
			}},
		{name: "CodexToMarkdown", src: owHarnesses[1], tgt: owHarnesses[0], key: "worker",
			seed: func(t *testing.T, r *trRun) {
				cxPut(t, r, "worker.toml", codexSourceAgent(cxOpts{name: "worker", model: cxSourceModelSub, role: "subagent"}))
			}},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			spy := &owSpy{}
			r := trNew(t, nil, p.src.id, p.tgt.id, withCodexTarget(t), owSpyDeps(spy))
			p.seed(t, r)
			req := r.request()
			req.TargetModel = p.tgt.model
			req.ModelMap = app.TransformModelMapping{cxSourceModelSub: p.tgt.model, cxSourceModelOrch: p.tgt.model}

			res := r.mustRun(req)

			if f := trFile(t, res, p.key); f.Status != app.StatusTransformed {
				t.Fatalf("status = %q (reason %q); want transformed", f.Status, f.Reason)
			}
			owAssertChannel(t, spy)
			// The probe of the follow-up run covers catalog artifacts only, so the record is
			// compared with the canonical form of the written file.
			owAssertRecords(t, p.tgt, r.ws, []string{p.key}, false)
		})
	}
}

func TestWriteBasis_GhcpAgentMigration_RecordEqualsProbeOfWrittenFile(t *testing.T) {
	ghcp := owHarness{name: "GHCP", id: hCLI, model: parityModel, dir: gamAgentsDir, ext: ".agent.md"}
	for _, f := range gamFlows() {
		t.Run(f.name, func(t *testing.T) {
			ws, legacy := gamLegacyWorkspace(t, hCLI)
			spy := &owSpy{}

			if err := f.run(t, ws, hCLI, gamOpts{deps: []func(*app.Deps){owSpyDeps(spy)}}); err != nil {
				t.Fatalf("%s over a legacy workspace: %v", f.name, err)
			}

			// The legacy files are current, so every migrated agent is a rename with nothing
			// stale: the executor writes its legacy bytes verbatim (the verbatim-keep path),
			// and the record must still be that of the written file.
			for _, key := range f.migrates {
				item := owPlanItem(t, spy.lastPlan(t), key)
				if item.RenamedFrom == "" || len(item.Stale) != 0 {
					t.Fatalf("%s: RenamedFrom = %q, Stale = %v; want a rename with nothing stale", key, item.RenamedFrom, item.Stale)
				}
				if got := gamRead(t, ws, gamRel(key, ".agent.md")); string(got) != string(legacy[key]) {
					t.Errorf("%s: migrated bytes differ from the legacy bytes; an unchanged rename must keep them verbatim", key)
				}
			}
			owAssertChannel(t, spy)
			owAssertRecordsMatchDisk(t, ghcp, ws, f.migrates)
		})
	}
}

// ---------------------------------------------------------------------------
// The record value the application supplies to the executor
// ---------------------------------------------------------------------------

// ownedChannelCapture runs DeployNew over the Codex test harness with an executor that only
// captures the request, and returns the captured request and the agent plan item.
func ownedChannelCapture(t *testing.T) (deploy.ExecRequest, domain.PlanItem) {
	t.Helper()
	stub := interactiontest.NewBuilder().AnswerReview(true).Build()
	deps, workspace := newCodexOpDeps(t, stub)
	deps.Planner = &stubPlanner{plan: newCodexOpAgentPlan(workspace, domain.ActionCreate)}
	exec := &capturingExecutor{}
	deps.Executor = exec

	if _, err := app.New(deps).DeployNew(context.Background(), codexOpDeployRequest(workspace)); err != nil {
		t.Fatalf("DeployNew: %v", err)
	}

	if exec.capturedReq == nil || exec.capturedReq.AgentOwnedContent == nil {
		t.Fatal("the executor request carries no AgentOwnedContent")
	}
	return *exec.capturedReq, exec.capturedReq.Plan.Items[0]
}

func TestAgentOwnedContent_CodexBytes_IsRecordOfDecodedForm(t *testing.T) {
	req, item := ownedChannelCapture(t)
	written := codexOwnedFileForApp()

	got := req.AgentOwnedContent(item, written)

	want := owCanonicalRecord(t, owHarnesses[1], written)
	if got == nil || got.Hash != want.Hash || len(got.Regions) != len(want.Regions) || len(want.Regions) < 2 {
		t.Errorf("AgentOwnedContent = %+v; want the record of the decoded form %+v", got, want)
	}
}

func TestAgentOwnedContent_UndecodableBytes_ReturnsNil(t *testing.T) {
	req, item := ownedChannelCapture(t)

	got := req.AgentOwnedContent(item, []byte("this = = is not toml\n"))

	if got != nil {
		t.Errorf("AgentOwnedContent = %+v; want nil (no record) for bytes the translator cannot decode", got)
	}
}

// codexOwnedFileForApp is a Codex agent file with a Deploy-owned region and a project region.
func codexOwnedFileForApp() []byte {
	return []byte("# mosaic_version: 1.0\n\nname = \"codex-op-agent\"\nsandbox_mode = \"read-only\"\n" +
		"developer_instructions = '''\nOutside.\n\n<Identity type=\"core\">\nIdentity prose.\n" +
		"<IdentityExtension type=\"project\">\nTeam.\n</IdentityExtension>\n</Identity>\n'''\n")
}

// TestUpdate_HarnessOnlyRefresh_RequestCarriesAgentRecordChannel verifies the Update flow's
// executor request carries the record channel when it refreshes a harness-only agent, and
// that the channel's record of the refreshed bytes is the record of their canonical form.
func TestUpdate_HarnessOnlyRefresh_RequestCarriesAgentRecordChannel(t *testing.T) {
	const agentFileName = "my-harness-agent.md"
	agentsDir := harnessOnlyTestAgentsDir
	stub := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QHarnessOnlyRefreshScope, filepath.Join(agentsDir, agentFileName), string(app.RefreshProtocolOnly)).
		AnswerReview(true).
		Build()
	deps, workspace := newBaseDepsWithAgentsDir(t, stub, agentsDir)
	writeAgentFile(t, filepath.Join(workspace, agentsDir), agentFileName, harnessOnlyAgentContent())
	exec := &capturingExecutor{}
	deps.Executor = exec

	_, _ = app.New(deps).Update(context.Background(), app.UpdateRequest{
		HarnessID: "stub-harness", WorkspacePath: workspace, AutoConfirmPlan: true})

	if exec.capturedReq == nil || exec.capturedReq.AgentOwnedContent == nil {
		t.Fatal("the Update executor request carries no AgentOwnedContent")
	}
	for _, item := range exec.capturedReq.Plan.Items {
		if item.Ref.Kind != domain.ArtifactAgent ||
			!strings.HasSuffix(filepath.ToSlash(item.TargetPath), "/"+agentFileName) {
			continue
		}
		content, err := exec.capturedReq.Content(item)
		if err != nil {
			t.Fatalf("Content(harness-only agent): %v", err)
		}
		want, err := transform.OwnedContentOf(domain.ArtifactAgent, content)
		got := exec.capturedReq.AgentOwnedContent(item, content)
		if err != nil || got == nil || got.Hash != want.Hash {
			t.Errorf("AgentOwnedContent = %+v (err %v); want %+v", got, err, want)
		}
		return
	}
	t.Fatal("the plan holds no harness-only agent item")
}
