package main

// translator_imports_test.go verifies that the import-boundary guard enforces
// the translator layer's purity and downward-only dependency direction.
//
// Background: the translator layer (mosaic-common/agentformat and its sub-packages)
// must be a pure function with no I/O, no clock, and no randomness, and must
// depend only downward -- it must never import a harness package, the application
// layer, transform, the frontends, or any infrastructure package.
//
// This is an imports-only rule and does not require call-level AST inspection.
// The rules are per-directory (sub-packages are not covered by a parent's rule),
// so the guard carries one entry per agentformat directory:
//   - mosaic-common/agentformat
//   - mosaic-common/agentformat/codextoml
//   - mosaic-common/agentformat/formatid (forbidAllModuleImports, like internal/domain)
//   - mosaic-common/agentformat/all      (permits only blank imports of translator sub-packages)
//
// Test strategy: each test passes a small fixture directory (under testdata/) to
// runChecks as its module root and asserts rejection or acceptance outcomes only,
// not the rule table's contents.
//
// The layer may import from mosaic-common only agentfields, docformat, mosaic and
// the agentformat packages; formatid imports nothing from mosaic-common; the
// wiring package imports only agentformat sub-packages. Each restriction has its
// own rejection fixture, and the acceptance fixtures prove the rules do not
// degrade into rejecting every import.

import (
	"path/filepath"
	"testing"
)

// TestTranslatorImports_NonPermittedCommonImport_IsRejected asserts that the
// translator layer importing a mosaic-common package outside its permitted set
// is rejected. The fixture has no I/O and no mosaic-deploy import, so only the
// mosaic-common restriction can reject it.
func TestTranslatorImports_NonPermittedCommonImport_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("nonpermitted_common_in_agentformat"),
		"a non-permitted mosaic-common import in mosaic-common/agentformat should be rejected",
	)
}

// TestTranslatorImports_CommonImportInFormatID_IsRejected asserts that formatid,
// the leaf vocabulary package, is rejected for importing any mosaic-common
// package, even one the translator layer itself may import.
func TestTranslatorImports_CommonImportInFormatID_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("common_import_in_formatid"),
		"a mosaic-common import in mosaic-common/agentformat/formatid should be rejected",
	)
}

// TestTranslatorImports_NonAgentFormatImportInAll_IsRejected asserts that the
// wiring package is rejected for importing a mosaic-common package outside the
// agentformat sub-packages.
func TestTranslatorImports_NonAgentFormatImportInAll_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("non_agentformat_import_in_all"),
		"a non-agentformat import in mosaic-common/agentformat/all should be rejected",
	)
}

// TestTranslatorImports_SubpackageImportsInAll_AreAccepted asserts that the
// wiring package importing only agentformat sub-packages is accepted.
func TestTranslatorImports_SubpackageImportsInAll_AreAccepted(t *testing.T) {
	assertTranslatorAccepts(t,
		translatorFixture("permitted_imports_in_all"),
		"agentformat sub-package imports in mosaic-common/agentformat/all should be accepted",
	)
}

// TestUnderAny_MatchesPackageBoundaries asserts the path matching used by the
// mosaic-common restriction: equal or strictly below a package path, never a
// mere string prefix, and a trailing slash means strictly below.
func TestUnderAny_MatchesPackageBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		imp   string
		paths []string
		want  bool
	}{
		{"equal path", "mosaic-common/agentfields", []string{"mosaic-common/agentfields"}, true},
		{"nested path", "mosaic-common/agentfields/x", []string{"mosaic-common/agentfields"}, true},
		{"string prefix only", "mosaic-common/agentfieldsX", []string{"mosaic-common/agentfields"}, false},
		{"unrelated path", "mosaic-common/harness", []string{"mosaic-common/agentfields"}, false},
		{"trailing slash excludes parent", "mosaic-common/agentformat", []string{"mosaic-common/agentformat/"}, false},
		{"trailing slash includes child", "mosaic-common/agentformat/codextoml", []string{"mosaic-common/agentformat/"}, true},
		{"second entry matches", "mosaic-common/mosaic", []string{"mosaic-common/docformat", "mosaic-common/mosaic"}, true},
		{"empty list", "mosaic-common/mosaic", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := underAny(tc.imp, tc.paths); got != tc.want {
				t.Fatalf("underAny(%q, %v) = %v, want %v", tc.imp, tc.paths, got, tc.want)
			}
		})
	}
}

