// Package opencode — OpenCodeAdapter implementation.
//
// OpenCodeAdapter maps Runner's domain vocabulary onto mosaic-common/harness's
// OpenCode spawner, mirroring ClaudeCodeAdapter's thin mapping-wrapper shape:
// it converts domain.AgentReference/ProtocolRequest into the shared package's
// neutral types, delegates the spawn, and converts the result back.
//
// Kill behaviour (verified on Windows with OpenCode 1.18.18): `opencode run`
// hosts the agent session inside the spawned process, so the whole-descendant
// kill used for timeout and force stop ends the session's work. Method: a
// `run --format json --auto` agent performing 60 sequential bash tool calls
// (file append plus a 3-second sleep) was killed with `taskkill /F /T` after
// 8 calls; the file stayed unchanged for the next 25 seconds. No session abort through the OpenCode server API is
// therefore needed, and the adapter makes no server calls. Re-verify if `run`
// ever defaults to attaching to a persistent server.
package opencode

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

// Sentinel errors specific to OpenCodeAdapter, aliased onto the shared
// module's so errors.Is works across the boundary. The taxonomy
// ClaudeCodeAdapter exposes (ErrExecutableNotFound, ErrNonZeroExit,
// ErrTimeout, ErrEmptyResponse, ErrMalformedJSON, cliexec.ErrMalformedOutput) is
// package-level and shared unchanged; these two are additional.
var (
	// ErrStreamError is returned when the harness's own event stream
	// reported the run failed. It has no Claude Code counterpart because
	// that harness reports a failed run through its exit code.
	ErrStreamError = commonharness.ErrOpenCodeStreamError

	// ErrStreamIncomplete is returned when the event stream ended without a
	// terminal event, leaving the run's outcome unknown.
	ErrStreamIncomplete = commonharness.ErrOpenCodeStreamIncomplete
)

// OpenCodeAdapter implements domain.HarnessAdapter by delegating to
// mosaic-common/harness for spawning, argument construction and
// event-stream parsing. See claudecode.go's ClaudeCodeAdapter for the
// mapping shape this implementation mirrors.
type OpenCodeAdapter struct {
	executablePath string
	timeout        time.Duration
	hitlTimeout    time.Duration
	activityObs    domain.ActivityObserver
	spawner        commonharness.Spawner
	sink           commonharness.Sink
	logger         domain.DebugLogger
	roleEnv        roleenv.Config
}

// RunID returns the run id the adapter was constructed with ("" when unknown).
func (a *OpenCodeAdapter) RunID() string { return a.roleEnv.RunID }

// ExecutablePath implements domain.ExecutableRevealer. It returns the
// executable path or command name this adapter was constructed with.
func (a *OpenCodeAdapter) ExecutablePath() string { return a.executablePath }

// NewOpenCodeAdapter creates an OpenCodeAdapter with debug logging disabled.
// Zero timeout means 30 minutes, matching the existing adapter.
func NewOpenCodeAdapter(executablePath string, timeout time.Duration) *OpenCodeAdapter {
	return NewOpenCodeAdapterWithLogger(executablePath, timeout, nil)
}

// NewOpenCodeAdapterWithLogger creates an adapter that records every
// invocation's raw stdout, raw stderr and outcome to the debug log. A nil
// logger is normalised to domain.NopDebugLogger.
func NewOpenCodeAdapterWithLogger(executablePath string, timeout time.Duration, logger domain.DebugLogger, opts ...roleenv.Option) *OpenCodeAdapter {
	if timeout == 0 {
		timeout = domain.DefaultTimeout
	}
	if logger == nil {
		logger = domain.NopDebugLogger{}
	}
	sink := cliexec.NewSink(logger)
	spawner := commonharness.NewOpenCode(executablePath,
		commonharness.WithTimeout(timeout),
		commonharness.WithSink(sink),
	)
	return &OpenCodeAdapter{executablePath: executablePath, timeout: timeout, spawner: spawner, sink: sink, logger: logger, roleEnv: roleenv.NewConfig(opts...)}
}

