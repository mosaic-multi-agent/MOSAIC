package app

// promote_regions_test.go covers how buildGenericAgent treats project and managed regions of
// the source under the keep and empty choices, and holds the helpers shared with the other
// promote region tests.
//
// Verified behaviours:
//   - Keep (the default): every project region is present and byte-identical to the source,
//     at any nesting depth, including inside managed regions and a version attribute on the
//     open tag.
//   - Empty (EmptyProjectRegions): every project region is present and holds nothing but
//     its nested project regions; a region with none is an empty tag pair.
//   - Managed regions are emptied but keep their nested project regions as direct children.

import (
	"bytes"
	"sort"
	"testing"

	"mosaic-common/docformat"
)

// regionsSource wraps body in eligible harness-only frontmatter.
func regionsSource(body string) []byte {
	return []byte("---\n" +
		"transform_version: \"2.1.0\"\n" +
		"version: \"1.0.0\"\n" +
		"id: \"10\"\n" +
		"role: subagent\n" +
		"---\n" + body)
}

// generateRegionsBody runs buildGenericAgent and returns only the body (after frontmatter).
func generateRegionsBody(t *testing.T, body string, empty bool) string {
	t.Helper()
	in := standardPromoteInput(regionsSource(body))
	in.EmptyProjectRegions = empty
	out, err := buildGenericAgent(in)
	if err != nil {
		t.Fatalf("buildGenericAgent failed: %v", err)
	}
	return bodyAfterFrontmatter(t, out)
}

// bodyAfterFrontmatter returns the bytes following the closing frontmatter delimiter.
func bodyAfterFrontmatter(t *testing.T, doc []byte) string {
	t.Helper()
	const delim = "\n---\n"
	idx := bytes.Index(doc[3:], []byte(delim))
	if idx < 0 {
		t.Fatalf("no closing frontmatter delimiter in %q", doc)
	}
	return string(doc[3+idx+len(delim):])
}

