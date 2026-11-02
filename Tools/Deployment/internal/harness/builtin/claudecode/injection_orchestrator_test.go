package claudecode_test

// injection_orchestrator_test.go: Tests for the new Injection(InjectionRequest) contract
// introduced in Stage 4 (orchestrator-only injection mechanism). These tests define the
// TDD RED specification for agent-identity-aware injection resolution.
//
// Test coverage (per plan task T4.1):
//
//  (a) Subagent key returns shared content only — same as pre-Stage-4 behavior.
//  (b) Orchestrator role: HarnessConstraints returns shared content only and
//      OrchestratorHarnessConstraints returns orchestrator-only content; no merging.
//  (c) Orchestrator key with no orchestrator-only content for a given name (both sources
//      declared-but-empty) returns ok=true with an empty string.
//  (d) Non-canonical injection names return ok=false regardless of agent key.
//  (e) Determinism — two calls with equal InjectionRequest return equal output.
//  (f) Both sources non-empty: each region name returns exactly its own source's content.
//  (g) Only shared content present for a name (orchestrator-only content absent): orchestrator
//      receives shared content unmodified, with no trailing separator (Case 4).
//
// RED STATE:
//   - Case (b): FAILS until I4.2 (create HarnessInjectionsOrchestrator.md) and I4.3
//     (update claudecode.Injection to load the orchestrator file and merge content) are done.
//   - Case (f): FAILS until I4.3 adds orchInjections to module and updates NewWithMapsForTesting.
//   - Cases (a), (c), (d), (e), (g): PASS in RED phase (existing behavior satisfies these).

import (
	"sort"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/claudecode"
)

// TestInjection_SubagentKey_ReturnsSharedContentOnly verifies case (a): when AgentKey is
// not "orchestrator", Injection returns only the shared (subagent-level) content.
// For Claude Code HarnessConstraints, the shared content is the output-artifacts constraint.
// This test passes in the RED phase: existing behavior is preserved for non-orchestrator keys.
func TestInjection_SubagentKey_ReturnsSharedContentOnly(t *testing.T) {
	mod := newModule(t)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "some-subagent"})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, some-subagent) returned ok=false; Claude Code declares this injection (ok=true)")
	}
	// Subagent must receive only shared content, with no orchestrator-only content merged in.
	if content != claudeHarnessConstraints {
		t.Errorf("subagent should receive shared content only\ngot:  %q\nwant: %q", content, claudeHarnessConstraints)
	}
}

// TestInjection_SubagentKey_EmptyAgentKey_ReturnsSharedContentOnly verifies that an empty
// AgentKey (equivalent to a non-orchestrator key) also returns only shared content.
func TestInjection_SubagentKey_EmptyAgentKey_ReturnsSharedContentOnly(t *testing.T) {
	mod := newModule(t)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: ""})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, \"\") returned ok=false; Claude Code declares this injection (ok=true)")
	}
	if content != claudeHarnessConstraints {
		t.Errorf("empty agent key should receive shared content only\ngot:  %q\nwant: %q", content, claudeHarnessConstraints)
	}
}

// TestInjection_OrchestratorKey_OrchestratorRegionReceivesOrchestratorContentOnly verifies
// case (b): for the orchestrator role, the OrchestratorHarnessConstraints region returns the
// content of HarnessInjectionsOrchestrator.md alone. The skills-path instruction
// (".claude/skills/ in the workspace root") is the distinguishing marker that the
// orchestrator file was loaded.
func TestInjection_OrchestratorKey_OrchestratorRegionReceivesOrchestratorContentOnly(t *testing.T) {
	mod := newModule(t)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "OrchestratorHarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	if !ok {
		t.Fatal("Injection(OrchestratorHarnessConstraints, orchestrator) returned ok=false; the orchestrator must receive its own region")
	}
	// The orchestrator-only content must include the skills-path instruction.
	if !strings.Contains(content, ".claude/skills/ in the workspace root") {
		t.Errorf("OrchestratorHarnessConstraints content is missing the skills-path instruction\ngot:  %q\nwant: content containing \".claude/skills/ in the workspace root\"", content)
	}
	// Separation of the two sources is asserted with known content in
	// TestInjection_OrchestratorKey_BothContentSources_EachRegionReturnsOneSource.
}