// Invoke implements domain.HarnessAdapter.
//
// Error returns use the sentinel error taxonomy shared with ClaudeCodeAdapter
// (ErrExecutableNotFound, ErrNonZeroExit, ErrTimeout), plus this adapter's
// own ErrStreamError and ErrStreamIncomplete for the stream-derived verdict
// `opencode run`'s exit code cannot report.
//
// On context cancellation, terminates the subprocess and returns ctx.Err().
func (a *OpenCodeAdapter) invoke(ctx context.Context, agent domain.AgentReference, request domain.ProtocolRequest, onLine func([]byte)) (domain.ProtocolResponse, error) {
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
	spawnReq := commonharness.SpawnRequest{
		Agent: commonharness.AgentRef{
			Identifier:     agent.Identifier,
			DefinitionPath: agent.DefinitionPath,
		},
		Prompt:       string(reqBytes),
		Timeout:      a.invokeTimeout(request.HumanInTheLoop),
		Env:          roleenv.SubagentEnv(runID, request.AgentInstanceID),
		OutputFormat: "json",
		OnStdoutLine: onLine,
		// SystemPrompt carries a synthesized <env> block for BOTH invocation
		// kinds, unlike ClaudeCodeAdapter (which only needs one for the
		// orchestrator kind): OpenCode's --agent replaces the default system
		// prompt regardless of which agent is selected, so the <env>
		// preamble is lost in both cases and must be supplied in both.
		SystemPrompt: commonharness.EnvBlock(""),
	}

	resp, err := a.spawner.Spawn(ctx, spawnReq)
	if err != nil {
		// ErrMalformedJSON/ErrProtocolNotExtractable mean the harness's
		// output could not be turned into a protocol response despite a
		// process-level success; ErrStreamError/ErrStreamIncomplete mean the
		// same thing for OpenCode's stream-derived verdict, which has no
		// Claude Code counterpart because that harness reports failure
		// through its exit code instead.
		switch {
		case errors.Is(err, commonharness.ErrMalformedJSON),
			errors.Is(err, commonharness.ErrProtocolNotExtractable),
			errors.Is(err, ErrStreamError),
			errors.Is(err, ErrStreamIncomplete):
			a.logger.Log(domain.EventHarnessParseFailed, string(resp.Stdout),
				domain.F("agent", request.AgentInstanceID),
			)
		}
		// A missing or non-executable binary is recoverable via the override
		// screen. Wrap it in a domain.HarnessLaunchError so the TUI can
		// distinguish it from every other failure class by error identity.
		if errors.Is(err, commonharness.ErrExecutableNotFound) {
			launchErr := &domain.HarnessLaunchError{
				Harness:    commonharness.HarnessIDOpenCode,
				Executable: a.executablePath,
				Err:        err,
			}
			a.logger.Log(domain.EventHarnessInvokeError, launchErr.Error(),
				domain.F("agent", request.AgentInstanceID),
			)
			return domain.ProtocolResponse{}, launchErr
		}
		// The shared package distinguishes parent-context cancellation from
		// adapter timeout via ErrCancelled. Runner's existing call sites and
		// tests expect ctx.Err() on cancellation, so translate it back here.
		if ctx.Err() != nil {
			a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
				domain.F("agent", request.AgentInstanceID),
			)
			return domain.ProtocolResponse{}, ctx.Err()
		}
		a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
			domain.F("agent", request.AgentInstanceID),
		)
		return domain.ProtocolResponse{}, err
	}

	protoResp, uErr := harness.UnmarshalResponse(resp.Protocol)
	if uErr != nil {
		wrapped := fmt.Errorf("%w: response decode failed", cliexec.ErrMalformedOutput)
		a.logger.Log(domain.EventHarnessParseFailed, string(resp.Protocol),
			domain.F("agent", request.AgentInstanceID),
		)
		a.logger.Log(domain.EventHarnessInvokeError, wrapped.Error(),
			domain.F("agent", request.AgentInstanceID),
		)
		return domain.ProtocolResponse{}, wrapped
	}

	a.logger.Log(domain.EventHarnessInvokeOK, "invocation succeeded",
		domain.F("agent", request.AgentInstanceID),
		domain.F("status", string(protoResp.StatusCode)),
	)
	return protoResp, nil
}

