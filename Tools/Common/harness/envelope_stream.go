package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// parseClaudeCodeStream interprets data as the --output-format stream-json
// JSONL transcript. recognised is false when no line is a typed JSON object,
// meaning data is not a stream transcript and the caller should fall back to
// raw-text recovery.
//
// When recognised, the last line of type "result" supplies the text. A stream
// with no result line, or whose last result line has no "result" field, yields
// ErrProtocolNotExtractable, matching the single-object form. Lines that do
// not parse (for example a truncated final line) are skipped.
func parseClaudeCodeStream(data []byte) (text string, recognised bool, err error) {
	var last *cliEnvelopeObject
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var obj cliEnvelopeObject
		if json.Unmarshal(line, &obj) != nil || obj.Type == "" {
			continue
		}
		recognised = true
		if obj.Type == "result" {
			o := obj
			last = &o
		}
	}
	if !recognised {
		return "", false, nil
	}
	if last == nil {
		return "", true, fmt.Errorf("%w: no result line in CLI stream", ErrProtocolNotExtractable)
	}
	if last.Result == nil {
		return "", true, fmt.Errorf("%w: no result field in CLI stream result line", ErrProtocolNotExtractable)
	}
	return *last.Result, true, nil
}
