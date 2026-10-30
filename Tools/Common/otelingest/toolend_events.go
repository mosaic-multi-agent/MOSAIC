package otelingest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// toolEvent is the part of a tool_call_start or tool_call_end line the end matching needs.
type toolEvent struct {
	Event     string `json:"event"`
	CallID    string `json:"call_id"`
	ToolName  string `json:"tool_name"`
	Harness   string `json:"harness"`
	SessionID string `json:"session_id"`
}

// toolStream holds the tool calls of one stream file: the first start of every call id (in file order) and the
// call ids that already have an end.
type toolStream struct {
	path   string
	starts []toolEvent
	ends   map[string]bool
}

// vscodeCallSuffix is the suffix the VS Code hook appends to a logged call id.
var vscodeCallSuffix = regexp.MustCompile(`__vscode-[0-9]+$`)

// streamPaths lists the stream files that can hold tool events: the orchestrator stream and the invocation streams.
func streamPaths(runDir string) []string {
	paths := []string{filepath.Join(runDir, orchestratorStream)}
	entries, _ := os.ReadDir(runDir)
	for _, d := range entries {
		if d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			paths = append(paths, filepath.Join(runDir, d.Name(), invocationStream))
		}
	}
	return paths
}

// readToolStream reads the tool starts and ends of one stream file; a missing file is an empty stream.
func readToolStream(path string) *toolStream {
	ts := &toolStream{path: path, ends: map[string]bool{}}
	f, err := os.Open(path)
	if err != nil {
		return ts
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var e toolEvent
			if json.Unmarshal(line, &e) == nil && e.CallID != "" {
				switch e.Event {
				case "tool_call_start":
					ts.starts = append(ts.starts, e)
				case "tool_call_end":
					ts.ends[e.CallID] = true
				}
			}
		}
		if err != nil {
			return ts
		}
	}
}

// matches reports whether a logged start call id belongs to the span call id: verbatim for a GHCP CLI span, and
// after removing the VS Code suffix for a VS Code span.
func matches(dialect, startCallID, spanCallID string) bool {
	if dialect == dialectCLI {
		return startCallID == spanCallID
	}
	return vscodeCallSuffix.ReplaceAllString(startCallID, "") == spanCallID
}

// firstStart is the first start in the stream that belongs to the span, if any.
func (ts *toolStream) firstStart(t *failedTool) (toolEvent, bool) {
	for _, s := range ts.starts {
		if matches(t.dialect, s.CallID, t.callID) {
			return s, true
		}
	}
	return toolEvent{}, false
}
