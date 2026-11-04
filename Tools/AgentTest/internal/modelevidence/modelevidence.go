// Package modelevidence implements domain.ObservedModelProvider by delegating
// to the log-analysis tool's per-run report command. No log parsing exists in
// this module: the subject session is the report's orchestrator actor (the
// harness main session); stubs run as subagents and appear as agent actors,
// which are never read.
package modelevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"time"

	"mosaic-agent-test/internal/domain"
)

// CommandRunner is the seam that makes the outcome mapping testable without
// the real log-analysis tool present.
type CommandRunner func(ctx context.Context, path string, args []string, workingDir string) (stdout []byte, exitCode int, err error)

// Options configures the delegating provider.
type Options struct {
	ExecutablePath string
	Timeout        time.Duration
	WorkingDir     string
	Invoke         CommandRunner // nil selects real process execution
}

// New constructs the delegating provider.
func New(opts Options) domain.ObservedModelProvider {
	return &provider{opts: opts}
}

type provider struct{ opts Options }

var _ domain.ObservedModelProvider = (*provider)(nil)

func (p *provider) ObservedSubjectModels(ctx context.Context, q domain.ObservedModelQuery) (domain.ObservedModelEvidence, error) {
	base := MapInput{ExecutablePath: p.opts.ExecutablePath, RunID: q.RunID, Form: q.Form}
	if !validForm(q.Form) {
		return Map(base), nil
	}

	invoke := p.opts.Invoke
	if invoke == nil {
		invoke = execCommandRunner
	}
	runCtx := ctx
	if p.opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, p.opts.Timeout)
		defer cancel()
	}

	scanPath := q.LogRoot
	if q.LogsRoot != "" {
		scanPath = q.LogsRoot
	}
	args := []string{"report", "--run", q.RunID, "--path", scanPath, "--format", "json"}
	stdout, exitCode, invokeErr := invoke(runCtx, p.opts.ExecutablePath, args, p.opts.WorkingDir)

	base.ExitCode = exitCode
	base.InvokeErr = invokeErr
	if invokeErr == nil && exitCode == 0 {
		if parseErr := json.Unmarshal(stdout, &base.Report); parseErr != nil {
			base.ParseErr = parseErr
		}
	}
	return Map(base), nil
}

// execCommandRunner is the real process-execution CommandRunner.
func execCommandRunner(ctx context.Context, path string, args []string, workingDir string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = workingDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard

	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return stdout.Bytes(), exitErr.ExitCode(), nil
	}
	return nil, 0, err
}