// TestTranslatorImports_IOImport_IsRejected asserts that a file in
// mosaic-common/agentformat that imports an I/O package ("os") is rejected by the
// guard. The translator layer must be a pure function; an I/O import violates
// that purity and must be caught at the guard rather than at code-review.
func TestTranslatorImports_IOImport_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("io_import_in_agentformat"),
		"I/O import (os) in mosaic-common/agentformat should be rejected by the translator-layer purity rule",
	)
}

// TestTranslatorImports_HarnessImport_IsRejected asserts that a file in
// mosaic-common/agentformat that imports a harness package is rejected by the guard.
// The translator layer must depend only downward; a harness-package import inverts
// the dependency direction and violates the architectural boundary.
//
// Note: the existing harness-isolation check already catches this class of import
// across the whole source tree. This test additionally proves the translator-layer
// direction rule fires for the same scenario once it is added, providing a named
// architectural reason in the violation message.
func TestTranslatorImports_HarnessImport_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("harness_import_in_agentformat"),
		"harness-package import in mosaic-common/agentformat should be rejected by the translator-layer direction rule",
	)
}

// TestTranslatorImports_PermittedImports_IsAccepted asserts that a file in
// mosaic-common/agentformat that uses only the permitted import set is accepted by
// the guard. The permitted mosaic-common set is agentfields, docformat, mosaic
// and the agentformat packages.
//
// agentfields is on the permitted list deliberately: it is the repository's
// authority for MOSAIC-owned and stamp key vocabulary, and forbidding it would
// force the translator into a duplicated key list, which is the failure this
// permission exists to prevent.
func TestTranslatorImports_PermittedImports_IsAccepted(t *testing.T) {
	assertTranslatorAccepts(t,
		translatorFixture("permitted_imports"),
		"permitted imports (mosaic-common/agentfields, mosaic-common/docformat) in mosaic-common/agentformat should be accepted",
	)
}

// TestTranslatorImports_DeployImport_IsRejected asserts that a file in the shared
// translator layer that imports any mosaic-deploy package is rejected: the layer
// lives in mosaic-common and must not depend on Deploy.
func TestTranslatorImports_DeployImport_IsRejected(t *testing.T) {
	assertTranslatorRejects(t,
		translatorFixture("deploy_import_in_agentformat"),
		"a mosaic-deploy import in mosaic-common/agentformat should be rejected: the shared layer must not depend on Deploy",
	)
}

// translatorFixture returns the fixture root of a translator-layer scenario. The
// fixture holds a Common/ directory standing in for the mosaic-common module.
func translatorFixture(name string) string {
	return filepath.Join("testdata", "translator_imports", name)
}

// assertTranslatorRejects checks the fixture with its Common/ directory as the
// mosaic-common root and fails when no violation is reported.
func assertTranslatorRejects(t *testing.T, fixtureRoot, scenario string) {
	t.Helper()
	violations, errs := runChecksIn(fixtureRoot, filepath.Join(fixtureRoot, "Common"))
	if len(errs) > 0 {
		t.Fatalf("unexpected tool errors checking %q: %v", fixtureRoot, errs)
	}
	if len(violations) == 0 {
		t.Fatalf("expected at least one violation for: %s (fixture: %s); got no violations", scenario, fixtureRoot)
	}
}

// assertTranslatorAccepts checks the fixture with its Common/ directory as the
// mosaic-common root and fails when any violation is reported.
func assertTranslatorAccepts(t *testing.T, fixtureRoot, scenario string) {
	t.Helper()
	violations, errs := runChecksIn(fixtureRoot, filepath.Join(fixtureRoot, "Common"))
	if len(errs) > 0 {
		t.Fatalf("unexpected tool errors checking %q: %v", fixtureRoot, errs)
	}
	if len(violations) > 0 {
		t.Fatalf("expected no violations for: %s (fixture: %s); got %d violation(s): %v", scenario, fixtureRoot, len(violations), violations)
	}
}
