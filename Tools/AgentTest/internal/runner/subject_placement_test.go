package runner_test

// Tests for where the subject definition lies after provisioning. The subject
// is launched with `--agent <identity>`, which Claude Code resolves by the
// definition's `name:` among the project-level agents under
// <SubjectDir>/.claude/agents/. A subject provisioned anywhere else, or whose
// name differs from its identity, would not be found, so setup must stop with
// an error that names the subject and its definition file before any launch.
//
// The deployer doubles copy tool-local fixtures from testdata/placement into
// the sandbox, so the check reads real files written where the double reports.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/runner"
)

const (
	placementIdentity = "orchestrator"
	// placementFileName differs from the identity on purpose: the file name is
	// irrelevant to resolution, so an error naming the file proves it carries
	// the file and not just the subject.
	placementFileName = "lead-agent.md"

	fixtureMatchingName   = "subject_named_orchestrator.md"
	fixtureDifferentName  = "subject_named_differently.md"
	fixtureWithoutAName   = "subject_without_name.md"
	projectAgentsRelative = ".claude/agents"
)

// copyPlacementFixture copies a fixture from testdata/placement to dest,
// creating the destination directory.
func copyPlacementFixture(t *testing.T, fixture, dest string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "placement", fixture))
	if err != nil {
		t.Fatalf("reading placement fixture %q: %v", fixture, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("creating %q: %v", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatalf("writing %q: %v", dest, err)
	}
}

// subjectDestination returns where the doubles place the subject file: under
// workspaceRoot, in relDir, named placementFileName.
func subjectDestination(workspaceRoot, relDir string) string {
	return filepath.Join(workspaceRoot, filepath.FromSlash(relDir), placementFileName)
}

// placeSubjectViaDeploy scripts the catalogue path: Deploy writes fixture
// into relDir of the workspace and reports it for the subject key.
func placeSubjectViaDeploy(t *testing.T, h *harness, fixture, relDir string) {
	t.Helper()
	h.Adapter.agentsDir = projectAgentsRelative
	h.Deployer.deployFn = func(req domain.DeployRequest) (domain.DeployResult, error) {
		dest := subjectDestination(req.WorkspaceRoot, relDir)
		copyPlacementFixture(t, fixture, dest)
		return domain.DeployResult{Agents: []domain.DeployedAgent{{Key: placementIdentity, DestinationPath: dest}}}, nil
	}
}

// placeSubjectViaRender scripts the stub path: Render of the subject key
// writes fixture into relDir of the workspace and reports it; stub renders keep
// the double's default result.
func placeSubjectViaRender(t *testing.T, h *harness, fixture, relDir string) {
	t.Helper()
	h.Adapter.agentsDir = projectAgentsRelative
	h.Deployer.renderFn =func(req domain.RenderAgentRequest) (domain.RenderAgentResult, error) {
		if req.CatalogAgentKey == "" {
			return domain.RenderAgentResult{
				DestinationPath: filepath.Join(req.WorkspaceRoot, "stub.md"),
				AgentKey:        "stub",
			}, nil
		}
		dest := subjectDestination(req.WorkspaceRoot, relDir)
		copyPlacementFixture(t, fixture, dest)
		return domain.RenderAgentResult{DestinationPath: dest, AgentKey: req.CatalogAgentKey}, nil
	}
}

func placementCatalogueRequest(testID string) runner.Request {
	req := catalogueRequestWithModels(testID, "", "", domain.ModelSelection{Subject: selectedSubjectModel})
	req.Test.Definition.Subject.Identity = placementIdentity
	return req
}

func placementRenderRequest(testID string) runner.Request {
	req := renderPathRequest(testID, domain.ModelSelection{Subject: selectedSubjectModel})
	req.Test.Definition.Subject.Identity = placementIdentity
	return req
}

// placementPath is one provisioning path under test.
type placementPath struct {
	name    string
	request func(testID string) runner.Request
	place   func(t *testing.T, h *harness, fixture, relDir string)
}

func placementPaths() []placementPath {
	return []placementPath{
		{"catalogue path", placementCatalogueRequest, placeSubjectViaDeploy},
		{"stub path", placementRenderRequest, placeSubjectViaRender},
	}
}

func TestSubjectPlacement_ProjectAgentWithMatchingNameLetsTheRunProceed(t *testing.T) {
	for _, p := range placementPaths() {
		t.Run(p.name, func(t *testing.T) {
			h := newHarness(t)
			p.place(t, h, fixtureMatchingName, projectAgentsRelative)

			runProceeds(t, h, p.request("placement-ok"))
		})
	}
}

// runAbortedByPlacement runs the request and requires a setup error that names
// the subject and its definition file, with no spawn plan and no launch.
func runAbortedByPlacement(t *testing.T, h *harness, req runner.Request) {
	t.Helper()
	_, err := runner.Run(context.Background(), h.Deps, req, nil)
	if err == nil {
		t.Fatal("Run returned no error, want a setup error for a subject that --agent cannot resolve")
	}
	for _, want := range []string{placementIdentity, placementFileName} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Run error = %q, want it to name %q", err.Error(), want)
		}
	}
	calls := h.Rec.all()
	for _, c := range calls {
		if c == "adapter.SpawnPlan" || c == "launcher.Launch" {
			t.Errorf("%s was called although the subject cannot be resolved; calls: %v", c, calls)
		}
	}
	if !containsPath(calls, "workspace.Teardown") {
		t.Errorf("workspace.Teardown not recorded after the setup error; calls: %v", calls)
	}
}

func TestSubjectPlacement_SubjectOutsideTheProjectAgentsDirectoryIsASetupError(t *testing.T) {
	for _, p := range placementPaths() {
		for _, relDir := range []string{"agents", "."} {
			t.Run(p.name+" in "+relDir, func(t *testing.T) {
				h := newHarness(t)
				p.place(t, h, fixtureMatchingName, relDir)

				runAbortedByPlacement(t, h, p.request("placement-elsewhere"))
			})
		}
	}
}

func TestSubjectPlacement_SubjectWhoseNameDiffersFromItsIdentityIsASetupError(t *testing.T) {
	for _, p := range placementPaths() {
		for _, fixture := range []string{fixtureDifferentName, fixtureWithoutAName} {
			t.Run(p.name+" "+fixture, func(t *testing.T) {
				h := newHarness(t)
				p.place(t, h, fixture, projectAgentsRelative)

				runAbortedByPlacement(t, h, p.request("placement-name"))
			})
		}
	}
}

func TestSubjectPlacement_ReportedSubjectFileThatDoesNotExistIsASetupError(t *testing.T) {
	h := newHarness(t)
	h.Adapter.agentsDir = projectAgentsRelative
	h.Deployer.deployFn = func(req domain.DeployRequest) (domain.DeployResult, error) {
		dest := subjectDestination(req.WorkspaceRoot, projectAgentsRelative)
		return domain.DeployResult{Agents: []domain.DeployedAgent{{Key: placementIdentity, DestinationPath: dest}}}, nil
	}

	runAbortedByPlacement(t, h, placementCatalogueRequest("placement-missing-file"))
}
