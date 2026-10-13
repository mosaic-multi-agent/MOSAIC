// Envelope handling for the GHCP CLI lives in its own file, sitting beside
// envelope.go and envelope_opencode.go rather than being forced through either:
// the `copilot -p ... --output-format json` output is a newline-delimited JSON
// event stream, structurally unlike both the Claude Code single-document envelope
// and the OpenCode event stream, and shares nothing with either beyond feeding
// the same ExtractProtocolJSON. The schema is undocumented upstream; what follows
// was captured empirically from copilot CLI v1.0.77 on Windows, and that version
// is noted here so a future schema divergence is diagnosable rather than mysterious.
//
// Two further events are acted on, observed with v1.0.91 and v1.0.92:
//
//   - subagent.deselected: GHCP silently ran its default agent instead of the
//     requested --agent (known issue GC-022, fixed upstream in 1.0.92, but
//     Copilot self-updates, so older versions reappear). The reply can still be
//     a well-formed protocol response from the wrong agent, so the event fails
//     the run whatever the reply and the result event say.
//   - session.error (data.errorType, data.errorCode, data.statusCode,
//     data.message): a failed model call such as an exhausted quota is reported
//     only here, with nothing on stderr, and the result event may still carry
//     exitCode 0. The event's detail is carried into the error text.
//
// Both events are honoured wherever they sit relative to the result event, so
// the parser scans the whole stream; the returned text and the result verdict
// still come from the first result line only.
package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Sentinel errors specific to the GHCP CLI event-stream verdict. See harness.go
// for the shared spawn/parse taxonomy these sit alongside.
var (
	// ErrGHCPCLIStreamError reports that the event stream's terminal result
	// event declared the run failed (non-zero exitCode on the result line).
	// It exists separately from the shared taxonomy because the process exit
	// code is not a trustworthy verdict for this CLI — copilot has been
	// observed to exit 0 after producing no output — so the stream's own
	// result line is the authoritative verdict, and a non-zero value there
	// has no shared sentinel that can express it.
	ErrGHCPCLIStreamError = errors.New("harness: ghcp-cli run reported an error")

	// ErrGHCPCLIStreamIncomplete reports an event stream that ended without a
	// terminal result event, leaving the run's outcome unknown. It exists
	// separately from the shared taxonomy for the same reason: a process exit
	// code of zero is not sufficient for success, so a stream that ends with
	// no result line can never be reported as success, and that condition has
	// no other way to be expressed.
	ErrGHCPCLIStreamIncomplete = errors.New("harness: ghcp-cli event stream ended without a terminal event")

	// ErrGHCPCLIAgentDeselected: the stream contained subagent.deselected -
	// GHCP did not apply the requested --agent and ran its default agent.
	ErrGHCPCLIAgentDeselected = errors.New("harness: ghcp-cli did not apply the requested agent")

	// ErrGHCPCLISessionError: the stream contained session.error (for example
	// an exhausted quota); the reason appears only there, not on stderr.
	ErrGHCPCLISessionError = errors.New("harness: ghcp-cli session reported an error")
)

// Recognised type discriminators in the GHCP CLI JSONL event stream.
// Only ghcpEventAssistantMessage, ghcpEventResult, ghcpEventDeselected and
// ghcpEventSessionError change the parser's state.
// ghcpEventUserMessage and ghcpEventAssistantMessageDelta are named explicitly
// because they are the two events a future reader is most likely to add
// accumulation for by mistake; naming them makes their deliberate exclusion
// from accumulation explicit. Every other observed type falls through as
// unrecognised, contributing nothing and never producing an error.
const (
	ghcpEventUserMessage           = "user.message"
	ghcpEventAssistantMessage      = "assistant.message"
	ghcpEventAssistantMessageDelta = "assistant.message_delta"
	ghcpEventResult                = "result"
	ghcpEventDeselected            = "subagent.deselected"
	ghcpEventSessionError          = "session.error"
)

// ghcpCLIEventLine decodes one stdout line to the fields the parser acts on.
// Note that ExitCode is top-level, not nested under Data: the terminal result
// event has no data wrapper. Data is kept raw because its shape varies by event
// type (and can be a non-object); the members the parser reads are extracted
// tolerantly by ghcpDataString / ghcpDataInt so a badly typed member never
// discards the line. Every other observed field — timestamps, ids, parent ids,
// session ids, model names, token counts, request ids, usage totals, skill
// lists, MCP server status — is deliberately left undecoded: a field the
// parser does not read is a field that cannot break it, and this schema is
// undocumented and actively changing.
type ghcpCLIEventLine struct {
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data"`
	ExitCode int             `json:"exitCode"`
}

// ghcpDataMember returns the raw member key of an event's data object, or nil
// when data is absent, not an object, or lacks the member.
func ghcpDataMember(data json.RawMessage, key string) json.RawMessage {
	if len(data) == 0 || data[0] != '{' {
		return nil
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return nil
	}
	return members[key]
}

// ghcpDataString returns data.<key> when it is a JSON string, otherwise "".
func ghcpDataString(data json.RawMessage, key string) string {
	var s string
	if err := json.Unmarshal(ghcpDataMember(data, key), &s); err != nil {
		return ""
	}
	return s
}

// ghcpDataInt returns data.<key> when it is a JSON integer number; ok is false
// otherwise. A number carried as a string is deliberately not accepted.
func ghcpDataInt(data json.RawMessage, key string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(string(ghcpDataMember(data, key))))
	return n, err == nil
}

