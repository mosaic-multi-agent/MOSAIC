package otelingest

// Attribution rules. All placement decisions live in this file, separate from the dialect parsers, so they can be
// adjusted after live checks. A call is placed only by hard keys, never by time window.

const (
	destOrchestrator = "orchestrator"
	destUnattributed = "unattributed"
	destInvocation   = "invocation/"

	maxOwnerDepth = 8
)

// resolver places calls using the run's keys and the span links known so far.
type resolver struct {
	keys   *runKeys
	links  map[string]*link
	byCall map[string]*link // tool_call_id -> execute_tool link
}

func newResolver(keys *runKeys, links map[string]*link) *resolver {
	r := &resolver{keys: keys, links: links, byCall: map[string]*link{}}
	for _, l := range links {
		if l.Kind == kindExecuteTool && l.ToolCallID != "" {
			r.byCall[l.ToolCallID] = l
		}
	}
	return r
}

// place returns the destination of a call tied to the run. ok is false when the keys needed to decide have not
// arrived yet (the call stays pending).
func (r *resolver) place(c *call) (dest string, ok bool) {
	k := r.keys
	if ids := k.invBySpawn[c.ChatSessionID]; c.ChatSessionID != "" && len(ids) > 0 {
		return soleInvocation(ids), true
	}
	conv := k.conversation(c)
	if k.orchSessions[conv] {
		return r.owner(c.ChatSessionID, 0)
	}
	if ids := k.invBySession[conv]; len(ids) > 0 {
		return soleInvocation(ids), true
	}
	return destUnattributed, true
}

// owner resolves the VS Code chat session a call ran in: the orchestrator session, a spawn_call_id, or the id of
// an execution_subagent tool call whose own session says who ran the tool.
func (r *resolver) owner(chatSession string, depth int) (string, bool) {
	k := r.keys
	if chatSession == "" || k.orchSessions[chatSession] {
		return destOrchestrator, true
	}
	if ids := k.invBySpawn[chatSession]; len(ids) > 0 {
		return soleInvocation(ids), true
	}
	tool := r.byCall[chatSession]
	if tool == nil {
		return "", false
	}
	if tool.ToolName == "execution_subagent" && depth < maxOwnerDepth {
		return r.owner(tool.ChatSessionID, depth+1)
	}
	return destUnattributed, true
}

// soleInvocation returns the invocation stream for a key carried by exactly one invocation; a key shared by
// several is ambiguous and the call goes to the unattributed stream.
func soleInvocation(ids []string) string {
	if len(ids) == 1 {
		return destInvocation + ids[0]
	}
	return destUnattributed
}
