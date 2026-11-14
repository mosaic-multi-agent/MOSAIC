package dispatchlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"mosaic-run/internal/domain"
)

const runLogSuffix = "-dispatch.log"

// RunLogPath returns the path a Logger rooted at workingDir uses for runID:
// {workingDir}/RunnerLogs/{runID}/{runID}-dispatch.log. No filesystem access.
func RunLogPath(workingDir, runID string) string {
	return filepath.Join(workingDir, LogsFolderName, runID, runID+runLogSuffix)
}

// IsRunLog reports whether path is a run's own dispatch log for runID: its base
// name is "{runID}-dispatch.log". False for "", for a startup-{ts} fallback
// file and for another run's log. No filesystem access.
func IsRunLog(path, runID string) bool {
	if path == "" || runID == "" {
		return false
	}
	return filepath.Base(path) == runID+runLogSuffix
}

// rawEntry is the union of the JSONL envelopes the Logger writes, reduced to
// the members ReadEntries needs.
type rawEntry struct {
	Type            string `json:"type"`
	AgentInstanceID string `json:"agent_instance_id"`
	Status          string `json:"status"`
	Request         *struct {
		AgentInstanceID string `json:"agent_instance_id"`
	} `json:"request"`
	Response *struct {
		StatusCode domain.StatusCode `json:"status_code"`
		ErrorCode  domain.ErrorCode  `json:"error_code"`
	} `json:"response"`
}

// ReadEntries decodes the JSONL dispatch log at path, in file order. A line
// that does not decode as an entry is skipped. Returns an error only when the
// file cannot be opened or read.
func ReadEntries(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	// A whole request or response is one line and may be arbitrarily large, so
	// lines are read without a fixed buffer limit.
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if e, ok := decodeEntry(line); ok {
			entries = append(entries, e)
		}
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return entries, err
		}
	}
}

// decodeEntry decodes one line; ok is false for a blank or undecodable line.
func decodeEntry(line []byte) (Entry, bool) {
	var raw rawEntry
	if err := json.Unmarshal(line, &raw); err != nil || raw.Type == "" {
		return Entry{}, false
	}
	e := Entry{Type: raw.Type, AgentInstanceID: raw.AgentInstanceID}
	if raw.Request != nil && e.AgentInstanceID == "" {
		e.AgentInstanceID = raw.Request.AgentInstanceID
	}
	if raw.Response != nil {
		e.StatusCode = raw.Response.StatusCode
		e.ErrorCode = raw.Response.ErrorCode
	}
	e.HarnessError = raw.Type == "error" && raw.Status == string(domain.StatusHARNESS_ERROR)
	return e, true
}