// ghcpSessionErrorDetail renders the readable parts of a session.error event
// as "type T, code C, status S: message", omitting every part that is absent
// or badly typed, or "no details" when nothing is left.
func ghcpSessionErrorDetail(data json.RawMessage) string {
	var parts []string
	if v := ghcpDataString(data, "errorType"); v != "" {
		parts = append(parts, "type "+v)
	}
	if v := ghcpDataString(data, "errorCode"); v != "" {
		parts = append(parts, "code "+v)
	}
	if n, ok := ghcpDataInt(data, "statusCode"); ok {
		parts = append(parts, "status "+strconv.Itoa(n))
	}
	detail := strings.Join(parts, ", ")
	if msg := ghcpDataString(data, "message"); msg != "" {
		if detail != "" {
			detail += ": "
		}
		detail += msg
	}
	if detail == "" {
		return "no details"
	}
	return detail
}

// ghcpStreamScan is the outcome of reading a whole event stream.
type ghcpStreamScan struct {
	text         string  // assistant text accumulated up to the first result
	linesDecoded int     // lines that decoded as JSON objects
	resultSeen   bool    // a result event was seen
	resultExit   int     // exitCode of the first result line
	deselected   bool    // a subagent.deselected event was seen
	sessionError *string // detail of the first session.error event, if any
}

// scanGHCPCLIStream reads every line of the stream. Text accumulation and the
// result verdict stop at the first result line; the deselection and
// session.error events are collected from the whole stream.
func scanGHCPCLIStream(stdout []byte) ghcpStreamScan {
	var scan ghcpStreamScan
	var accumulated strings.Builder

	for _, rawLine := range strings.Split(string(stdout), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var event ghcpCLIEventLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// A malformed line is skipped without aborting: it never
			// converts a failure into a success and never produces a
			// terminal verdict of its own.
			continue
		}
		scan.linesDecoded++

		switch event.Type {
		case ghcpEventAssistantMessage:
			if !scan.resultSeen {
				accumulated.WriteString(ghcpDataString(event.Data, "content"))
			}

		case ghcpEventResult:
			// The first result line is authoritative; later ones change nothing.
			if !scan.resultSeen {
				scan.resultSeen = true
				scan.resultExit = event.ExitCode
				scan.text = accumulated.String()
			}

		case ghcpEventDeselected:
			scan.deselected = true

		case ghcpEventSessionError:
			if scan.sessionError == nil {
				detail := ghcpSessionErrorDetail(event.Data)
				scan.sessionError = &detail
			}

		case ghcpEventUserMessage, ghcpEventAssistantMessageDelta:
			// Recognised; contributes nothing to the accumulator. user.message
			// also carries data.content (the prompt text); accumulating it would
			// echo the request back as the response. assistant.message_delta
			// streams the response chunk by chunk; the subsequent assistant.message
			// carries the same text complete, so accumulating both would double it.

		default:
			// Unrecognised type: ignored without error.
		}
	}

	if !scan.resultSeen {
		scan.text = accumulated.String()
	}
	return scan
}

// ParseGHCPCLIEnvelope reads `copilot -p ... --output-format json` output as a
// newline-delimited JSON event stream and returns the assistant text
// accumulated from its assistant.message events up to the first result event.
//
// The whole stream is read. Verdict precedence: subagent.deselected
// (ErrGHCPCLIAgentDeselected), then session.error (ErrGHCPCLISessionError),
// then the first result event's exit code, then an incomplete stream. The
// deselection error carries no agent name because the stream has none; callers
// that know the requested agent wrap it.
//
// stderr and exitCode are diagnostic context only: they are attached to the
// failure paths' error messages and can never produce a success verdict.
func ParseGHCPCLIEnvelope(stdout, stderr []byte, exitCode int) (string, error) {
	if len(bytes.TrimSpace(stdout)) == 0 {
		return "", ErrEmptyResponse
	}

	scan := scanGHCPCLIStream(stdout)
	if scan.linesDecoded == 0 {
		trimmed := strings.TrimSpace(string(stdout))
		if len(trimmed) > rawOutputCap {
			total := len(trimmed)
			trimmed = trimmed[:rawOutputCap] + fmt.Sprintf("... [truncated, %d bytes total]", total)
		}
		return "", fmt.Errorf("%w: %s", ErrMalformedJSON, trimmed)
	}

	stderrStr := strings.TrimSpace(string(stderr))
	switch {
	case scan.deselected:
		return "", fmt.Errorf("%w (stderr: %s; process exit code: %d)",
			ErrGHCPCLIAgentDeselected, stderrStr, exitCode)
	case scan.sessionError != nil:
		return "", fmt.Errorf("%w: %s (stderr: %s; process exit code: %d)",
			ErrGHCPCLISessionError, *scan.sessionError, stderrStr, exitCode)
	case !scan.resultSeen:
		return "", fmt.Errorf("%w (stderr: %s; process exit code: %d)",
			ErrGHCPCLIStreamIncomplete, stderrStr, exitCode)
	case scan.resultExit != 0:
		return "", fmt.Errorf("%w: stream exit code %d (stderr: %s; process exit code: %d)",
			ErrGHCPCLIStreamError, scan.resultExit, stderrStr, exitCode)
	}
	return scan.text, nil
}
