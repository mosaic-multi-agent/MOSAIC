package claudecode_test

// injection_role_test.go: Tests verifying that Injection respects req.Role for
// orchestrator-specific content selection, not req.AgentKey literal matching.
//
// An orchestrator-role agent under a non-"orchestrator" AgentKey (e.g. "orchestrator-script")
// must receive the orchestrator region content exactly as the literal "orchestrator" key
// does; the gate is the role, never the key. The shared HarnessConstraints region is the
// same for every role.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

// TestInjection_OrchestratorRole_NonOrchestratorKey_ServesOrchestratorRegion verifies that
// when an agent has Role == domain.RoleOrchestrator but AgentKey is not "orchestrator"
// (e.g. "orchestrator-script"), the OrchestratorHarnessConstraints region is served and the
// HarnessConstraints region equals what a subagent receives.
func TestInjection_OrchestratorRole_NonOrchestratorKey_ServesOrchestratorRegion(t *testing.T) {
	mod := newModule(t)

	// Baseline: what a genuine subagent receives (shared content only).
	subagentContent, subagentOK := mod.Injection(domain.InjectionRequest{
		Name:     "HarnessConstraints",
		AgentKey: "some-subagent",
	})
	if !subagentOK {
		t.Fatal("baseline: Injection(HarnessConstraints, some-subagent) returned ok=false; " +
			"Claude Code declares HarnessConstraints as an injection it fills (ok=true)")
	}

	// Subject under test: orchestrator-role agent with a non-literal key.
	orchShared, orchSharedOK := mod.Injection(domain.InjectionRequest{
		Name:     "HarnessConstraints",
		AgentKey: "orchestrator-script",
		Role:     domain.RoleOrchestrator,
	})
	orchContent, orchOK := mod.Injection(domain.InjectionRequest{
		Name:     "OrchestratorHarnessConstraints",
		AgentKey: "orchestrator-script", // not "orchestrator"
		Role:     domain.RoleOrchestrator,
	})
	if !orchOK {
		t.Fatal("Injection(OrchestratorHarnessConstraints, orchestrator-script, RoleOrchestrator) returned ok=false; " +
			"orchestrator-role agents must receive the orchestrator region whatever their key")
	}

	// The shared region is identical for the orchestrator role and the subagent.
	if !orchSharedOK || orchShared != subagentContent {
		t.Errorf("orchestrator-role HarnessConstraints = %q (ok=%v), want the same shared content as a subagent %q",
			orchShared, orchSharedOK, subagentContent)
	}
	// The orchestrator region carries the skills-path instruction that distinguishes
	// orchestrator content from shared content for Claude Code.
	if !strings.Contains(orchContent, ".claude/skills/ in the workspace root") {
		t.Errorf("orchestrator-role OrchestratorHarnessConstraints content is missing the skills-path instruction\n"+
			"got:  %q\nwant: content containing \".claude/skills/ in the workspace root\"", orchContent)
	}
	// A subagent gets no orchestrator region.
	if _, ok := mod.Injection(domain.InjectionRequest{Name: "OrchestratorHarnessConstraints", AgentKey: "some-subagent"}); ok {
		t.Error("a subagent received the OrchestratorHarnessConstraints region, want ok=false")
	}
}

// TestInjection_OrchestratorRole_WithOrchestratorKey_StillServesOrchestratorRegion is a
// regression guard: agents with the literal "orchestrator" AgentKey and
// Role == domain.RoleOrchestrator receive the orchestrator region content.
func TestInjection_OrchestratorRole_WithOrchestratorKey_StillServesOrchestratorRegion(t *testing.T) {
	mod := newModule(t)

	content, ok := mod.Injection(domain.InjectionRequest{
		Name:     "OrchestratorHarnessConstraints",
		AgentKey: "orchestrator",
		Role:     domain.RoleOrchestrator,
	})
	if !ok {
		t.Fatal("Injection(OrchestratorHarnessConstraints, orchestrator, RoleOrchestrator) returned ok=false; " +
			"orchestrator must receive its region")
	}
	if !strings.Contains(content, ".claude/skills/ in the workspace root") {
		t.Errorf("Injection(OrchestratorHarnessConstraints, orchestrator, RoleOrchestrator) missing skills-path instruction\n"+
			"got: %q\nwant: content containing \".claude/skills/ in the workspace root\"", content)
	}
}

// TestInjection_ZeroRole_ReturnsSharedContentOnly verifies the backward-compatibility
// contract: when Role is zero-value (empty string), Injection returns shared content
// only, regardless of AgentKey value. This ensures that existing callers that construct
// InjectionRequest without setting Role continue to receive the same content they always
// received (shared only).
//
// After the role-based fix:
//   - AgentKey: "orchestrator", Role: "" -> shared content only (backward compat)
//   - AgentKey: "orchestrator-script", Role: "" -> shared content only (backward compat)
//
// RED: compile-fails until I1.1 adds the Role field to InjectionRequest.
// RUNTIME: passes immediately after I1.2 (role-based switch, zero role = shared only).
func TestInjection_ZeroRole_ReturnsSharedContentOnly(t *testing.T) {
	mod := newModule(t)

	// Even with the literal "orchestrator" AgentKey, zero Role must return shared content.
	content, ok := mod.Injection(domain.InjectionRequest{
		Name:     "HarnessConstraints",
		AgentKey: "orchestrator",
		// Role is zero value (not set)
	})
	if !ok {
		t.Fatal("Injection(HarnessConstraints, orchestrator, zero-Role) returned ok=false; " +
			"Claude Code declares HarnessConstraints (ok=true)")
	}
	// With zero Role, the orchestrator-only content must NOT be merged in.
	if content != claudeHarnessConstraints {
		t.Errorf("Injection(HarnessConstraints, orchestrator, zero-Role) = %q, want %q (shared content only); "+
			"zero Role must not trigger orchestrator-merged content -- Role == domain.RoleOrchestrator is required",
			content, claudeHarnessConstraints)
	}
}
