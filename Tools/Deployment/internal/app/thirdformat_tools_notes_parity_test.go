package app_test

// thirdformat_tools_notes_parity_test.go specifies that the tools handling of an update is
// identical for the test-only agent format and for Markdown, over the twin harnesses whose tools
// field is permission-shaped: the plan notes for built-in tools the update adds or removes, the
// resulting tools field in decoded form, and the tools the user added to the deployed field,
// which stay while built-in tools follow the catalog. The notes are asserted positively on both
// twins, so a deployed field that one format fails to read cannot hide as a missing note.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
)

const (
	twinToolsKey   = "permission"
	twinAllow      = "allow"
	twinDeny       = "deny"
	twinCustomTool = "my_tool"
	twinDeniedSrv  = "srv_*"
)

// twinSetPermission returns an edit that sets one entry of the permission mapping of canonical
// agent bytes to the disposition, adding the entry at the end of the mapping when absent.
func twinSetPermission(t *testing.T, name, disposition string) func([]byte) []byte {
	return func(canonical []byte) []byte {
		lines := strings.Split(string(canonical), "\n")
		head := -1
		for i, l := range lines {
			if l == twinToolsKey+":" {
				head = i
				break
			}
		}
		if head < 0 {
			t.Fatalf("the deployed agent has no %s mapping:\n%s", twinToolsKey, canonical)
		}
		end := head + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
			if strings.HasPrefix(strings.TrimSpace(lines[end]), name+":") {
				lines[end] = "  " + name + ": " + disposition
				return []byte(strings.Join(lines, "\n"))
			}
			end++
		}
		entry := "  " + name + ": " + disposition
		lines = append(lines[:end], append([]string{entry}, lines[end:]...)...)
		return []byte(strings.Join(lines, "\n"))
	}
}

// twinReasonWith returns the reason of the item that starts with prefix, "" when none does.
func twinReasonWith(item twinPlanItem, prefix string) string {
	for _, r := range item.Reasons {
		if strings.HasPrefix(r, prefix) {
			return r
		}
	}
	return ""
}

// twinToolsOf returns the decoded permission field of the deployed twin agent of an observation.
func twinToolsOf(t *testing.T, obs twinObservation, key string) string {
	t.Helper()
	a, ok := obs.Agents[key]
	if !ok {
		t.Fatalf("agent %s is not deployed after the step", key)
	}
	return a.Frontmatter[twinToolsKey]
}

// twinRequireToolsEntry fails unless the decoded tools field holds the entry name:disposition.
func twinRequireToolsEntry(t *testing.T, label, tools, name, disposition string) {
	t.Helper()
	if want := name + ":" + disposition; !strings.Contains(tools, want) {
		t.Errorf("%s: tools field %q lacks the entry %q", label, tools, want)
	}
}

// twinStaleToolsUpdate ages the deployed runner, applies the tools edits to it on both sides and
// runs Update.
func twinStaleToolsUpdate(t *testing.T, edits ...func([]byte) []byte) (twinObservation, twinObservation) {
	t.Helper()
	p := twinDeployed(t)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinChain(append(edits, twinAgeVersion())...))
	}
	return runTwinStep(t, p, twinUpdateStep)
}

func TestTwinParity_UpdateToolsNotes_CatalogAddsBuiltInTool_PlanNamesIt(t *testing.T) {
	// Arrange: the deployed runner lost the edit tool the catalog still grants
	// Act
	test, md := twinStaleToolsUpdate(t, twinSetPermission(t, "edit", twinDeny))

	// Assert
	assertTwinParity(t, "update that adds a built-in tool", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		item := twinItem(t, obs, owRunner)
		if item.Action != domain.ActionUpdate {
			t.Errorf("%s: plan action = %s; want %s", label, item.Action, domain.ActionUpdate)
		}
		if got := twinReasonWith(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
			t.Errorf("%s: added note = %q; want %q; reasons %q", label, got, reasonAddedPrefix+"edit", item.Reasons)
		}
		if got := twinReasonWith(item, reasonRemovedPrefix); got != "" {
			t.Errorf("%s: unexpected removed note %q", label, got)
		}
		twinRequireToolsEntry(t, label, twinToolsOf(t, obs, owRunner), "edit", twinAllow)
	}
}

