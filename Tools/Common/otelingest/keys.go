package otelingest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	orchestratorStream = "00_orchestrator_events.jsonl"
	invocationStream   = "03_events.jsonl"
)

// runKeys are the attribution keys of one run. They come only from the run's MOSAIC log events: the envelope
// session_id of the orchestrator stream and of the invocation streams, and invocation_start.spawn_call_id.
type runKeys struct {
	orchSessions     map[string]bool
	invBySession     map[string][]string // session_id -> invocation ids
	invBySpawn       map[string][]string // spawn_call_id -> invocation ids
	harnessBySession map[string]string
	harnessByInv     map[string]string
	firstHarness     string
}

type logEvent struct {
	Event       string `json:"event"`
	Source      string `json:"source"`
	Harness     string `json:"harness"`
	SessionID   string `json:"session_id"`
	SpawnCallID string `json:"spawn_call_id"`
}

// readRunKeys reads the attribution keys from the log folder of a run.
func readRunKeys(runDir string) *runKeys {
	k := &runKeys{
		orchSessions:     map[string]bool{},
		invBySession:     map[string][]string{},
		invBySpawn:       map[string][]string{},
		harnessBySession: map[string]string{},
		harnessByInv:     map[string]string{},
	}
	eachLogEvent(filepath.Join(runDir, orchestratorStream), func(e logEvent) {
		if e.SessionID != "" {
			k.orchSessions[e.SessionID] = true
			k.noteHarness(e.SessionID, e.Harness)
		}
	})
	entries, _ := os.ReadDir(runDir)
	for _, d := range entries {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		inv := d.Name()
		eachLogEvent(filepath.Join(runDir, inv, invocationStream), func(e logEvent) {
			if e.SessionID != "" {
				k.invBySession[e.SessionID] = appendUnique(k.invBySession[e.SessionID], inv)
				k.noteHarness(e.SessionID, e.Harness)
			}
			if e.SpawnCallID != "" {
				k.invBySpawn[e.SpawnCallID] = appendUnique(k.invBySpawn[e.SpawnCallID], inv)
			}
			if _, ok := k.harnessByInv[inv]; !ok && e.Harness != "" {
				k.harnessByInv[inv] = e.Harness
			}
		})
	}
	return k
}

func (k *runKeys) noteHarness(session, harness string) {
	if harness == "" {
		return
	}
	if _, ok := k.harnessBySession[session]; !ok {
		k.harnessBySession[session] = harness
	}
	if k.firstHarness == "" {
		k.firstHarness = harness
	}
}

// eachLogEvent calls fn for every parsable event of a JSONL log, skipping OTel-sourced usage records so the
// ingest never derives keys from its own output.
func eachLogEvent(path string, fn func(logEvent)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var e logEvent
			if json.Unmarshal(line, &e) == nil && !(e.Event == "usage_record" && e.Source == "otel") {
				fn(e)
			}
		}
		if err != nil {
			return
		}
	}
}

// known reports whether a conversation or session id belongs to the run.
func (k *runKeys) known(session string) bool {
	return session != "" && (k.orchSessions[session] || len(k.invBySession[session]) > 0)
}

// conversation is the run session a call belongs to: its conversation id, else its parent chat session.
func (k *runKeys) conversation(c *call) string {
	if k.known(c.ConversationID) {
		return c.ConversationID
	}
	if k.known(c.ParentChatSessionID) {
		return c.ParentChatSessionID
	}
	return ""
}

// tied reports whether any hard key ties the span to the run.
func (k *runKeys) tied(conv, chatSession, parentChatSession string) bool {
	return k.known(conv) || k.known(parentChatSession) || (chatSession != "" && len(k.invBySpawn[chatSession]) > 0)
}

// sessionIDs lists the run's session ids, sorted.
func (k *runKeys) sessionIDs() []string {
	set := map[string]bool{}
	for s := range k.orchSessions {
		set[s] = true
	}
	for s := range k.invBySession {
		set[s] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
