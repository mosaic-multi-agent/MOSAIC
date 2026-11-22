// OpenCode V2 stream parser, verdict and sentinels.
package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrOpenCodeV2UnknownAgent     = errors.New("harness: opencode-v2 unknown agent")
	ErrOpenCodeV2UnknownModel     = errors.New("harness: opencode-v2 unknown model")
	ErrOpenCodeV2UsageError       = errors.New("harness: opencode-v2 rejected the command line")
	ErrOpenCodeV2RunFailed        = errors.New("harness: opencode-v2 run failed")
	ErrOpenCodeV2Interrupted      = errors.New("harness: opencode-v2 run interrupted")
	ErrOpenCodeV2Terminated       = errors.New("harness: opencode-v2 run terminated")
	ErrOpenCodeV2StreamIncomplete = errors.New("harness: opencode-v2 stream ended without a final answer")
)

// OpenCodeV2Error is the top-level error event of a V2 stream.
type OpenCodeV2Error struct {
	Type    string // error.type, e.g. "unknown", "provider.no-route"
	Message string // error.message verbatim
}

// OpenCodeV2Stream is the pure result of parsing one V2 stream.
type OpenCodeV2Stream struct {
	SessionID string           // first non-empty top-level sessionID; "" if none
	Complete  bool             // the last step_finish is a stop and no background_wait follows it
	FinalText string           // final-text rule; "" when !Complete
	Error     *OpenCodeV2Error // last top-level error event; nil when none
}

const (
	ocv2EventStepStart      = "step_start"
	ocv2EventText           = "text"
	ocv2EventStepFinish     = "step_finish"
	ocv2EventBackgroundWait = "background_wait"
	ocv2EventError          = "error"

	ocv2ReasonStop = "stop"

	ocv2ExitInterrupted = 130
	ocv2ExitTerminated  = 143

	ocv2UsageDetailRunes = 200
)

