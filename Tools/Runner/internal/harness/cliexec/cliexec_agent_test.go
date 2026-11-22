package cliexec_test

// Tests for WithAgent: attributing the shared package's events to one
// invocation before they reach the Runner debug log.

import (
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/cliexec"
)

func TestWithAgentPlacesTheAgentFieldBeforeTheEventsOwnFields(t *testing.T) {
	logger := &recordingLogger{}
	sink := cliexec.WithAgent(cliexec.NewSink(logger), "dev#3")

	sink.Log(commonharness.Event{
		Name:    "spawn.stdout.lines",
		Message: "a\nb",
		Fields:  []commonharness.Field{{Key: "first", Value: "12"}, {Key: "lines", Value: "2"}},
	})

	if len(logger.entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(logger.entries))
	}
	e := logger.entries[0]
	if e.event != domain.EventHarnessStdoutLines || e.message != "a\nb" {
		t.Errorf("want the mapped name and unchanged message, got %q / %q", e.event, e.message)
	}
	want := []domain.DebugField{{Key: "agent", Value: "dev#3"}, {Key: "first", Value: "12"}, {Key: "lines", Value: "2"}}
	if len(e.fields) != len(want) {
		t.Fatalf("fields = %v, want %v", e.fields, want)
	}
	for i := range want {
		if e.fields[i] != want[i] {
			t.Errorf("field[%d] = %v, want %v", i, e.fields[i], want[i])
		}
	}
}

func TestWithAgentAddsTheAgentToAnEventWithoutFields(t *testing.T) {
	logger := &recordingLogger{}

	cliexec.WithAgent(cliexec.NewSink(logger), "orchestrator-agent").Log(commonharness.Event{Name: "spawn.stderr", Message: "boom"})

	if len(logger.entries) != 1 || len(logger.entries[0].fields) != 1 ||
		logger.entries[0].fields[0] != (domain.DebugField{Key: "agent", Value: "orchestrator-agent"}) {
		t.Fatalf("entries = %+v, want one entry with only agent=orchestrator-agent", logger.entries)
	}
}

func TestWithAgentKeepsTheEndEventFieldsAfterTheAgent(t *testing.T) {
	logger := &recordingLogger{}

	cliexec.WithAgent(cliexec.NewSink(logger), "dev#3").Log(commonharness.Event{
		Name:    "spawn.end",
		Message: "process ended: cancelled",
		Fields:  []commonharness.Field{{Key: "reason", Value: "cancelled"}, {Key: "stdout_bytes", Value: "7"}},
	})

	e := logger.entries[0]
	if e.event != domain.EventHarnessProcessEnd || len(e.fields) != 3 ||
		e.fields[0].Key != "agent" || e.fields[1] != (domain.DebugField{Key: "reason", Value: "cancelled"}) ||
		e.fields[2] != (domain.DebugField{Key: "stdout_bytes", Value: "7"}) {
		t.Errorf("want harness.process.end with agent, reason, stdout_bytes in order, got %+v", e)
	}
}

func TestWithAgentDoesNotShareStateAcrossInvocations(t *testing.T) {
	logger := &recordingLogger{}
	shared := cliexec.NewSink(logger)
	first, second := cliexec.WithAgent(shared, "a#1"), cliexec.WithAgent(shared, "b#2")

	first.Log(commonharness.Event{Name: "spawn.stderr", Message: "x"})
	second.Log(commonharness.Event{Name: "spawn.stderr", Message: "y"})

	if len(logger.entries) != 2 || len(logger.entries[0].fields) == 0 || len(logger.entries[1].fields) == 0 ||
		logger.entries[0].fields[0].Value != "a#1" || logger.entries[1].fields[0].Value != "b#2" {
		t.Errorf("want each event attributed to its own wrapper's agent, got %+v", logger.entries)
	}
}
