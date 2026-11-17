// Package opencodev2 is the Runner's OpenCode V2 adapter. It maps the Runner's
// domain vocabulary onto mosaic-common/harness's V2 spawn and stream layer,
// mirroring the V1 OpenCode adapter's shape.
//
// Every dispatch is one standalone `run` of the agent by id, in the workspace
// root, with the agent file used in place. No model is passed: the agent
// file's own `model:` applies. Timeout and force stop end the whole process
// tree through the shared Run, which ends the standalone session with it.
package opencodev2

import (
	"context"
	"errors"
	"fmt"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/roleenv"
)

// harnessID is the catalog id of this harness.
const harnessID = commonharness.HarnessIDOpenCodeV2

// OpenCodeV2Adapter implements domain.HarnessAdapter, domain.RawInvoker and
// domain.ExecutableRevealer for OpenCode V2.
type OpenCodeV2Adapter struct {
	executablePath string
	timeout        time.Duration
	hitlTimeout    time.Duration
	// noOutputTimeout is the no-output watchdog duration; 0 disables it. It
	// never applies to HITL dispatches (an ask-the-user call is silent).
	noOutputTimeout time.Duration
	workspaceRoot   string
	activityObs     domain.ActivityObserver
	spawner         commonharness.Spawner
	sink            commonharness.Sink
	logger          domain.DebugLogger
	roleEnv         roleenv.Config
}

// NewOpenCodeV2Adapter creates an adapter with debug logging disabled. Zero
// timeout means domain.DefaultTimeout.
func NewOpenCodeV2Adapter(executablePath string, timeout time.Duration) *OpenCodeV2Adapter {
	return NewOpenCodeV2AdapterWithLogger(executablePath, timeout, nil)
}

// NewOpenCodeV2AdapterWithLogger creates an adapter that records every
// invocation to the debug log. A nil logger is normalised to
// domain.NopDebugLogger.
func NewOpenCodeV2AdapterWithLogger(executablePath string, timeout time.Duration, logger domain.DebugLogger, opts ...roleenv.Option) *OpenCodeV2Adapter {
	if timeout == 0 {
		timeout = domain.DefaultTimeout
	}
	if logger == nil {
		logger = domain.NopDebugLogger{}
	}
	sink := outcomeSink{next: cliexec.NewSink(logger)}
	return &OpenCodeV2Adapter{
		executablePath: executablePath,
		timeout:        timeout,
		spawner: commonharness.NewOpenCodeV2(executablePath,
			commonharness.WithTimeout(timeout),
			commonharness.WithSink(sink),
		),
		sink:    sink,
		logger:  logger,
		roleEnv: roleenv.NewConfig(opts...),
	}
}

// WithHITLTimeout sets the timeout for Invoke calls whose request has
// HumanInTheLoop == true. d <= 0 selects domain.DefaultHITLTimeout. Returns a.
func (a *OpenCodeV2Adapter) WithHITLTimeout(d time.Duration) *OpenCodeV2Adapter {
	if d <= 0 {
		d = domain.DefaultHITLTimeout
	}
	a.hitlTimeout = d
	return a
}

// WithNoOutputTimeout sets the no-output watchdog duration for non-HITL
// invocations. d > 0 enables it; d <= 0 disables it (the default). Returns a.
func (a *OpenCodeV2Adapter) WithNoOutputTimeout(d time.Duration) *OpenCodeV2Adapter {
	if d < 0 {
		d = 0
	}
	a.noOutputTimeout = d
	return a
}

// WithWorkspaceRoot sets the child's working directory. "" means the Runner's
// current directory. Returns a.
func (a *OpenCodeV2Adapter) WithWorkspaceRoot(dir string) *OpenCodeV2Adapter {
	a.workspaceRoot = dir
	return a
}

// ExecutablePath implements domain.ExecutableRevealer. It returns the path or
// command name the adapter was constructed with.
func (a *OpenCodeV2Adapter) ExecutablePath() string { return a.executablePath }

// RunID returns the run id the adapter was constructed with ("" when unknown).
func (a *OpenCodeV2Adapter) RunID() string { return a.roleEnv.RunID }

func (a *OpenCodeV2Adapter) invokeTimeout(hitl bool) time.Duration {
	if !hitl {
		return a.timeout
	}
	if a.hitlTimeout <= 0 {
		return domain.DefaultHITLTimeout
	}
	return a.hitlTimeout
}

// invokeNoOutput returns the watchdog duration for a dispatch: 0 (disabled)
// for human-in-the-loop dispatches.
func (a *OpenCodeV2Adapter) invokeNoOutput(hitl bool) time.Duration {
	if hitl {
		return 0
	}
	return a.noOutputTimeout
}

func (a *OpenCodeV2Adapter) spawnRequest(agent domain.AgentReference, prompt string, env []string, timeout, noOutput time.Duration, onLine func([]byte)) commonharness.SpawnRequest {
	return commonharness.SpawnRequest{
		Agent:           commonharness.AgentRef{Identifier: agent.Identifier, DefinitionPath: agent.DefinitionPath},
		Prompt:          prompt,
		WorkingDir:      a.workspaceRoot,
		Timeout:         timeout,
		NoOutputTimeout: noOutput,
		Env:             env,
		OutputFormat:    "json",
		OnStdoutLine:    onLine,
		// V2's --agent replaces the default system prompt, so the <env>
		// preamble must be supplied for both dispatch kinds.
		SystemPrompt: commonharness.EnvBlock(a.workspaceRoot),
	}
}

