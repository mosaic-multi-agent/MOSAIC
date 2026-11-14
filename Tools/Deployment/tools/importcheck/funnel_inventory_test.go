package main

// funnel_inventory_test.go verifies the read-funnel inventory rule: the files in
// internal/app that reference a read-funnel identifier (readDeployedArtifact,
// decodeDeployedBytes, funnelReader, canonicalReader) must be exactly the files listed in
// the FUNNEL-ROUTED READS section of deployed_read.go. Each fixture under
// testdata/funnel_inventory/ holds one scenario and is checked through runChecks.

import (
	"path/filepath"
	"strings"
	"testing"
)

func funnelFixture(name string) string {
	return filepath.Join("testdata", "funnel_inventory", name)
}

// assertRejectedMentioning requires the fixture to be rejected with violations that name
// every given text (a file name), so a rejection for an unrelated reason does not pass.
func assertRejectedMentioning(t *testing.T, fixture string, wants ...string) {
	t.Helper()
	violations, errs := runChecks(funnelFixture(fixture))
	if len(errs) > 0 {
		t.Fatalf("unexpected tool errors: %v", errs)
	}
	joined := strings.Join(violations, "\n")
	if joined == "" {
		t.Fatalf("fixture %s: expected a violation, got none", fixture)
	}
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Errorf("fixture %s: violations do not mention %q:\n%s", fixture, want, joined)
		}
	}
}

func TestFunnelInventory_MultiEntrySection_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("multi_entry_caller"),
		"several listed callers (full, bare and colon-only entries) with a dash line after the section end")
}

func TestFunnelInventory_CallerListedOnlyAfterSectionEnd_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "listed_after_section_end", "reader.go")
}

func TestFunnelInventory_InventoryFileReferencingFunnel_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("inventory_file_references_funnel"),
		"the inventory file itself references funnel identifiers and is not listed")
}

func TestFunnelInventory_TestFileCaller_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("test_file_caller"),
		"an unlisted _test.go file that references a funnel identifier")
}

func TestFunnelInventory_NoInventoryFile_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("no_inventory_file"),
		"a package without deployed_read.go leaves the rule inactive")
}

func TestFunnelInventory_ListedCaller_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("listed_caller"),
		"a file that references a funnel identifier and is listed in the inventory")
}

func TestFunnelInventory_UnlistedCaller_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "unlisted_caller", "reader.go")
}

func TestFunnelInventory_UnlistedCaller_ViolationNamesFileAndIdentifier(t *testing.T) {
	violations, errs := runChecks(funnelFixture("unlisted_caller"))
	if len(errs) > 0 {
		t.Fatalf("unexpected tool errors: %v", errs)
	}
	joined := strings.Join(violations, "\n")
	for _, want := range []string{"reader.go", "readDeployedArtifact", "deployed_read.go"} {
		if !strings.Contains(joined, want) {
			t.Errorf("violations do not mention %q:\n%s", want, joined)
		}
	}
}

func TestFunnelInventory_ListedFileWithoutReference_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "stale_entry", "reader.go")
}

func TestFunnelInventory_ListedFileMissing_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "listed_missing_file", "reader.go")
}

func TestFunnelInventory_MissingSection_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "missing_section", "deployed_read.go")
}

func TestFunnelInventory_CommentOnlyMention_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("comment_only_mention"),
		"an unlisted file that names a funnel identifier only in comments")
}

func TestFunnelInventory_DeclarationOnly_IsAccepted(t *testing.T) {
	assertGuardAcceptsFixture(t, funnelFixture("declaration_only"),
		"an unlisted file that only declares a function or method with a funnel name")
}

func TestFunnelInventory_MethodValueReference_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "method_value_caller", "method_value.go", "canonicalReader")
}

func TestFunnelInventory_InventoryFileListingItself_IsRejected(t *testing.T) {
	assertRejectedMentioning(t, "self_listed", "deployed_read.go")
}
