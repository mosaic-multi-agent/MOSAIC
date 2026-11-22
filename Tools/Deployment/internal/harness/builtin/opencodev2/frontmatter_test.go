package opencodev2_test

// Tests for the frontmatter the OpenCode V2 module plans: an explicit mode chosen by the
// agent's role (a missing mode makes a V2 agent primary), the V1 permission key and the
// generic-only keys dropped, and the V2 key order. A final test runs a whole agent through
// the transform and checks the deployed frontmatter.

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/catalog"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

func modeFor(t *testing.T, role domain.AgentRole) string {
	t.Helper()
	plan, err := newModule(t).Frontmatter(domain.FrontmatterRequest{
		Kind: domain.ArtifactAgent, AgentKey: "any-agent", Role: role,
	})
	if err != nil {
		t.Fatalf("Frontmatter(%q): %v", role, err)
	}
	for _, f := range plan.Set {
		if f.Key == "mode" {
			return f.Value.Scalar
		}
	}
	t.Fatalf("no %q field in the planned frontmatter for role %q", "mode", role)
	return ""
}

func TestFrontmatter_ModeIsChosenByRole(t *testing.T) {
	cases := map[domain.AgentRole]string{
		domain.RoleSubagent:     "subagent",
		domain.RoleOrchestrator: "primary",
		domain.RoleUtility:      "primary",
		domain.RoleStandalone:   "primary",
	}
	for role, want := range cases {
		t.Run(string(role), func(t *testing.T) {
			if got := modeFor(t, role); got != want {
				t.Errorf("mode for role %q = %q, want %q", role, got, want)
			}
		})
	}
}

func TestFrontmatter_DropsTheV1PermissionKeyAndTheGenericOnlyKeys(t *testing.T) {
	plan, err := newModule(t).Frontmatter(domain.FrontmatterRequest{
		Kind: domain.ArtifactAgent, AgentKey: "any-agent", Role: domain.RoleSubagent,
	})
	if err != nil {
		t.Fatalf("Frontmatter: %v", err)
	}

	for _, key := range []string{"name", "permission", "tools", "recommended_tier", "tier_rationale",
		"required_skills", "infrastructure", "triggers", "on_failure"} {
		if !containsString(plan.Remove, key) {
			t.Errorf("Remove = %v, want it to drop %q", plan.Remove, key)
		}
	}
	if containsString(plan.Remove, "permissions") {
		t.Errorf("Remove = %v must not drop the V2 %q key", plan.Remove, "permissions")
	}
}

func TestFrontmatter_KeyOrderEndsWithModeModelPermissions(t *testing.T) {
	plan, err := newModule(t).Frontmatter(domain.FrontmatterRequest{
		Kind: domain.ArtifactAgent, AgentKey: "any-agent", Role: domain.RoleSubagent,
	})
	if err != nil {
		t.Fatalf("Frontmatter: %v", err)
	}

	want := []string{"description", "mode", "model", "permissions"}
	n := len(plan.KeyOrder)
	if n < len(want) || !reflect.DeepEqual(plan.KeyOrder[n-len(want):], want) {
		t.Errorf("KeyOrder = %v, want it to end with %v", plan.KeyOrder, want)
	}
}

// deployFixtureAgent runs a generic agent fixture through the transform with the V2 module.
func deployFixtureAgent(t *testing.T, fixture, key string, role domain.AgentRole) *docformat.Document {
	t.Helper()
	frozen := deploymentTestdata(t, "frozen-catalog")
	protocol, err := catalog.FileProtocolLoader{}.LoadProtocol(filepath.Join(frozen, "Catalog"))
	if err != nil {
		t.Fatalf("load protocol: %v", err)
	}
	src, err := os.ReadFile(deploymentTestdata(t, "agent-fixtures", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	res, err := transform.Apply(transform.Request{
		Source: src, Kind: domain.ArtifactAgent, Key: key, Module: newModule(t),
		Model: domain.ModelSelection{ModelID: "github-copilot/claude-sonnet-4.6", Origin: domain.OriginHarnessList},
		Scope: domain.ScopeProject, Role: role, Protocol: protocol,
	})
	if err != nil {
		t.Fatalf("transform.Apply: %v", err)
	}
	doc, err := docformat.Parse(res.Output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	return doc
}

func TestDeployedAgent_HasModeAndV2PermissionsAndNoV1Keys(t *testing.T) {
	cases := []struct {
		fixture, key string
		role         domain.AgentRole
		wantMode     string
	}{
		{"test-runner.md", "test-runner", domain.RoleSubagent, "subagent"},
		{"orchestrator.md", "orchestrator", domain.RoleOrchestrator, "primary"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			fm := deployFixtureAgent(t, tc.fixture, tc.key, tc.role).Frontmatter()

			mode, ok := fm.Get("mode")
			if !ok || mode.Scalar != tc.wantMode {
				t.Errorf("mode = %q (present %v), want %q", mode.Scalar, ok, tc.wantMode)
			}
			if _, ok := fm.Get("permissions"); !ok {
				t.Error("deployed agent has no permissions key")
			}
			for _, gone := range []string{"permission", "name", "tools", "recommended_tier", "tier_rationale", "required_skills"} {
				if _, ok := fm.Get(gone); ok {
					t.Errorf("deployed agent still carries %q", gone)
				}
			}
			assertRelativeOrder(t, fm.Keys(), "description", "mode", "model", "permissions")
		})
	}
}

// assertRelativeOrder fails when the keys that are present are not in the given order.
func assertRelativeOrder(t *testing.T, keys []string, order ...string) {
	t.Helper()
	pos := map[string]int{}
	for i, k := range keys {
		pos[k] = i
	}
	for i := 1; i < len(order); i++ {
		prev, okPrev := pos[order[i-1]]
		cur, okCur := pos[order[i]]
		if okPrev && okCur && prev >= cur {
			t.Errorf("key %q (position %d) must come before %q (position %d); keys: %v", order[i-1], prev, order[i], cur, keys)
		}
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
