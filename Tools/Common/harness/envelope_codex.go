// Codex event-stream parsing for `codex exec --json`: a newline-delimited JSON
// stream of thread, turn and item events. Like the OpenCode parser it derives
// success and failure from the stream's own content, never from an exit code,
// and tolerates unknown events and fields.
package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrCodexStreamError reports that `codex exec` reported an error (a failed
	// turn or an unrecovered error event).
	ErrCodexStreamError = errors.New("harness: codex exec reported an error")
	// ErrCodexStreamIncomplete reports an event stream that ended without a
	// terminal event.
	ErrCodexStreamIncomplete = errors.New("harness: codex event stream ended without a terminal event")
)

// Recognised discriminator values in the Codex event stream.
const (
	codexEventTurnStarted   = "turn.started"
	codexEventTurnCompleted = "turn.completed"
	codexEventTurnFailed    = "turn.failed"
	codexEventItemCompleted = "item.completed"
	codexEventError         = "error"

	codexItemAgentMessage = "agent_message"
)

// codexEventLine is one stream line decoded to only the fields the parser reads.
type codexEventLine struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Item    struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ParseCodexEnvelope scans a `codex exec --json` stdout stream and returns the
// final agent message text of the last terminal turn. A completed turn yields
// its last agent message; a failed turn is ErrCodexStreamError; a stream with no
// terminal event is ErrCodexStreamError when an error event was seen, otherwise
// ErrCodexStreamIncomplete. Output without any JSON event is ErrEmptyResponse.
func ParseCodexEnvelope(data []byte) (string, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return "", ErrEmptyResponse
	}

	decoded := 0
	turnMessage := ""   // last agent message of the turn in progress
	verdictSet := false // a terminal event was seen
	verdictText := ""
	var verdictErr error
	lastErrorMessage, sawError := "", false

	for _, raw := range bytes.Split(data, []byte("\n")) {
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			continue
		}
		var event codexEventLine
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		decoded++

		switch event.Type {
		case codexEventTurnStarted:
			turnMessage = ""
		case codexEventItemCompleted:
			if event.Item.Type == codexItemAgentMessage && event.Item.Text != "" {
				turnMessage = event.Item.Text
			}
		case codexEventTurnCompleted:
			verdictSet = true
			verdictText, verdictErr = turnMessage, nil
			if turnMessage == "" {
				verdictErr = ErrEmptyResponse
			}
			turnMessage = ""
		case codexEventTurnFailed:
			verdictSet = true
			verdictText = ""
			verdictErr = fmt.Errorf("%w: %s", ErrCodexStreamError, event.Error.Message)
			turnMessage = ""
		case codexEventError:
			sawError = true
			lastErrorMessage = event.Message
		}
	}

	switch {
	case verdictSet:
		return verdictText, verdictErr
	case sawError:
		return "", fmt.Errorf("%w: %s", ErrCodexStreamError, lastErrorMessage)
	case decoded == 0:
		return "", ErrEmptyResponse
	default:
		return "", ErrCodexStreamIncomplete
	}
}
