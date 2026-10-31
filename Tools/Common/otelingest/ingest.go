// Package otelingest ingests Copilot OpenTelemetry usage into a run's MOSAIC log folder: it reads an OTel JSONL
// file (VS Code dialect), turns each per-call chat span into a usage_record with source "otel" and
// places it on the stream of the actor a hard key ties it to.
//
// This package mirrors the Python ingest module of the mosaic-logger hook bundle (counterpart file:
// Catalog/Hooks/mosaic-logger/copilot-shared/mosaic_logger_otel.py). Behaviour is defined by the conformance
// fixtures in testdata/conformance, which both implementations run.
package otelingest

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"mosaic-common/otelingest/diag"
)

// Ingest runs one ingest of the OTel file into the run folder.
func Ingest(req Request) Result {
	now := time.Now()
	if req.Now != nil {
		now = req.Now()
	}
	info, err := os.Stat(req.OTelPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Result{Status: StatusFileMissing, Diagnostics: []diag.Diagnostic{{Kind: diag.KindFileMissing, Message: "OTel file does not exist"}}}
		}
		return Result{Status: StatusFailed, Err: err}
	}
	release, held, err := acquireLock(req.RunDir, now)
	if err != nil {
		return Result{Status: StatusFailed, Err: err}
	}
	if held {
		return Result{Status: StatusLockHeld, Diagnostics: []diag.Diagnostic{{Kind: diag.KindLockHeld, Message: "another ingest of this run holds the lock"}}}
	}
	defer release()
	st, _, unknown, err := loadState(req.RunDir)
	if err != nil {
		return Result{Status: StatusFailed, Err: err}
	}
	if unknown != nil {
		d := diag.Diagnostic{Kind: diag.KindUnknownStateVersion, Message: "unknown .otel-ingest state version"}
		if *unknown != "" {
			d.IDs = []string{*unknown}
		}
		return Result{Status: StatusUnknownState, Diagnostics: []diag.Diagnostic{d}}
	}
	res, err := ingestInto(req, st, info.Size(), now)
	if err != nil {
		return Result{Status: StatusFailed, Err: err}
	}
	return res
}

// ingestInto reads the new lines, places every call it can and saves the state.
func ingestInto(req Request, st *ingestState, size int64, now time.Time) (Result, error) {
	start := startOffset(req, st, size)
	sc, err := scanFile(req.OTelPath, start)
	if err != nil {
		return Result{}, err
	}
	keys := readRunKeys(req.RunDir)
	var calls []*call
	var failed []*failedTool
	for _, c := range st.Pending {
		if c != nil {
			calls = append(calls, c)
		}
	}
	for _, p := range sc.lines {
		if p.tool != nil {
			failed = append(failed, p.tool)
		}
		if p.link != nil && keys.tied(p.link.ConversationID, p.link.ChatSessionID, p.link.ParentChatSessionID) {
			st.Links[p.linkID] = p.link
		}
		if p.call != nil && !hasCall(calls, p.call.RecordID) {
			if p.call.Timestamp == "" {
				p.call.Timestamp = now.UTC().Format(timestampLayout)
			}
			calls = append(calls, p.call)
		}
	}
	res := Result{Status: StatusIngested, LinesRead: sc.read, LinesMalformed: sc.malformed}
	w := &writer{runDir: req.RunDir, runID: filepath.Base(req.RunDir), now: now, keys: keys}
	if err := placeCalls(req, st, newResolver(keys, st.Links), w, calls, &res); err != nil {
		return Result{}, err
	}
	if err := writeToolEnds(req.RunDir, w.runID, now, failed, &res); err != nil {
		return Result{}, err
	}
	end := max(sc.end, start)
	headLen, sum, err := headDigest(req.OTelPath, max(size, end))
	if err != nil {
		return Result{}, err
	}
	st.Cursor = &cursor{Offset: end, HeadLen: headLen, HeadSHA256: sum}
	if err := saveState(req.RunDir, st); err != nil {
		return Result{}, err
	}
	res.Diagnostics = append(res.Diagnostics, readingDiagnostics(sc)...)
	sort.SliceStable(res.Diagnostics, func(i, j int) bool { return res.Diagnostics[i].Kind < res.Diagnostics[j].Kind })
	return res, nil
}

// startOffset chooses where reading starts: byte 0 for a whole-file recovery or when the stored cursor no longer
// describes the file, the stored cursor otherwise, and the requested first offset when there is no cursor yet.
func startOffset(req Request, st *ingestState, size int64) int64 {
	switch {
	case req.WholeFile:
		return 0
	case st.Cursor != nil:
		if cursorValid(st.Cursor, req.OTelPath, size) {
			return st.Cursor.Offset
		}
		return 0
	case req.StartOffset != nil && *req.StartOffset >= 0 && *req.StartOffset <= size:
		return *req.StartOffset
	}
	return 0
}

// readingDiagnostics reports the lines that could not be parsed.
func readingDiagnostics(sc scanned) []diag.Diagnostic {
	if sc.malformed == 0 {
		return nil
	}
	return []diag.Diagnostic{{
		Kind:    diag.KindLinesSkipped,
		Message: "lines that could not be parsed were skipped",
		Counts:  map[string]int{"malformed": sc.malformed, "read": sc.read},
	}}
}

// ReadPendingStatus reads the state without taking the lock and without writing.
func ReadPendingStatus(runDir string) PendingStatus {
	st, present, unknown, err := loadState(runDir)
	if err != nil || unknown != nil {
		return PendingStatus{StatePresent: present}
	}
	return PendingStatus{StatePresent: present, StateKnown: present, Pending: len(st.Pending)}
}

// hasCall reports whether a call with the record id is already in the list; a span read again while pending stays
// pending once.
func hasCall(calls []*call, recordID string) bool {
	for _, c := range calls {
		if c.RecordID == recordID {
			return true
		}
	}
	return false
}
