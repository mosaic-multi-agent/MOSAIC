package otelingest

// statusCodeError is the OpenTelemetry span status code of a failed span.
const statusCodeError = 2

// failedTool is a failed execute_tool span with a call id: the facts the end of its tool call is written from.
type failedTool struct {
	callID   string
	toolName string
	message  string
	endTime  string // formatted span end time; "" when the span has no usable endTime
}

// failedToolOf returns the failed tool call a span describes, or nil when the span did not fail or has no call id.
// Content attributes (arguments, results) are never read.
func failedToolOf(obj, attrs map[string]any) *failedTool {
	status, _ := obj["status"].(map[string]any)
	if code := intValue(status["code"]); code == nil || *code != statusCodeError {
		return nil
	}
	callID := stringValue(attrs["gen_ai.tool.call.id"])
	if callID == "" {
		return nil
	}
	return &failedTool{
		callID:   callID,
		toolName: stringValue(attrs["gen_ai.tool.name"]),
		message:  stringValue(status["message"]),
		endTime:  startTimestamp(obj["endTime"]),
	}
}
