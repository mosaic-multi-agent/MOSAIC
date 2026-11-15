// Package codex holds the Runner's harness adapter for the Codex CLI.
//
// CodexAdapter maps Runner's domain vocabulary onto mosaic-common/harness's
// Codex spawner, mirroring the OpenCode adapter's shape. The agent is loaded
// read-only from its definition file on every invocation and its model and
// declared settings travel as command-line overrides; the adapter never
// modifies the agent file. Codex reports neither model, context size nor cost,
// so none of them is ever shown.
package codex

import (
	"context"
	"errors"
	"fmt"
	"time"

	"mosaic-common/agentformat/formatid"
	"mosaic-common/agentload"
	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
	"mosaic-run/internal/harness/cliexec"
	"mosaic-run/internal/harness/roleenv"
)

// harnessID identifies Codex in launch errors.
const harnessID = "codex"

// Sentinel errors aliased onto the shared module's so errors.Is works across
// the boundary.
var (
	// ErrStreamError is returned when the event stream reported the run failed.
	ErrStreamError = commonharness.ErrCodexStreamError
	// ErrStreamIncomplete is returned when the event stream ended without a
	// terminal event, leaving the run's outcome unknown.
	ErrStreamIncomplete = commonharness.ErrCodexStreamIncomplete
	// ErrAgentUnreadable is returned when the agent file cannot be loaded.
	ErrAgentUnreadable = commonharness.ErrCodexAgentUnreadable
)

// CodexAdapter implements domain.HarnessAdapter and domain.RawInvoker for the
// Codex CLI.
type CodexAdapter struct {
	executablePath string
	timeout        time.Duration
	hitlTimeout    time.Duration
	activityObs    domain.ActivityObserver
	spawner        commonharness.Spawner
	sink           commonharness.Sink
	logger         domain.DebugLogger
	roleEnv        roleenv.Config
}

// NewCodexAdapter creates an adapter with debug logging disabled. Zero timeout
// means domain.DefaultTimeout.
func NewCodexAdapter(executablePath string, timeout time.Duration) *CodexAdapter {
	return NewCodexAdapterWithLogger(executablePath, timeout, nil)
}

// NewCodexAdapterWithLogger creates an adapter that records every invocation's
// raw stdout, raw stderr and outcome to the debug log. A nil logger is
// normalised to domain.NopDebugLogger.
func NewCodexAdapterWithLogger(executablePath string, timeout time.Duration, logger domain.DebugLogger, opts ...roleenv.Option) *CodexAdapter {
	if timeout == 0 {
		timeout = domain.DefaultTimeout
	}
	if logger == nil {
		logger = domain.NopDebugLogger{}
	}
	sink := cliexec.NewSink(logger)
	spawner := commonharness.NewCodex(executablePath,
		commonharness.WithTimeout(timeout),
		commonharness.WithSink(sink),
	)
	return &CodexAdapter{executablePath: executablePath, timeout: timeout, spawner: spawner, sink: sink, logger: logger, roleEnv: roleenv.NewConfig(opts...)}
}

// WithHITLTimeout sets the timeout for Invoke calls whose request has
// HumanInTheLoop == true. d <= 0 selects domain.DefaultHITLTimeout. Returns a.
// Call before the first invocation.
func (a *CodexAdapter) WithHITLTimeout(d time.Duration) *CodexAdapter {
	if d <= 0 {
		d = domain.DefaultHITLTimeout
	}
	a.hitlTimeout = d
	return a
}

// RunID returns the run id the adapter was constructed with ("" when unknown).
func (a *CodexAdapter) RunID() string { return a.roleEnv.RunID }

// ExecutablePath implements domain.ExecutableRevealer.
func (a *CodexAdapter) ExecutablePath() string { return a.executablePath }

// invokeTimeout returns the timeout bounding an Invoke call for the given HITL
// flag.
func (a *CodexAdapter) invokeTimeout(hitl bool) time.Duration {
	if !hitl {
		return a.timeout
	}
	if a.hitlTimeout <= 0 {
		return domain.DefaultHITLTimeout
	}
	return a.hitlTimeout
}

// logError records an invocation failure.
func (a *CodexAdapter) logError(agent string, err error) {
	a.logger.Log(domain.EventHarnessInvokeError, err.Error(), domain.F("agent", agent))
}

// failure maps an error from the shared package onto what the adapter returns:
// a missing executable becomes a launch error, cancellation the context error.
func (a *CodexAdapter) failure(ctx context.Context, agent string, err error) error {
	if errors.Is(err, commonharness.ErrExecutableNotFound) {
		err = &domain.HarnessLaunchError{Harness: harnessID, Executable: a.executablePath, Err: err}
	} else if ctx.Err() != nil {
		err = ctx.Err()
	}
	a.logError(agent, err)
	return err
}

