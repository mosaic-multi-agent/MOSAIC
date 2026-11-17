package opencodev2

import (
	"context"
	"fmt"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/domain"
)

// DefaultSpawnTimeout is this adapter's own backstop on one invocation; the
// test's declared timeout is enforced by the supervisor cancelling the launch.
const DefaultSpawnTimeout = 30 * time.Minute

// SpawnPlan implements domain.HarnessAdapter. It describes one standalone
// `run` of the subject agent, selected by id with no rewrite of agent files.
// Arguments and stdin come from the shared V2 builder.
//
// The environment relocates user configuration into the run's control
// directory and pins PWD to the sandbox subject directory: V2 derives its
// project from PWD, and the launcher appends Env to the inherited environment,
// so a stale inherited PWD would otherwise select the wrong project.
func (a *Adapter) SpawnPlan(_ context.Context, subject domain.SubjectUnderTest, p domain.Provisioning) (domain.SpawnPlan, error) {
	if subject.Model == "" {
		return domain.SpawnPlan{}, fmt.Errorf("opencode-v2: %w", domain.ErrNoSubjectModel)
	}
	args, stdin, err := commonharness.BuildOpenCodeV2Args(commonharness.SpawnRequest{
		Agent:        commonharness.AgentRef{Identifier: subject.Identity},
		Prompt:       subject.OpeningMessage,
		SystemPrompt: commonharness.EnvBlock(p.Sandbox.SubjectDir),
		Model:        subject.Model,
		OutputFormat: "json",
	})
	if err != nil {
		return domain.SpawnPlan{}, fmt.Errorf("opencode-v2: building spawn arguments: %w", err)
	}

	env := []string{fmt.Sprintf("%s=%s", ConfigHomeEnvVar, relocatedConfigHome(p.Sandbox))}
	return domain.SpawnPlan{
		Executable:        CLIExecutable,
		Args:              args,
		Stdin:             stdin,
		WorkingDir:        p.Sandbox.SubjectDir,
		Timeout:           DefaultSpawnTimeout,
		EarlyExitSentinel: p.Sandbox.EarlyExitSentinelPath(),
		Env:               commonharness.OpenCodeV2Env(p.Sandbox.SubjectDir, env),
	}, nil
}
