package app_test

// deploy_injection_stamps_test.go specifies the version stamps a deploy hands to the executor
// for the manifest: every agent carries HarnessInjections.md's version in the injections
// field, and only orchestrators additionally carry HarnessInjectionsOrchestrator.md's version
// in the orchestrator field. The stamps equal the region tags written into the same files.

import (
	"context"
	"testing"

	"mosaic-deploy/internal/deploy"
	"mosaic-deploy/internal/domain"
)

// stampCapturingExecutor records the version stamps of the request and renders the content of
// every agent item the way the real executor does before it writes a file.
type stampCapturingExecutor struct {
	stamps   map[string]domain.VersionStamp
	contents map[string][]byte
}

func (e *stampCapturingExecutor) Execute(_ context.Context, req deploy.ExecRequest) (deploy.ExecResult, error) {
	e.stamps = req.VersionStamps
	e.contents = make(map[string][]byte)
	for _, item := range req.Plan.Items {
		if item.Ref.Kind != domain.ArtifactAgent {
			continue
		}
		content, err := req.Content(item)
		if err != nil {
			return deploy.ExecResult{}, err
		}
		e.contents[item.TargetPath] = content
	}
	return deploy.ExecResult{DeploymentRoot: req.Plan.WorkspacePath}, nil
}

func TestDeployBuild_ManifestStamps_OrchestratorsCarryBothVersionsSubagentSharedOnly(t *testing.T) {
	_, _, deps, workspace := deployCaptureDeps(t, stampSourceBothMarkers, 1)
	exec := &stampCapturingExecutor{}
	deps.Executor = exec

	runDeployCapture(t, deps, workspace)

	for _, target := range []string{"orchestrator.md", "orchestrator-script.md"} {
		stamp, ok := exec.stamps[target]
		if !ok {
			t.Fatalf("no version stamp for %s", target)
		}
		if stamp.InjectionsVersion != stampSharedVersion {
			t.Errorf("%s: InjectionsVersion = %q, want the HarnessInjections.md version %q", target, stamp.InjectionsVersion, stampSharedVersion)
		}
		if stamp.OrchestratorInjectionsVersion != stampOrchVersion {
			t.Errorf("%s: OrchestratorInjectionsVersion = %q, want the HarnessInjectionsOrchestrator.md version %q",
				target, stamp.OrchestratorInjectionsVersion, stampOrchVersion)
		}
	}
	sub, ok := exec.stamps["test-runner.md"]
	if !ok {
		t.Fatal("no version stamp for the subagent test-runner.md")
	}
	if sub.InjectionsVersion != stampSharedVersion {
		t.Errorf("subagent InjectionsVersion = %q, want %q", sub.InjectionsVersion, stampSharedVersion)
	}
	if sub.OrchestratorInjectionsVersion != "" {
		t.Errorf("subagent OrchestratorInjectionsVersion = %q, want empty", sub.OrchestratorInjectionsVersion)
	}
}

func TestDeployBuild_ManifestStamps_EqualTheRegionTagsOfTheDeployedContent(t *testing.T) {
	_, _, deps, workspace := deployCaptureDeps(t, stampSourceBothMarkers, 1)
	exec := &stampCapturingExecutor{}
	deps.Executor = exec

	runDeployCapture(t, deps, workspace)

	for _, target := range []string{"orchestrator.md", "orchestrator-script.md"} {
		assertRegionStamps(t, target, exec.contents[target])
		stamp := exec.stamps[target]
		if stamp.InjectionsVersion != stampSharedVersion || stamp.OrchestratorInjectionsVersion != stampOrchVersion {
			t.Errorf("%s: stamps {%q, %q} differ from the region tags {%q, %q}", target,
				stamp.InjectionsVersion, stamp.OrchestratorInjectionsVersion, stampSharedVersion, stampOrchVersion)
		}
	}
}
