package opencodev2

import (
	"errors"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/domain"
)

// DecodeEnvelope maps a finished V2 invocation onto the subject's result. It
// is a package-level function because the launcher consumes it as a value.
//
// An exit code alone decides nothing for V2: the stream is always judged by
// the shared verdict. A failure keeps its cause as the first line of Stderr
// (followed by the process's own stderr), which is where the neutral readers
// look. DispositionEarlyExit is never produced here; the supervisor observes
// it. The function never returns a non-nil error: every failure degrades to
// a disposition.
func DecodeEnvelope(res commonharness.Response, runErr error) (domain.SubjectResult, error) {
	if errors.Is(runErr, commonharness.ErrTimeout) {
		return domain.SubjectResult{Disposition: domain.DispositionTimedOut, Duration: res.Duration}, nil
	}

	text, err := commonharness.OpenCodeV2Verdict(res, runErr)
	if err != nil {
		stderr := err.Error()
		if res.Stderr != "" {
			stderr += "\n" + res.Stderr
		}
		return domain.SubjectResult{
			Disposition: domain.DispositionSpawnFailed,
			RawOutput:   string(res.Stdout),
			ExitCode:    res.ExitCode,
			Stderr:      stderr,
			Duration:    res.Duration,
		}, nil
	}

	protocolMessage := text
	if protocol, perr := commonharness.ExtractProtocolJSON(text); perr == nil {
		protocolMessage = string(protocol)
	}
	return domain.SubjectResult{
		Disposition:     domain.DispositionCompleted,
		ProtocolMessage: protocolMessage,
		RawOutput:       string(res.Stdout),
		ExitCode:        res.ExitCode,
		Stderr:          res.Stderr,
		Duration:        res.Duration,
	}, nil
}
