package descriptor_test

// tool_grants_test.go covers ExtractToolGrants: the names of a deployed tools field with their
// dispositions. A permission-shaped field yields allow and deny pairs in document order; a
// list-shaped field yields every item as allow; each name appears once, first occurrence wins.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

func loadBuiltin(t *testing.T, rel string) *domain.HarnessDescriptor {
	t.Helper()
	d, err := descriptor.Load("../builtin/" + rel)
	if err != nil {
		t.Fatalf("load %s: %v", rel, err)
	}
	return d
}

func permissionValue(pairs ...[2]string) domain.FieldValue {
	var fp []domain.FieldPair
	for _, p := range pairs {
		fp = append(fp, domain.FieldPair{Key: p[0], Value: domain.ScalarValue(p[1], domain.QuotePlain)})
	}
	return domain.MappingValue(fp)
}

func TestExtractToolGrants_PermissionShapeReturnsAllowAndDenyInDocumentOrder(t *testing.T) {
	d := loadBuiltin(t, "opencode/opencode.yaml")
	value := permissionValue(
		[2]string{"read", "allow"}, [2]string{"bash", "deny"}, [2]string{"srv_*", "allow"}, [2]string{"other_tool", "deny"},
	)

	got := descriptor.ExtractToolGrants(d, value)

	want := []domain.ToolGrant{
		{Name: "read", Disposition: domain.Allow},
		{Name: "bash", Disposition: domain.Deny},
		{Name: "srv_*", Disposition: domain.Allow},
		{Name: "other_tool", Disposition: domain.Deny},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grants: want %+v, got %+v", want, got)
	}
}

func TestExtractToolGrants_FirstOccurrenceOfANameWins(t *testing.T) {
	d := loadBuiltin(t, "opencode/opencode.yaml")
	value := permissionValue([2]string{"srv_*", "deny"}, [2]string{"srv_*", "allow"})

	got := descriptor.ExtractToolGrants(d, value)

	want := []domain.ToolGrant{{Name: "srv_*", Disposition: domain.Deny}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grants: want %+v, got %+v", want, got)
	}
}

func TestExtractToolGrants_ListShapeReturnsEveryItemAsAllow(t *testing.T) {
	d := loadBuiltin(t, "ghcpcli/ghcp-cli.yaml")
	value := domain.ListValue([]domain.FieldValue{
		domain.ScalarValue("read", domain.QuotePlain),
		domain.ScalarValue("srv/*", domain.QuotePlain),
		domain.ScalarValue("read", domain.QuotePlain),
	}, domain.ListBlock)

	got := descriptor.ExtractToolGrants(d, value)

	want := []domain.ToolGrant{
		{Name: "read", Disposition: domain.Allow},
		{Name: "srv/*", Disposition: domain.Allow},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grants: want %+v, got %+v", want, got)
	}
}

func TestExtractToolGrants_ZeroValueYieldsNoGrants(t *testing.T) {
	d := loadBuiltin(t, "opencode/opencode.yaml")

	if got := descriptor.ExtractToolGrants(d, domain.FieldValue{}); len(got) != 0 {
		t.Errorf("grants: want none, got %+v", got)
	}
}
