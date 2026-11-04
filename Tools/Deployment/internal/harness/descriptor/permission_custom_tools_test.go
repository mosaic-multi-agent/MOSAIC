package descriptor_test

// permission_custom_tools_test.go verifies that a permission-shape descriptor writes every
// custom tool into the permission mapping as "<name>: allow", after all universe pairs, and
// that output without custom tools is unchanged.

import (
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// pairKeys returns the keys of pairs in order.
func pairKeys(pairs []domain.FieldPair) []string {
	keys := make([]string, len(pairs))
	for i, p := range pairs {
		keys[i] = p.Key
	}
	return keys
}

// mapPermission runs MapTools on the plain permission descriptor and returns its pairs.
func mapPermission(t *testing.T, generic []string, custom map[string]string) []domain.FieldPair {
	t.Helper()
	result, err := descriptor.MapTools(makePermNoCustomDestDescriptor(), domain.ToolRequest{
		AgentKey:    "test-agent",
		Generic:     generic,
		CustomNames: custom,
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	return permPairs(t, result)
}

// wantPair asserts pairs[idx] has the given key and the value "allow" or "deny".
func wantPair(t *testing.T, pairs []domain.FieldPair, idx int, key, value string) {
	t.Helper()
	if idx >= len(pairs) {
		t.Fatalf("pair %d (%q) missing; got keys %v", idx, key, pairKeys(pairs))
	}
	if pairs[idx].Key != key || pairs[idx].Value.Scalar != value {
		t.Errorf("pair %d: want %s: %s, got %s: %s (all keys %v)",
			idx, key, value, pairs[idx].Key, pairs[idx].Value.Scalar, pairKeys(pairs))
	}
}

func TestPermissionCustomTools_CustomServerAppendedAsAllowAfterUniversePairs(t *testing.T) {
	pairs := mapPermission(t, []string{"file_read", "terminal"}, map[string]string{"terminal": "my-server"})

	if len(pairs) != 3 {
		t.Fatalf("want 3 pairs (2 universe + 1 custom), got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 0, "read", "allow")
	wantPair(t, pairs, 1, "write", "deny")
	wantPair(t, pairs, 2, "my-server", "allow")
}

func TestPermissionCustomTools_MultipleCustomServersKeepResolutionOrder(t *testing.T) {
	pairs := mapPermission(t, []string{"terminal", "web_search"},
		map[string]string{"terminal": "zeta-server", "web_search": "alpha-server"})

	if len(pairs) != 4 {
		t.Fatalf("want 4 pairs, got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 2, "zeta-server", "allow")
	wantPair(t, pairs, 3, "alpha-server", "allow")
}

func TestPermissionCustomTools_SameServerForTwoGenericsAppearsOnce(t *testing.T) {
	pairs := mapPermission(t, []string{"terminal", "web_search"},
		map[string]string{"terminal": "shared", "web_search": "shared"})

	if len(pairs) != 3 {
		t.Fatalf("want 3 pairs (shared server once), got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 2, "shared", "allow")
}

func TestPermissionCustomTools_NameEqualToUniverseToolSetsThatPairToAllowWithoutDuplicate(t *testing.T) {
	// "write" is a universe tool left unused (deny); a custom server named "write" must
	// turn that pair to allow and must not add a second pair.
	pairs := mapPermission(t, []string{"file_read", "terminal"}, map[string]string{"terminal": "write"})

	if len(pairs) != 2 {
		t.Fatalf("want 2 pairs (no duplicate), got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 0, "read", "allow")
	wantPair(t, pairs, 1, "write", "allow")
}

func TestPermissionCustomTools_NoCustomToolsLeavesPermissionMapUnchanged(t *testing.T) {
	pairs := mapPermission(t, []string{"file_read"}, nil)

	if len(pairs) != 2 {
		t.Fatalf("want 2 universe pairs, got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 0, "read", "allow")
	wantPair(t, pairs, 1, "write", "deny")
}

func TestPermissionCustomTools_CustomToolTemplateIsAppliedToPermissionKey(t *testing.T) {
	d := makePermNoCustomDestDescriptor()
	d.Tools.CustomToolTemplate = "mcp__%s__*"

	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:    "test-agent",
		Generic:     []string{"terminal"},
		CustomNames: map[string]string{"terminal": "srv"},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	pairs := permPairs(t, result)

	if len(pairs) != 3 {
		t.Fatalf("want 3 pairs, got %d: %v", len(pairs), pairKeys(pairs))
	}
	wantPair(t, pairs, 2, "mcp__srv__*", "allow")
}

func TestPermissionCustomTools_FieldDestinedCustomToolStaysOutOfPermissionMap(t *testing.T) {
	// A custom tool routed to a separate field by CustomToolDestinations has no
	// main-tools destination, so it must not be added to the permission mapping.
	d := makePermCustomDestDescriptor()
	result, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:    "test-agent",
		Generic:     []string{"terminal"},
		CustomNames: map[string]string{"terminal": "my-server"},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	pairs := permPairs(t, result)

	if len(pairs) != 2 {
		t.Errorf("want 2 universe pairs only, got %d: %v", len(pairs), pairKeys(pairs))
	}
}