// TestInjection_OrchestratorKey_SharedRegionHasNoOrchestratorContent verifies that for the
// orchestrator role the HarnessConstraints region returns the shared content exactly, with
// none of the orchestrator-only content merged in.
func TestInjection_OrchestratorKey_SharedRegionHasNoOrchestratorContent(t *testing.T) {
	mod := newModule(t)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, orchestrator) returned ok=false")
	}
	if content != claudeHarnessConstraints {
		t.Errorf("orchestrator HarnessConstraints = %q, want the shared content only %q", content, claudeHarnessConstraints)
	}
}

// TestInjection_OrchestratorKey_DeclaredButBothEmpty_ReturnsOkTrueEmptyString verifies case (c):
// when an injection name is declared but both the shared content and the orchestrator-only
// content are empty, the orchestrator receives ok=true with an empty string — not ok=false.
//
// This is the "declared-but-empty" scenario (Case 6 of the design merge strategy). It is
// distinct from Case 4 (shared non-empty, orch absent), which is tested separately in
// TestInjection_OrchestratorKey_SharedContentOnly_NoOrchContent_ReturnsSharedUnmodified.
//
// Uses NewWithMapsForTesting to supply a declared-but-empty shared entry directly: real
// Claude Code content no longer has a canonical name that is declared-but-empty in both
// HarnessInjections.md and HarnessInjectionsOrchestrator.md (LanguagePatterns, formerly used
// here, is no longer a tool-managed name and is not declared by this harness at all), so the
// scenario is constructed rather than sourced from the actual harness files.
func TestInjection_OrchestratorKey_DeclaredButBothEmpty_ReturnsOkTrueEmptyString(t *testing.T) {
	mod := claudecode.NewWithMapsForTesting(t,
		map[string]string{"HarnessConstraints": ""},
		nil, // no orchestrator-only content declared
	)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, orchestrator) returned ok=false; declared-but-empty injection must return ok=true")
	}
	// No shared content and no orchestrator-only content → result is empty string.
	if content != "" {
		t.Errorf("orchestrator HarnessConstraints should be empty (no shared or orchestrator-only content declared); got %q", content)
	}
}

// TestInjection_NonCanonicalName_ReturnsFalse_ForAllAgentKeys verifies case (d):
// non-canonical injection names return ok=false regardless of the agent key. This preserves
// the existing contracttest invariant with the new InjectionRequest signature.
func TestInjection_NonCanonicalName_ReturnsFalse_ForAllAgentKeys(t *testing.T) {
	mod := newModule(t)

	nonCanonical := []string{
		"NotAnInjection",
		"",
		"random-garbage-xyz",
		"harness_constraints", // wrong casing — must not match "HarnessConstraints"
	}
	agentKeys := []string{"", "some-subagent", "orchestrator"}

	for _, name := range nonCanonical {
		for _, agentKey := range agentKeys {
			req := domain.InjectionRequest{Name: name, AgentKey: agentKey}
			_, ok := mod.Injection(req)
			if ok {
				t.Errorf("Injection(%q, agentKey=%q) returned ok=true; non-canonical names must return ok=false for all agent keys", name, agentKey)
			}
		}
	}
}

