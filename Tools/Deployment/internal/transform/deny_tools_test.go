package transform_test

// deny_tools_test.go covers the DenyTools option of Request: target-harness names written as
// "<name>: deny" in a permission-shaped tools field after AppendTools. A deny always wins over
// an allow of the same name; list-shaped tools fields ignore the option; nil adds nothing.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
)

// permissionDisposition returns the disposition written for a tool name in a permission
// mapping, and how many pairs carry that name.
func permissionDisposition(doc *docformat.Document, key, name string) (string, int) {
	v, ok := doc.Frontmatter().Get(key)
	if !ok || v.Kind != domain.KindMapping {
		return "", 0
	}
	disposition, count := "", 0
	for _, pair := range v.Pairs {
		if pair.Key == name {
			disposition = pair.Value.Scalar
			count++
		}
	}
	return disposition, count
}

func denyRequest(t *testing.T, yaml string, appendTools, denyTools []string) docformat.Document {
	t.Helper()
	req := appendRequest(t, yaml, appendTools)
	req.DenyTools = denyTools
	doc, _ := applyAndParse(t, req)
	return *doc
}

func TestDenyTools_Permission_AddsNameAsDeny(t *testing.T) {
	doc := denyRequest(t, preservationPermissionDescriptorYAML, nil, []string{"srv.*"})

	if got, n := permissionDisposition(&doc, "tools", "srv.*"); got != "deny" || n != 1 {
		t.Errorf("srv.* must be written once as deny, got %q x%d", got, n)
	}
	if !permissionToolContains(&doc, "tools", "alpha") {
		t.Error("the resolved tool alpha must stay allowed")
	}
}

func TestDenyTools_Permission_DenyWinsOverAnAppendedAllowOfTheSameName(t *testing.T) {
	doc := denyRequest(t, preservationPermissionDescriptorYAML, []string{"native-one"}, []string{"native-one"})

	if got, n := permissionDisposition(&doc, "tools", "native-one"); got != "deny" || n != 1 {
		t.Errorf("native-one must be written once as deny, got %q x%d", got, n)
	}
}

func TestDenyTools_Permission_DenyWinsOverAResolvedAllow(t *testing.T) {
	doc := denyRequest(t, preservationPermissionDescriptorYAML, nil, []string{"alpha"})

	if got, n := permissionDisposition(&doc, "tools", "alpha"); got != "deny" || n != 1 {
		t.Errorf("alpha must be written once as deny, got %q x%d", got, n)
	}
}

func TestDenyTools_Permission_RepeatedNameIsWrittenOnce(t *testing.T) {
	doc := denyRequest(t, preservationPermissionDescriptorYAML, nil, []string{"srv.*", "srv.*"})

	if got, n := permissionDisposition(&doc, "tools", "srv.*"); got != "deny" || n != 1 {
		t.Errorf("srv.* must be written once as deny, got %q x%d", got, n)
	}
}

func TestDenyTools_Nil_AddsNothing(t *testing.T) {
	_, out := applyAndParse(t, appendRequest(t, preservationPermissionDescriptorYAML, nil))
	doc := denyRequest(t, preservationPermissionDescriptorYAML, nil, nil)

	if _, n := permissionDisposition(&doc, "tools", "srv.*"); n != 0 {
		t.Errorf("no deny pair may be added for a nil DenyTools:\n%s", out)
	}
}

func TestDenyTools_List_IsIgnored(t *testing.T) {
	doc := denyRequest(t, preservationListDescriptorYAML, nil, []string{"srv.*", "alpha"})

	got := listFieldScalars(t, &doc, "tools")
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a list-shaped tools field must ignore DenyTools: want %v, got %v", want, got)
	}
	if strings.Contains(strings.Join(got, ","), "srv.*") {
		t.Errorf("a denied name must never be granted in a list: %v", got)
	}
}