// InvokeRaw implements domain.RawInvoker.
//
// It performs the same steps as Invoke up through envelope parsing but stops
// before ExtractProtocolJSON, so non-protocol replies (consultation responses,
// routing instructions) are returned as plain text without being rejected.
//
// On context cancellation, returns ctx.Err().
func (a *OpenCodeAdapter) invokeRaw(ctx context.Context, agent domain.AgentReference, payload []byte, onLine func([]byte)) ([]byte, error) {
	a.logger.Log(domain.EventHarnessInvokeStart, "dispatching raw invocation",
		domain.F("agent", agent.Identifier),
	)

	spawnReq := commonharness.SpawnRequest{
		Agent: commonharness.AgentRef{
			Identifier:     agent.Identifier,
			DefinitionPath: agent.DefinitionPath,
		},
		Prompt:       string(payload),
		Env:          roleenv.OrchestratorEnv(a.roleEnv.RunID),
		OutputFormat: "json",
		SystemPrompt: commonharness.EnvBlock(""),
	}

	cmd, err := commonharness.ResolveExecutable(a.executablePath)
	if err != nil {
		a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
			domain.F("agent", agent.Identifier),
		)
		return nil, err
	}

	args, stdin, err := commonharness.BuildOpenCodeArgs(spawnReq)
	if err != nil {
		a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
			domain.F("agent", agent.Identifier),
		)
		return nil, err
	}

	resp, err := commonharness.Run(ctx, cmd, args, commonharness.RunOptions{
		WorkingDir:   spawnReq.WorkingDir,
		Env:          spawnReq.Env,
		Stdin:        stdin,
		Timeout:      a.timeout,
		Sink:         a.sink,
		OnStdoutLine: onLine,
	})
	if err != nil {
		// A missing or non-executable binary is recoverable via the override
		// screen. Wrap it in a domain.HarnessLaunchError so the TUI can
		// distinguish it from every other failure class by error identity.
		if errors.Is(err, commonharness.ErrExecutableNotFound) {
			launchErr := &domain.HarnessLaunchError{
				Harness:    commonharness.HarnessIDOpenCode,
				Executable: a.executablePath,
				Err:        err,
			}
			a.logger.Log(domain.EventHarnessInvokeError, launchErr.Error(),
				domain.F("agent", agent.Identifier),
			)
			return nil, launchErr
		}
		if ctx.Err() != nil {
			a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
				domain.F("agent", agent.Identifier),
			)
			return nil, ctx.Err()
		}
		a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
			domain.F("agent", agent.Identifier),
		)
		return nil, err
	}

	text, err := commonharness.ParseOpenCodeEnvelope(resp.Stdout)
	if err != nil {
		a.logger.Log(domain.EventHarnessInvokeError, err.Error(),
			domain.F("agent", agent.Identifier),
		)
		return nil, err
	}

	a.logger.Log(domain.EventHarnessStdout, text,
		domain.F("agent", agent.Identifier),
	)
	a.logger.Log(domain.EventHarnessInvokeOK, "raw invocation succeeded",
		domain.F("agent", agent.Identifier),
	)
	return []byte(text), nil
}

// WithHITLTimeout sets the timeout for Invoke calls whose request has
// HumanInTheLoop == true. d <= 0 selects domain.DefaultHITLTimeout.
// Returns a. Call before the first invocation; not safe concurrently with
// invocations.
func (a *OpenCodeAdapter) WithHITLTimeout(d time.Duration) *OpenCodeAdapter {
	if d <= 0 {
		d = domain.DefaultHITLTimeout
	}
	a.hitlTimeout = d
	return a
}

// invokeTimeout returns the timeout bounding an Invoke call for the given
// effective HITL flag.
func (a *OpenCodeAdapter) invokeTimeout(hitl bool) time.Duration {
	if !hitl {
		return a.timeout
	}
	if a.hitlTimeout <= 0 {
		return domain.DefaultHITLTimeout
	}
	return a.hitlTimeout
}
