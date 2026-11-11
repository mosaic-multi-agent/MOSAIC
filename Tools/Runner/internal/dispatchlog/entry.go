package dispatchlog

import "mosaic-run/internal/domain"

// Entry is one decoded dispatch-log line, reduced to what readers need.
type Entry struct {
	Type            string            // "version", "correlation", "request", "response", "error"
	AgentInstanceID string            // request.agent_instance_id, or the entry's agent_instance_id; "" for version/correlation
	StatusCode      domain.StatusCode // response entries only
	ErrorCode       domain.ErrorCode  // response entries only
	HarnessError    bool              // error entries recorded with status "HARNESS_ERROR"
}