// ocv2EventLine is the subset of a V2 stream line the parser reads. cost and
// tokens are deliberately not decoded: money cost is never taken from the stream.
type ocv2EventLine struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Part      struct {
		Text   string `json:"text"`
		Reason string `json:"reason"`
	} `json:"part"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ocv2StreamState tracks the turn structure while a stream is read.
//
// Final-text rule: a turn ends at a step_finish with reason "stop". The final
// text is the text of the last text-bearing step of the final turn, several
// text parts of one step joined with "\n". A stop followed by background_wait
// is an intermediate turn, so a stream ending there is incomplete, as is a
// stream whose last step_finish has any other reason or that has none.
type ocv2StreamState struct {
	stepTexts []string // text parts of the step currently open
	turnText  string   // text of the last text-bearing finished step of the open turn

	sawFinish  bool
	lastReason string
	waitAfter  bool // a background_wait followed the last step_finish
	finalText  string
}

func (s *ocv2StreamState) finishStep(reason string) {
	if text := strings.Join(s.stepTexts, "\n"); text != "" {
		s.turnText = text
	}
	s.stepTexts = nil
	s.sawFinish = true
	s.lastReason = reason
	s.waitAfter = false
	if reason == ocv2ReasonStop {
		s.finalText = s.turnText
		s.turnText = ""
	}
}

func (s *ocv2StreamState) complete() bool {
	return s.sawFinish && s.lastReason == ocv2ReasonStop && !s.waitAfter
}

// ParseOpenCodeV2Stream decodes a complete `run --format json` stream. Pure.
func ParseOpenCodeV2Stream(data []byte) (OpenCodeV2Stream, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return OpenCodeV2Stream{}, ErrEmptyResponse
	}

	var out OpenCodeV2Stream
	var st ocv2StreamState
	linesDecoded := 0

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		var ev ocv2EventLine
		if line[0] != '{' || json.Unmarshal([]byte(line), &ev) != nil {
			// Non-JSON and non-object lines are skipped.
			continue
		}
		linesDecoded++
		if out.SessionID == "" {
			out.SessionID = ev.SessionID
		}

		switch ev.Type {
		case ocv2EventStepStart:
			st.stepTexts = nil
		case ocv2EventText:
			st.stepTexts = append(st.stepTexts, ev.Part.Text)
		case ocv2EventStepFinish:
			st.finishStep(ev.Part.Reason)
		case ocv2EventBackgroundWait:
			st.waitAfter = true
		case ocv2EventError:
			if ev.Error != nil {
				out.Error = &OpenCodeV2Error{Type: ev.Error.Type, Message: ev.Error.Message}
			}
		}
	}

	if linesDecoded == 0 {
		trimmed := strings.TrimSpace(string(data))
		if len(trimmed) > rawOutputCap {
			total := len(trimmed)
			trimmed = trimmed[:rawOutputCap] + fmt.Sprintf("... [truncated, %d bytes total]", total)
		}
		return OpenCodeV2Stream{}, fmt.Errorf("%w: %s", ErrMalformedJSON, trimmed)
	}

	out.Complete = st.complete()
	if out.Complete {
		out.FinalText = st.finalText
	}
	return out, nil
}

// openCodeV2Failure is a verdict failure: one sentinel, an optional detail and
// the exit status. It also matches ErrNonZeroExit when the exit was non-zero.
type openCodeV2Failure struct {
	sentinel error
	detail   string
	exit     int
}

func (f *openCodeV2Failure) Error() string {
	msg := f.sentinel.Error()
	if f.detail != "" {
		msg += ": " + f.detail
	}
	if f.exit != 0 {
		msg += " (exit status " + strconv.Itoa(f.exit) + ")"
	}
	return msg
}

func (f *openCodeV2Failure) Unwrap() []error {
	if f.exit != 0 {
		return []error{f.sentinel, ErrNonZeroExit}
	}
	return []error{f.sentinel}
}

// OpenCodeV2Verdict decides one finished V2 invocation from Run's result. Pure.
func OpenCodeV2Verdict(resp Response, runErr error) (text string, err error) {
	if runErr != nil && !errors.Is(runErr, ErrNonZeroExit) {
		return "", runErr
	}
	exit := resp.ExitCode
	fail := func(sentinel error, detail string) (string, error) {
		return "", &openCodeV2Failure{sentinel: sentinel, detail: detail, exit: exit}
	}

	switch exit {
	case ocv2ExitInterrupted:
		return fail(ErrOpenCodeV2Interrupted, "")
	case ocv2ExitTerminated:
		return fail(ErrOpenCodeV2Terminated, "")
	}

	stream, parseErr := ParseOpenCodeV2Stream(resp.Stdout)

	if parseErr == nil && stream.Error != nil {
		switch {
		case strings.HasPrefix(stream.Error.Message, "Agent not found"):
			return fail(ErrOpenCodeV2UnknownAgent, stream.Error.Message)
		case stream.Error.Type == "provider.no-route":
			return fail(ErrOpenCodeV2UnknownModel, stream.Error.Message)
		default:
			return fail(ErrOpenCodeV2RunFailed, stream.Error.Message)
		}
	}

	if exit != 0 {
		if parseErr != nil {
			return fail(ErrOpenCodeV2UsageError, usageDetail(resp))
		}
		return fail(ErrOpenCodeV2RunFailed, "")
	}

	if parseErr != nil {
		return "", parseErr
	}
	if !stream.Complete {
		return fail(ErrOpenCodeV2StreamIncomplete, incompleteReason(resp.Stdout))
	}
	if strings.TrimSpace(stream.FinalText) == "" {
		return "", ErrEmptyResponse
	}
	return stream.FinalText, nil
}

// usageDetail is the first non-blank line of stderr, else of stdout, capped.
func usageDetail(resp Response) string {
	line := firstNonBlankLine(resp.Stderr)
	if line == "" {
		line = firstNonBlankLine(string(resp.Stdout))
	}
	if r := []rune(line); len(r) > ocv2UsageDetailRunes {
		line = string(r[:ocv2UsageDetailRunes])
	}
	return line
}

func firstNonBlankLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return t
		}
	}
	return ""
}

// incompleteReason names why a stream that parsed has no final answer. The
// stdout is re-scanned for the last step_finish reason, which the parse result
// does not carry.
func incompleteReason(stdout []byte) string {
	var st ocv2StreamState
	for _, rawLine := range strings.Split(string(stdout), "\n") {
		var ev ocv2EventLine
		if err := json.Unmarshal([]byte(strings.TrimSpace(rawLine)), &ev); err != nil {
			continue
		}
		switch ev.Type {
		case ocv2EventStepFinish:
			st.finishStep(ev.Part.Reason)
		case ocv2EventBackgroundWait:
			st.waitAfter = true
		}
	}
	switch {
	case st.waitAfter:
		return "waiting for background subagents"
	case !st.sawFinish:
		return "no step finished"
	default:
		return "last step finished with reason " + st.lastReason
	}
}