// invoke dispatches one subagent request and decodes its protocol response.
// On context cancellation it returns ctx.Err().
func (a *OpenCodeV2Adapter) invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest, onLine func([]byte)) (domain.ProtocolResponse, error) {
	a.logger.Log(domain.EventHarnessInvokeStart, "dispatching invocation", domain.F("agent", request.AgentInstanceID))

	reqBytes, err := harness.MarshalRequest(request)
	if err != nil {
		return domain.ProtocolResponse{}, fmt.Errorf("marshal request: %w", err)
	}
	runID := request.RunID
	if runID == "" {
		runID = a.roleEnv.RunID
	}
	spawnReq := a.spawnRequest(agent, string(reqBytes), roleenv.SubagentEnv(runID, request.AgentInstanceID), a.invokeTimeout(request.HumanInTheLoop), a.invokeNoOutput(request.HumanInTheLoop), onLine)

	resp, err := a.spawner.Spawn(ctx, spawnReq)
	if err != nil {
		if isUnusableReply(err) {
			a.logger.Log(domain.EventHarnessParseFailed, string(resp.Stdout), domain.F("agent", request.AgentInstanceID))
		}
		return domain.ProtocolResponse{}, a.failure(ctx, err, request.AgentInstanceID)
	}

	protoResp, uErr := harness.UnmarshalResponse(resp.Protocol)
	if uErr != nil {
		wrapped := fmt.Errorf("%w: response decode failed", cliexec.ErrMalformedOutput)
		a.logger.Log(domain.EventHarnessParseFailed, string(resp.Protocol), domain.F("agent", request.AgentInstanceID))
		a.logger.Log(domain.EventHarnessInvokeError, wrapped.Error(), domain.F("agent", request.AgentInstanceID))
		return domain.ProtocolResponse{}, wrapped
	}

	a.logger.Log(domain.EventHarnessInvokeOK, "invocation succeeded",
		domain.F("agent", request.AgentInstanceID),
		domain.F("status", string(protoResp.StatusCode)),
	)
	return protoResp, nil
}

// invokeRaw dispatches an orchestrator consultation and returns the final text
// as is, without requiring protocol JSON in it. The HITL timeout never applies.
func (a *OpenCodeV2Adapter) invokeRaw(ctx context.Context, agent domain.AgentReference, payload []byte, onLine func([]byte)) ([]byte, error) {
	a.logger.Log(domain.EventHarnessInvokeStart, "dispatching raw invocation", domain.F("agent", agent.Identifier))

	spawnReq := a.spawnRequest(agent, string(payload), roleenv.OrchestratorEnv(a.roleEnv.RunID), a.timeout, a.noOutputTimeout, onLine)
	cmd, err := commonharness.ResolveExecutable(a.executablePath)
	if err != nil {
		return nil, a.failure(ctx, err, agent.Identifier)
	}
	args, stdin, err := commonharness.BuildOpenCodeV2Args(spawnReq)
	if err != nil {
		return nil, a.failure(ctx, err, agent.Identifier)
	}

	resp, runErr := commonharness.Run(ctx, cmd, args, commonharness.RunOptions{
		WorkingDir:   spawnReq.WorkingDir,
		Env:          commonharness.OpenCodeV2Env(spawnReq.WorkingDir, spawnReq.Env),
		Stdin:        stdin,
		Timeout:      spawnReq.Timeout,
		Sink:         a.sink,
		OnStdoutLine: onLine,

		NoOutputTimeout: spawnReq.NoOutputTimeout,
	})
	text, err := commonharness.OpenCodeV2Verdict(resp, runErr)
	if err != nil {
		return nil, a.failure(ctx, err, agent.Identifier)
	}

	a.logger.Log(domain.EventHarnessStdout, text, domain.F("agent", agent.Identifier))
	a.logger.Log(domain.EventHarnessInvokeOK, "raw invocation succeeded", domain.F("agent", agent.Identifier))
	return []byte(text), nil
}

// isUnusableReply reports whether the process ended without a usable reply
// (no protocol JSON, empty, or no final answer) rather than with a failure
// that V2 itself reported.
func isUnusableReply(err error) bool {
	return errors.Is(err, commonharness.ErrMalformedJSON) ||
		errors.Is(err, commonharness.ErrProtocolNotExtractable) ||
		errors.Is(err, commonharness.ErrEmptyResponse) ||
		errors.Is(err, commonharness.ErrOpenCodeV2StreamIncomplete)
}

// outcomeSink forwards the shared layer's diagnostics except its spawn
// failure events: the adapter reports each failure once, with the verdict's
// own wording.
type outcomeSink struct{ next commonharness.Sink }

func (s outcomeSink) Log(ev commonharness.Event) {
	if ev.Name == "spawn.error" || ev.Name == "spawn.timeout" || ev.Name == "spawn.no_output" {
		return
	}
	s.next.Log(ev)
}

// failure logs err and maps it for the caller: a missing executable becomes a
// domain.HarnessLaunchError, parent-context cancellation becomes ctx.Err().
func (a *OpenCodeV2Adapter) failure(ctx context.Context, err error, agent string) error {
	result := err
	switch {
	case errors.Is(err, commonharness.ErrExecutableNotFound):
		result = &domain.HarnessLaunchError{Harness: harnessID, Executable: a.executablePath, Err: err}
	case ctx.Err() != nil:
		result = ctx.Err()
	}
	a.logger.Log(domain.EventHarnessInvokeError, result.Error(), domain.F("agent", agent))
	return result
}
