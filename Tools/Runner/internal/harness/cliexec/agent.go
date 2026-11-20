package cliexec

import (
	commonharness "mosaic-common/harness"
)

// WithAgent returns a Sink that forwards every event to next with the field
// agent=<agent> placed before the event's own fields. Name and message are
// unchanged. next must be non-nil.
func WithAgent(next commonharness.Sink, agent string) commonharness.Sink {
	return agentSink{next: next, agent: agent}
}

// agentSink attributes each event to one invocation. It holds no mutable
// state, so wrappers around one shared sink never affect each other.
type agentSink struct {
	next  commonharness.Sink
	agent string
}

// Log implements commonharness.Sink.
func (s agentSink) Log(ev commonharness.Event) {
	fields := make([]commonharness.Field, 0, len(ev.Fields)+1)
	fields = append(fields, commonharness.Field{Key: "agent", Value: s.agent})
	fields = append(fields, ev.Fields...)
	ev.Fields = fields
	s.next.Log(ev)
}
