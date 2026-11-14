package domain_test

// The finding kind catalogue: every kind declared in the package is listed, every
// kind has a one-line plain-language explanation, and a kind with no explanation
// falls back to its raw name so a new kind never breaks a view.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// declaredKinds lists every finding kind of the domain package by constant.
func declaredKinds() []domain.FindingKind {
	return []domain.FindingKind{
		domain.FindingMalformedLine, domain.FindingTruncatedLine, domain.FindingUnreadableFile,
		domain.FindingUnknownSchema, domain.FindingUnrecognisedHarness, domain.FindingMissingTokenUsage,
		domain.FindingMissingModel, domain.FindingUnidentifiedEvent, domain.FindingUnrecognisedFolder,
		domain.FindingMissingEventFile, domain.FindingInvalidPricingEntry, domain.FindingInvalidSettingsEntry,
		domain.FindingCrossStreamRecord, domain.FindingInvalidCacheTierSplit, domain.FindingCompactionMismatch,
		domain.FindingInvalidDigest, domain.FindingUnsupportedDigestVersion, domain.FindingDuplicateRun,
		domain.FindingTranscriptFormatChanged, domain.FindingPricingCurrencyMismatch,
		domain.FindingInstanceTypeMismatch, domain.FindingUnresolvedInvocation,
		domain.FindingOverlappingInvocationStart, domain.FindingDispatchWithoutInvocation,
		domain.FindingTranscriptCaptureGap, domain.FindingInstancePromptMismatch,
	}
}

func TestExplanation_EveryDeclaredKindHasAOneLineExplanationOtherThanItsName(t *testing.T) {
	for _, k := range declaredKinds() {
		t.Run(string(k), func(t *testing.T) {
			got := k.Explanation()

			if got == "" || got == string(k) {
				t.Errorf("Explanation() = %q, want a plain-language text different from the raw name", got)
			}
			if strings.Contains(got, "\n") {
				t.Errorf("Explanation() = %q, want one line", got)
			}
		})
	}
}

func TestExplanation_CrossStreamRecordUsesTheSpecifiedWording(t *testing.T) {
	want := "the same usage record appeared in two actors' logs; counted once, for the first actor read"

	if got := domain.FindingCrossStreamRecord.Explanation(); got != want {
		t.Errorf("Explanation() = %q, want %q", got, want)
	}
}

func TestExplanation_UnknownKindFallsBackToItsRawName(t *testing.T) {
	k := domain.FindingKind("some_future_kind")

	if got := k.Explanation(); got != "some_future_kind" {
		t.Errorf("Explanation() = %q, want the raw name", got)
	}
}

func TestKnownFindingKinds_ListsEveryKindDeclaredInThePackageAndNothingElse(t *testing.T) {
	declared := declaredKindValuesInSource(t)

	known := map[string]bool{}
	for _, k := range domain.KnownFindingKinds() {
		known[string(k)] = true
	}

	for v := range declared {
		if !known[v] {
			t.Errorf("kind %q is declared in the domain package but missing from KnownFindingKinds()", v)
		}
	}
	for v := range known {
		if !declared[v] {
			t.Errorf("KnownFindingKinds() lists %q, which no constant of the domain package declares", v)
		}
	}
	if n := len(domain.KnownFindingKinds()); n != len(known) {
		t.Errorf("KnownFindingKinds() has %d entries for %d distinct kinds", n, len(known))
	}
}

func TestKnownFindingKinds_IsSortedByName(t *testing.T) {
	kinds := domain.KnownFindingKinds()

	if len(kinds) == 0 {
		t.Fatal("KnownFindingKinds() is empty")
	}
	if !sort.SliceIsSorted(kinds, func(i, j int) bool { return kinds[i] < kinds[j] }) {
		t.Errorf("KnownFindingKinds() = %v, want sorted by name", kinds)
	}
}

func TestKnownFindingKinds_EveryEntryHasAnExplanation(t *testing.T) {
	for _, k := range domain.KnownFindingKinds() {
		if k.Explanation() == string(k) {
			t.Errorf("kind %q is listed but has no explanation", k)
		}
	}
}

// declaredKindValuesInSource parses the non-test files of the package and returns
// the value of every constant declared with type FindingKind.
func declaredKindValuesInSource(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse domain package: %v", err)
	}
	values := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			collectKindConstants(file, values)
		}
	}
	if len(values) == 0 {
		t.Fatal("found no FindingKind constants in the domain package source")
	}
	return values
}

func collectKindConstants(file *ast.File, into map[string]bool) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "FindingKind" {
				continue
			}
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					into[strings.Trim(lit.Value, `"`)] = true
				}
			}
		}
	}
}
