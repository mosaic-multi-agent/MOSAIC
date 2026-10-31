package logread

import (
	"encoding/json"

	"mosaic-log-analyzer/internal/domain"
)

// decodeRequestedTools reads the requested_tools object; anything invalid reads as
// not provided, without a finding.
func decodeRequestedTools(raw json.RawMessage) domain.RequestedTools {
	return domain.ParseRequestedTools(raw)
}
