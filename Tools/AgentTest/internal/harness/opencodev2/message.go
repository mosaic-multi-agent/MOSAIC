package opencodev2

// Recovery of the Communication Protocol message carried in a dispatch prompt.

import (
	"encoding/json"

	"mosaic-agent-test/internal/domain"
)

// parseTaskMessage recovers the Communication Protocol invocation carried in
// a dispatch tool's prompt text. raw is preserved verbatim regardless of
// whether it parses, so protocol validation downstream can still inspect
// what was actually sent.
//
// When the entire raw string is not valid JSON, the function scans for
// embedded JSON objects by locating each '{' and walking forward to its
// matching '}' (tracking nested braces). Each candidate is attempted in
// order; the first one that unmarshals successfully and carries a non-empty
// agent_instance_id is returned as ExtractionRecovered. If no candidate
// satisfies the requirement, ExtractionDegraded is returned.
func parseTaskMessage(raw string) domain.TaskMessage {
	if raw == "" {
		return domain.TaskMessage{Extraction: domain.ExtractionDegraded}
	}
	var tm domain.TaskMessage
	if err := json.Unmarshal([]byte(raw), &tm); err == nil && tm.AgentInstanceID != "" {
		tm.Raw = raw
		tm.Extraction = domain.ExtractionParsed
		return tm
	}
	// Fallback: scan for embedded JSON objects within prose.
	if recovered, ok := extractJSONObject(raw); ok {
		recovered.Raw = raw
		recovered.Extraction = domain.ExtractionRecovered
		return recovered
	}
	return domain.TaskMessage{Raw: raw, Extraction: domain.ExtractionDegraded}
}

// extractJSONObject scans s for all top-level '{...}' substrings (handling
// nested braces) and returns the first domain.TaskMessage that unmarshals
// successfully and carries a non-empty AgentInstanceID. The second return
// value is false when no such object is found.
func extractJSONObject(s string) (domain.TaskMessage, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		// Walk forward tracking brace depth to find the matching '}'.
		depth := 0
		end := -1
		for j := i; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			// No closing brace found from this '{'; no further candidates possible.
			break
		}
		candidate := s[i : end+1]
		var tm domain.TaskMessage
		if err := json.Unmarshal([]byte(candidate), &tm); err == nil && tm.AgentInstanceID != "" {
			return tm, true
		}
		// Advance past this object to try the next one.
		i = end
	}
	return domain.TaskMessage{}, false
}
