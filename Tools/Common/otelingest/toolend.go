package otelingest

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"mosaic-common/logappend"
	"mosaic-common/otelingest/diag"
)

// toolEnd is the tool_call_end event written for a failed tool call.
type toolEnd struct {
	SchemaVersion string `json:"schema_version"`
	Event         string `json:"event"`
	Timestamp     string `json:"timestamp"`
	Harness       string `json:"harness"`
	SessionID     string `json:"session_id,omitempty"`
	RunID         string `json:"run_id"`
	CallID        string `json:"call_id"`
	ToolName      string `json:"tool_name,omitempty"`
	Status        string `json:"status"`
	Error         string `json:"error,omitempty"`
}

// writeToolEnds writes one tool_call_end for every failed tool whose call has exactly one stream holding its start
// and no end there yet. Nothing is kept for failed tools without a start: a whole-file ingest matches them later.
func writeToolEnds(runDir, runID string, now time.Time, tools []*failedTool, res *Result) error {
	if len(tools) == 0 {
		return nil
	}
	var streams []*toolStream
	for _, p := range streamPaths(runDir) {
		streams = append(streams, readToolStream(p))
	}
	timedOut := 0
	for _, t := range tools {
		ts, start, ok := owningStream(streams, t)
		if !ok || ts.ends[start.CallID] {
			continue
		}
		written, err := appendToolEnd(ts, start, toolEndOf(runID, now, t, start))
		if err != nil {
			if !errors.Is(err, logappend.ErrLockTimeout) {
				return err
			}
			timedOut++
			continue
		}
		if written {
			res.ToolEndsWritten++
		}
		ts.ends[start.CallID] = true
	}
	if timedOut > 0 {
		res.Diagnostics = append(res.Diagnostics, diag.Diagnostic{
			Kind:    diag.KindAppendTimedOut,
			Message: "lines were not written because the append lock wait limit was reached; a later ingest writes them",
			Counts:  map[string]int{"lines": timedOut},
		})
	}
	return nil
}

// owningStream is the only stream that holds a start for the failed tool; a start in two or more streams is
// ambiguous and gives none.
func owningStream(streams []*toolStream, t *failedTool) (*toolStream, toolEvent, bool) {
	var found *toolStream
	var start toolEvent
	for _, ts := range streams {
		s, ok := ts.firstStart(t)
		if !ok {
			continue
		}
		if found != nil {
			return nil, toolEvent{}, false
		}
		found, start = ts, s
	}
	return found, start, found != nil
}

// toolEndOf builds the end of a failed tool call from the span and the start it belongs to.
func toolEndOf(runID string, now time.Time, t *failedTool, start toolEvent) toolEnd {
	end := toolEnd{
		SchemaVersion: recordSchemaVersion,
		Event:         "tool_call_end",
		Timestamp:     t.endTime,
		Harness:       start.Harness,
		SessionID:     start.SessionID,
		RunID:         runID,
		CallID:        start.CallID,
		ToolName:      t.toolName,
		Status:        "error",
		Error:         t.message,
	}
	if end.Timestamp == "" {
		end.Timestamp = now.UTC().Format(timestampLayout)
	}
	if end.ToolName == "" {
		end.ToolName = start.ToolName
	}
	return end
}

// appendToolEnd appends the end to the stream under its lock, after checking again under the lock that a hook has
// not written an end meanwhile. written is false when it had.
func appendToolEnd(ts *toolStream, start toolEvent, end toolEnd) (written bool, err error) {
	line, err := json.Marshal(end)
	if err != nil {
		return false, err
	}
	err = logappend.WithLock(ts.path, func() error {
		if readToolStream(ts.path).ends[start.CallID] {
			return nil
		}
		f, err := os.OpenFile(ts.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = f.Write(append(line, '\n'))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		written = err == nil
		return err
	})
	return written, err
}