// assertRegionsBody compares the generated body with want.
func assertRegionsBody(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("generated body mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func projectNames(t *testing.T, doc []byte) []string {
	t.Helper()
	parsed, err := docformat.Parse(doc)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	var names []string
	for _, n := range parsed.Body().Injections() {
		names = append(names, n.Name())
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Keep
// ---------------------------------------------------------------------------

func TestBuildGenericAgent_Keep_TopLevelProjectRegionIsByteIdentical(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"Prose.\n" +
		"<Alpha type=\"project\">\n" +
		"  indented alpha line\n" +
		"\n" +
		"last alpha line\n" +
		"</Alpha>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, body)
}

func TestBuildGenericAgent_Keep_ProjectOpenTagVersionAttributeIsKept(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Alpha type=\"project\" version=\"2.0.0\">\n" +
		"alpha content\n" +
		"</Alpha>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, body)
}

func TestBuildGenericAgent_Keep_ProjectRegionInsideManagedIsKeptAndManagedEmptied(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"managed prose that must go\n" +
		"<Proj type=\"project\">\n" +
		"proj content\n" +
		"</Proj>\n" +
		"more managed prose that must go\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Proj type=\"project\">\n" +
		"proj content\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_Keep_DeeperNestingInsideManagedKeepsOutermostWithInner(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Outer type=\"project\">\n" +
		"outer text\n" +
		"<Inner type=\"project\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"<Second type=\"project\">\n" +
		"second text\n" +
		"</Second>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Outer type=\"project\">\n" +
		"outer text\n" +
		"<Inner type=\"project\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"<Second type=\"project\">\n" +
		"second text\n" +
		"</Second>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, want)
}

// ---------------------------------------------------------------------------
// Empty
// ---------------------------------------------------------------------------

// TestBuildGenericAgent_Empty_TopLevelProjectRegionIsEmptyPair is regression coverage of the
// emptying that exists today (the version attribute is dropped as before).
func TestBuildGenericAgent_Empty_TopLevelProjectRegionIsEmptyPair(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"Prose.\n" +
		"<Alpha type=\"project\" version=\"2.0.0\">\n" +
		"alpha content\n" +
		"</Alpha>\n" +
		"</Identity>\n"
	want := "<Identity type=\"core\">\n" +
		"Prose.\n" +
		"<Alpha type=\"project\">\n" +
		"</Alpha>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_Empty_ProjectRegionInsideManagedStaysAsEmptyPair(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"managed prose\n" +
		"<Proj type=\"project\">\n" +
		"proj content\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Proj type=\"project\">\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_Empty_NestedProjectRegionSurvivesInsideEmptiedParent(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Outer type=\"project\">\n" +
		"outer text\n" +
		"<Inner type=\"project\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"trailing outer text\n" +
		"</Outer>\n" +
		"</Identity>\n"
	want := "<Identity type=\"core\">\n" +
		"<Outer type=\"project\">\n" +
		"<Inner type=\"project\">\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_Empty_DeepNestingInsideManagedKeepsDirectChildrenInOrder(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Outer type=\"project\">\n" +
		"outer text\n" +
		"<Inner type=\"project\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"<Second type=\"project\">\n" +
		"second text\n" +
		"</Second>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Outer type=\"project\">\n" +
		"<Inner type=\"project\">\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"<Second type=\"project\">\n" +
		"</Second>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

// ---------------------------------------------------------------------------
// Version attribute, whitespace-only content, line terminators
// ---------------------------------------------------------------------------

func TestBuildGenericAgent_Empty_VersionAttributeOnNestedProjectRegionIsDropped(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Outer type=\"project\" version=\"1.0.0\">\n" +
		"<Inner type=\"project\" version=\"2.0.0\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"</Identity>\n"
	want := "<Identity type=\"core\">\n" +
		"<Outer type=\"project\">\n" +
		"<Inner type=\"project\">\n" +
		"</Inner>\n" +
		"</Outer>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_Empty_VersionAttributeOnProjectRegionInsideManagedIsDropped(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Proj type=\"project\" version=\"3.0.0\">\n" +
		"proj content\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Proj type=\"project\">\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_WhitespaceOnlyProjectRegion_KeepIsByteIdenticalEmptyIsEmptyPair(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Alpha type=\"project\">\n" +
		"   \n" +
		"</Alpha>\n" +
		"</Identity>\n"
	wantEmpty := "<Identity type=\"core\">\n" +
		"<Alpha type=\"project\">\n" +
		"</Alpha>\n" +
		"</Identity>\n"

	gotKeep := generateRegionsBody(t, body, false)
	gotEmpty := generateRegionsBody(t, body, true)

	assertRegionsBody(t, gotKeep, body)
	assertRegionsBody(t, gotEmpty, wantEmpty)
}

func TestBuildGenericAgent_CRLFSource_EmptiedManagedRegionKeepsChildrenWithCRLF(t *testing.T) {
	body := "<Constraints type=\"core\">\r\n" +
		"<ClosingProcedure type=\"managed\">\r\n" +
		"managed prose\r\n" +
		"<Proj type=\"project\">\r\n" +
		"proj content\r\n" +
		"</Proj>\r\n" +
		"</ClosingProcedure>\r\n" +
		"</Constraints>\r\n"
	wantKeep := "<Constraints type=\"core\">\r\n" +
		"<ClosingProcedure type=\"managed\">\r\n" +
		"<Proj type=\"project\">\r\n" +
		"proj content\r\n" +
		"</Proj>\r\n" +
		"</ClosingProcedure>\r\n" +
		"</Constraints>\r\n"
	wantEmpty := "<Constraints type=\"core\">\r\n" +
		"<ClosingProcedure type=\"managed\">\r\n" +
		"<Proj type=\"project\">\r\n" +
		"</Proj>\r\n" +
		"</ClosingProcedure>\r\n" +
		"</Constraints>\r\n"

	gotKeep := generateRegionsBody(t, body, false)
	gotEmpty := generateRegionsBody(t, body, true)

	assertRegionsBody(t, gotKeep, wantKeep)
	assertRegionsBody(t, gotEmpty, wantEmpty)
}

func TestBuildGenericAgent_CRLFSource_EmptiedProjectRegionKeepsNestedChildrenWithCRLF(t *testing.T) {
	body := "<Identity type=\"core\">\r\n" +
		"<Outer type=\"project\">\r\n" +
		"outer text\r\n" +
		"<Inner type=\"project\">\r\n" +
		"inner text\r\n" +
		"</Inner>\r\n" +
		"</Outer>\r\n" +
		"</Identity>\r\n"
	want := "<Identity type=\"core\">\r\n" +
		"<Outer type=\"project\">\r\n" +
		"<Inner type=\"project\">\r\n" +
		"</Inner>\r\n" +
		"</Outer>\r\n" +
		"</Identity>\r\n"

	got := generateRegionsBody(t, body, true)

	assertRegionsBody(t, got, want)
}

// emptyProjectsPromoteInput is standardPromoteInput with the empty choice set.
func emptyProjectsPromoteInput(src []byte) PromoteInput {
	in := standardPromoteInput(src)
	in.EmptyProjectRegions = true
	return in
}