// invoke runs one subagent invocation. On context cancellation it returns
// ctx.Err().
func (a *CodexAdapter) invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest, onLine func([]byte)) (domain.ProtocolResponse, error) {
	a.logger.Log(domain.EventHarnessInvokeStart, "dispatching invocation",
		domain.F("agent", request.AgentInstanceID),
	)

	reqBytes, err := harness.MarshalRequest(request)
	if err != nil {
		return domain.ProtocolResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	runID := request.RunID
	if runID == "" {
		runID = a.roleEnv.RunID
	}
	resp, err := a.spawner.Spawn(ctx, commonharness.SpawnRequest{
		Agent:        commonharness.AgentRef{Identifier: agent.Identifier, DefinitionPath: agent.DefinitionPath},
		Prompt:       string(reqBytes),
		Timeout:      a.invokeTimeout(request.HumanInTheLoop),
		Env:          roleenv.SubagentEnv(runID, request.AgentInstanceID),
		OutputFormat: "json",
		OnStdoutLine: onLine,
	})
	if err != nil {
		if unparseableStream(err) {
			a.logger.Log(domain.EventHarnessParseFailed, string(resp.Stdout),
				domain.F("agent", request.AgentInstanceID),
			)
		}
		return domain.ProtocolResponse{}, a.failure(ctx, request.AgentInstanceID, err)
	}

	protoResp, uErr := harness.UnmarshalResponse(resp.Protocol)
	if uErr != nil {
		wrapped := fmt.Errorf("%w: response decode failed", cliexec.ErrMalformedOutput)
		a.logger.Log(domain.EventHarnessParseFailed, string(resp.Protocol),
			domain.F("agent", request.AgentInstanceID),
		)
		a.logError(request.AgentInstanceID, wrapped)
		return domain.ProtocolResponse{}, wrapped
	}

	a.logger.Log(domain.EventHarnessInvokeOK, "invocation succeeded",
		domain.F("agent", request.AgentInstanceID),
		domain.F("status", string(protoResp.StatusCode)),
	)
	return protoResp, nil
}

// unparseableStream reports whether err means the harness's output could not be
// turned into a response despite the process running.
func unparseableStream(err error) bool {
	return errors.Is(err, commonharness.ErrMalformedJSON) ||
		errors.Is(err, commonharness.ErrProtocolNotExtractable) ||
		errors.Is(err, ErrStreamError) ||
		errors.Is(err, ErrStreamIncomplete)
}

// invokeRaw runs one consultation and returns the final agent message
// verbatim, without requiring protocol JSON. On context cancellation it
// returns ctx.Err().
func (a *CodexAdapter) invokeRaw(ctx context.Context, agent domain.AgentReference, payload []byte, onLine func([]byte)) ([]byte, error) {
	a.logger.Log(domain.EventHarnessInvokeStart, "dispatching raw invocation",
		domain.F("agent", agent.Identifier),
	)

	loaded, err := agentload.Load(agent.DefinitionPath, string(formatid.CodexTOML))
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrAgentUnreadable, err)
		a.logError(agent.Identifier, err)
		return nil, err
	}

	cmd, err := commonharness.ResolveExecutable(a.executablePath)
	if err != nil {
		return nil, a.failure(ctx, agent.Identifier, err)
	}

	spawnReq := commonharness.SpawnRequest{
		Agent:        commonharness.AgentRef{Identifier: agent.Identifier, DefinitionPath: agent.DefinitionPath},
		Prompt:       string(payload),
		Env:          roleenv.OrchestratorEnv(a.roleEnv.RunID),
		OutputFormat: "json",
	}
	args, stdin, err := commonharness.BuildCodexArgs(spawnReq, loaded)
	if err != nil {
		a.logError(agent.Identifier, err)
		return nil, err
	}

	resp, runErr := commonharness.Run(ctx, cmd, args, commonharness.RunOptions{
		WorkingDir:   spawnReq.WorkingDir,
		Env:          spawnReq.Env,
		Stdin:        stdin,
		Timeout:      a.timeout,
		Sink:         a.sink,
		OnStdoutLine: onLine,
	})
	text, parseErr := commonharness.ParseCodexEnvelope(resp.Stdout)
	if runErr != nil {
		// A non-zero exit that carries a stream-reported failure keeps both.
		if errors.Is(runErr, commonharness.ErrNonZeroExit) && errors.Is(parseErr, ErrStreamError) {
			runErr = fmt.Errorf("%w: %w", runErr, parseErr)
		}
		return nil, a.failure(ctx, agent.Identifier, runErr)
	}
	if parseErr != nil {
		a.logError(agent.Identifier, parseErr)
		return nil, parseErr
	}

	a.logger.Log(domain.EventHarnessStdout, text, domain.F("agent", agent.Identifier))
	a.logger.Log(domain.EventHarnessInvokeOK, "raw invocation succeeded",
		domain.F("agent", agent.Identifier),
	)
	return []byte(text), nil
}