// TestInjection_Deterministic verifies case (e): two calls with equal InjectionRequest
// inputs return equal outputs, for a representative set of advisory injection names and
// all agent keys. Injection names are open (no closed allowlist since Stage 2); the
// advisory InjectionParent map keys serve as the representative set.
func TestInjection_Deterministic(t *testing.T) {
	mod := newModule(t)

	// Collect advisory injection names from InjectionParent in sorted order for
	// deterministic subtest execution.
	var names []string
	for name := range docformat.InjectionParent {
		names = append(names, name)
	}
	sort.Strings(names)

	agentKeys := []string{"", "some-subagent", "orchestrator"}
	for _, name := range names {
		for _, agentKey := range agentKeys {
			req := domain.InjectionRequest{Name: name, AgentKey: agentKey}
			c1, ok1 := mod.Injection(req)
			c2, ok2 := mod.Injection(req)
			if ok1 != ok2 || c1 != c2 {
				t.Errorf("Injection(%q, agentKey=%q) non-deterministic: first=(%q,%v), second=(%q,%v)",
					name, agentKey, c1, ok1, c2, ok2)
			}
		}
	}
}

// TestInjection_OrchestratorKey_BothContentSources_EachRegionReturnsOneSource verifies case (f):
// when both the shared content and the orchestrator-only content are non-empty, the
// HarnessConstraints region returns exactly the shared content and the
// OrchestratorHarnessConstraints region returns exactly the orchestrator content. No returned
// string holds text from both sources.
//
// Uses NewWithMapsForTesting to supply known content so the test does not depend on the actual
// HarnessInjections.md or HarnessInjectionsOrchestrator.md content.
func TestInjection_OrchestratorKey_BothContentSources_EachRegionReturnsOneSource(t *testing.T) {
	sharedContent := "shared constraint content"
	orchContent := "orchestrator-only constraint content"

	mod := claudecode.NewWithMapsForTesting(t,
		map[string]string{"HarnessConstraints": sharedContent},
		map[string]string{"OrchestratorHarnessConstraints": orchContent},
	)

	shared, sharedOK := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	orch, orchOK := mod.Injection(domain.InjectionRequest{Name: "OrchestratorHarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})

	if !sharedOK || shared != sharedContent {
		t.Errorf("Injection(HarnessConstraints, orchestrator) = %q (ok=%v), want exactly the shared content %q (no merge)", shared, sharedOK, sharedContent)
	}
	if !orchOK || orch != orchContent {
		t.Errorf("Injection(OrchestratorHarnessConstraints, orchestrator) = %q (ok=%v), want exactly the orchestrator content %q", orch, orchOK, orchContent)
	}
}

// TestInjection_OrchestratorKey_SharedContentOnly_NoOrchContent_ReturnsSharedUnmodified
// verifies case (g): Case 4 of the design merge strategy — when shared content is non-empty for
// a given injection name but no orchestrator-only content exists for that name, the orchestrator
// receives the shared content unchanged, with no trailing separator appended.
//
// An implementation that unconditionally appends "\n\n" (even when orchestrator content is
// absent) would fail this test.
//
// Uses NewWithMapsForTesting to supply known shared content. No orchestrator content is provided
// for the tested injection name.
//
// This test PASSES in the RED phase: the current stub returns shared content for all agent keys,
// which is the correct Case 4 behavior. It becomes a regression guard once I4.3 is implemented.
func TestInjection_OrchestratorKey_SharedContentOnly_NoOrchContent_ReturnsSharedUnmodified(t *testing.T) {
	sharedContent := "shared constraint content"

	mod := claudecode.NewWithMapsForTesting(t,
		map[string]string{"HarnessConstraints": sharedContent},
		nil, // no orchestrator-only content for HarnessConstraints
	)

	content, ok := mod.Injection(domain.InjectionRequest{Name: "HarnessConstraints", AgentKey: "orchestrator", Role: domain.RoleOrchestrator})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, orchestrator) returned ok=false; must return ok=true when shared content exists")
	}
	if content != sharedContent {
		t.Errorf("Injection(HarnessConstraints, orchestrator) modified shared content when no orchestrator-only content exists\ngot:  %q\nwant: %q\n\nWhen only shared content is present, the orchestrator must receive it unmodified (no trailing separator).", content, sharedContent)
	}
}