func TestTwinParity_UpdateToolsNotes_CatalogRemovesBuiltInTool_PlanNamesIt(t *testing.T) {
	// Arrange: the deployed runner grants patch, which the catalog does not
	// Act
	test, md := twinStaleToolsUpdate(t, twinSetPermission(t, "patch", twinAllow))

	// Assert
	assertTwinParity(t, "update that removes a built-in tool", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		item := twinItem(t, obs, owRunner)
		if got := twinReasonWith(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"patch" {
			t.Errorf("%s: removed note = %q; want %q; reasons %q", label, got, reasonRemovedPrefix+"patch", item.Reasons)
		}
		if got := twinReasonWith(item, reasonAddedPrefix); got != "" {
			t.Errorf("%s: unexpected added note %q", label, got)
		}
		twinRequireToolsEntry(t, label, twinToolsOf(t, obs, owRunner), "patch", twinDeny)
	}
}

func TestTwinParity_UpdateToolsNotes_AddAndRemoveTogether_BothNotesPresent(t *testing.T) {
	// Act
	test, md := twinStaleToolsUpdate(t, twinSetPermission(t, "edit", twinDeny), twinSetPermission(t, "patch", twinAllow))

	// Assert
	assertTwinParity(t, "update that adds and removes built-in tools", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		item := twinItem(t, obs, owRunner)
		if got := twinReasonWith(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
			t.Errorf("%s: added note = %q; reasons %q", label, got, item.Reasons)
		}
		if got := twinReasonWith(item, reasonRemovedPrefix); got != reasonRemovedPrefix+"patch" {
			t.Errorf("%s: removed note = %q; reasons %q", label, got, item.Reasons)
		}
	}
}

func TestTwinParity_UpdateToolsNotes_NoBuiltInChange_PlanHasNoToolsNotes(t *testing.T) {
	// Act: the runner is only stale; its tools field matches the catalog
	test, md := twinStaleToolsUpdate(t)

	// Assert
	assertTwinParity(t, "update without a tools change", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		item := twinItem(t, obs, owRunner)
		if item.Action != domain.ActionUpdate {
			t.Errorf("%s: plan action = %s; want %s", label, item.Action, domain.ActionUpdate)
		}
		for _, prefix := range []string{reasonAddedPrefix, reasonRemovedPrefix} {
			if got := twinReasonWith(item, prefix); got != "" {
				t.Errorf("%s: unexpected tools note %q", label, got)
			}
		}
	}
}

func TestTwinParity_UpdateTools_UserAddedToolsSurvive_BuiltInToolsFollowCatalog(t *testing.T) {
	// Arrange: the user added a custom tool and a denied server pattern; a built-in tool drifted
	// Act
	test, md := twinStaleToolsUpdate(t,
		twinSetPermission(t, twinCustomTool, twinAllow),
		twinSetPermission(t, twinDeniedSrv, twinDeny),
		twinSetPermission(t, "edit", twinDeny))

	// Assert
	assertTwinParity(t, "update over user-added tools", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		tools := twinToolsOf(t, obs, owRunner)
		twinRequireToolsEntry(t, label, tools, twinCustomTool, twinAllow)
		twinRequireToolsEntry(t, label, tools, twinDeniedSrv, twinDeny)
		twinRequireToolsEntry(t, label, tools, "edit", twinAllow)
		item := twinItem(t, obs, owRunner)
		for _, r := range item.Reasons {
			if strings.Contains(r, twinCustomTool) || strings.Contains(r, twinDeniedSrv) {
				t.Errorf("%s: note %q names a user-added tool; only built-in changes are reported", label, r)
			}
		}
		if got := twinReasonWith(item, reasonAddedPrefix); got != reasonAddedPrefix+"edit" {
			t.Errorf("%s: added note = %q; want %q", label, got, reasonAddedPrefix+"edit")
		}
	}
}

func TestTwinParity_UpdateTools_UserAddedToolsSurvive_WithoutBuiltInChange(t *testing.T) {
	// Act
	test, md := twinStaleToolsUpdate(t, twinSetPermission(t, twinCustomTool, twinAllow))

	// Assert
	assertTwinParity(t, "update over a user-added tool only", test, md)
	for label, obs := range map[string]twinObservation{"test format": test, "markdown": md} {
		twinRequireToolsEntry(t, label, twinToolsOf(t, obs, owRunner), twinCustomTool, twinAllow)
	}
}
