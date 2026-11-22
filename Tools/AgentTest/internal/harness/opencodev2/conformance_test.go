package opencodev2_test

// TestConformance drives this adapter through the reusable adapter
// conformance suite (internal/harness/contract.Run), unmodified. If the suite
// ever needs changing to accommodate this adapter, that is evidence the port
// was shaped around one harness and must be raised rather than patched over
// here.
//
// The adapter correlates on the V2 tool call id, which every phase carries, so
// it declares no completion-correlation field and needs no agent-start seams.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/contract"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("conformance: marshal: " + err.Error())
	}
	return b
}

// conformanceNativePre synthesises the pre-invocation payload the plugin
// forwards; the token rides on the tool call id.
func conformanceNativePre(id domain.CollaboratorIdentity, msg domain.TaskMessage, token string) []byte {
	return mustJSON(map[string]any{
		"hook_event_name": "execute.before", "tool": "subagent", "sessionID": "ses_p",
		"agent": "orchestrator", "messageID": "msg_1", "id": token,
		"input": map[string]any{"agent": id.AgentIdentity, "description": msg.TaskDescription, "prompt": msg.Raw},
	})
}

// conformanceNativePost synthesises the post-invocation payload; its result is
// the launch acknowledgement and carries the observed text.
func conformanceNativePost(id domain.CollaboratorIdentity, token string, observed string) []byte {
	return mustJSON(map[string]any{
		"hook_event_name": "execute.after", "tool": "subagent", "sessionID": "ses_p",
		"agent": "orchestrator", "messageID": "msg_1", "id": token,
		"input":  map[string]any{"agent": id.AgentIdentity},
		"status": "completed",
		"result": map[string]any{"output": map[string]any{"sessionID": "ses_c", "status": "running", "output": observed}},
	})
}

// conformanceNativeCompletion synthesises the plugin's completion payload for
// the child's first wake-up, with observed as the child's final text.
func conformanceNativeCompletion(observed string) []byte {
	return mustJSON(map[string]any{
		"hook_event_name": "subagent.completed", "sessionID": "ses_p", "id": "call_conf",
		"agent": "Worker", "childSessionID": "ses_c", "state": "completed",
		"text": "<subagent sessionID=\"ses_c\" state=\"completed\" description=\"d\">\n" + observed + "\n</subagent>",
	})
}

func conformanceObserve(t *testing.T, native []byte) contract.ObservedEffect {
	t.Helper()
	var reply struct {
		Decision     string `json:"decision"`
		UpdatedInput struct {
			Prompt string `json:"prompt"`
		} `json:"updatedInput"`
	}
	if err := json.Unmarshal(native, &reply); err != nil {
		t.Fatalf("conformanceObserve: malformed reply: %v", err)
	}
	switch reply.Decision {
	case "deny":
		return contract.ObservedEffect{Kind: domain.OutcomeHalt}
	case "allow":
		return contract.ObservedEffect{Kind: domain.OutcomeRewritePrompt, RewrittenPrompt: reply.UpdatedInput.Prompt}
	default:
		return contract.ObservedEffect{Kind: domain.OutcomePassthrough}
	}
}

// conformanceCompetingRewriteRequest plants a foreign plugin in the project
// plugins folder before Provision runs: presence alone activates a plugin, so
// a second entry is a second possible rewriter that must be refused.
func conformanceCompetingRewriteRequest(sb domain.Sandbox, subject domain.SubjectUnderTest) domain.ProvisionRequest {
	dir := filepath.Join(sb.SubjectDir, filepath.FromSlash(opencodev2.PluginsRelDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic("conformanceCompetingRewriteRequest: creating plugins dir: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(dir, "someone-elses-plugin.ts"), []byte("// not written by this provisioning\n"), 0o644); err != nil {
		panic("conformanceCompetingRewriteRequest: writing foreign plugin: " + err.Error())
	}
	return domain.ProvisionRequest{
		Sandbox:         sb,
		Subject:         subject,
		InterceptorPath: "interceptor",
		InterceptorArgs: []string{"intercept"},
	}
}

func newConformanceConfig() contract.Config {
	return contract.Config{
		Name: "opencode-v2",
		New: func(t *testing.T, dir string) domain.HarnessAdapter {
			return opencodev2.New(opencodev2.Options{ScopeProbe: fixtureScopeProbe{}})
		},
		NativePre:               conformanceNativePre,
		NativePost:              conformanceNativePost,
		NativeCompletion:        conformanceNativeCompletion,
		Observe:                 conformanceObserve,
		CompetingRewriteRequest: conformanceCompetingRewriteRequest,
		Subject: domain.SubjectUnderTest{
			Identity:       "orchestrator",
			DefinitionPath: "agents/orchestrator.md",
			OpeningMessage: "begin the run",
			Model:          "test/model",
		},
	}
}

func TestConformance(t *testing.T) {
	contract.Run(t, newConformanceConfig())
}
