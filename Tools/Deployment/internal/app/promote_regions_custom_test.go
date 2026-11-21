package app

// promote_regions_custom_test.go covers how buildGenericAgent removes custom regions and the
// invariants that hold over a source mixing project, managed and custom regions.
//
// Verified behaviours:
//   - No custom region remains at any depth; a project region inside a removed custom
//     region is kept where the custom region was; identical-looking text elsewhere is
//     untouched.
//   - The set of project region names equals the source's under both choices.
//   - The input bytes are never modified.
//   - The source's line terminator is used by the kept-children layout.

import (
	"bytes"
	"testing"

	"mosaic-common/docformat"
)

// ---------------------------------------------------------------------------
// Custom regions
// ---------------------------------------------------------------------------

func TestBuildGenericAgent_CustomRegionTopLevel_IsRemovedWithTags(t *testing.T) {
	for _, empty := range []bool{false, true} {
		body := "<Identity type=\"core\">\n" +
			"before\n" +
			"<Note type=\"custom\">\n" +
			"note text\n" +
			"</Note>\n" +
			"after\n" +
			"</Identity>\n"
		want := "<Identity type=\"core\">\n" +
			"before\n" +
			"after\n" +
			"</Identity>\n"

		got := generateRegionsBody(t, body, empty)

		assertRegionsBody(t, got, want)
	}
}

func TestBuildGenericAgent_CustomRegionInsideProject_Keep_RemovedFromKeptContent(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Proj type=\"project\">\n" +
		"before\n" +
		"<Note type=\"custom\">\n" +
		"secret\n" +
		"</Note>\n" +
		"after\n" +
		"</Proj>\n" +
		"</Identity>\n"
	want := "<Identity type=\"core\">\n" +
		"<Proj type=\"project\">\n" +
		"before\n" +
		"after\n" +
		"</Proj>\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, want)
}

func TestBuildGenericAgent_CustomRegionInsideManaged_IsRemoved(t *testing.T) {
	for _, empty := range []bool{false, true} {
		body := "<Constraints type=\"core\">\n" +
			"<ClosingProcedure type=\"managed\">\n" +
			"<Note type=\"custom\">\n" +
			"note text\n" +
			"</Note>\n" +
			"</ClosingProcedure>\n" +
			"</Constraints>\n"
		want := "<Constraints type=\"core\">\n" +
			"<ClosingProcedure type=\"managed\">\n" +
			"</ClosingProcedure>\n" +
			"</Constraints>\n"

		got := generateRegionsBody(t, body, empty)

		assertRegionsBody(t, got, want)
	}
}

func TestBuildGenericAgent_ProjectInsideCustomInsideManaged_IsKeptWhereCustomWas(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Note type=\"custom\">\n" +
		"<Proj type=\"project\">\n" +
		"proj content\n" +
		"</Proj>\n" +
		"</Note>\n" +
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

func TestBuildGenericAgent_ProjectInsideCustomInsideProject_Empty_IsKeptAsDirectChild(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"<Outer type=\"project\">\n" +
		"outer text\n" +
		"<Note type=\"custom\">\n" +
		"<Inner type=\"project\">\n" +
		"inner text\n" +
		"</Inner>\n" +
		"</Note>\n" +
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

func TestBuildGenericAgent_CustomRemoval_LeavesIdenticalLookingContentElsewhere(t *testing.T) {
	body := "<Identity type=\"core\">\n" +
		"shared line\n" +
		"<Proj type=\"project\">\n" +
		"shared line\n" +
		"</Proj>\n" +
		"<Note type=\"custom\">\n" +
		"shared line\n" +
		"</Note>\n" +
		"shared line\n" +
		"</Identity>\n"
	want := "<Identity type=\"core\">\n" +
		"shared line\n" +
		"<Proj type=\"project\">\n" +
		"shared line\n" +
		"</Proj>\n" +
		"shared line\n" +
		"</Identity>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, want)
}

// ---------------------------------------------------------------------------
// Invariants over a combined source
// ---------------------------------------------------------------------------

const combinedRegionsBody = "<Identity type=\"core\">\n" +
	"<A type=\"project\">\n" +
	"a text\n" +
	"<Note1 type=\"custom\">\n" +
	"<B type=\"project\">\n" +
	"b text\n" +
	"</B>\n" +
	"</Note1>\n" +
	"</A>\n" +
	"</Identity>\n" +
	"<Constraints type=\"core\">\n" +
	"<ClosingProcedure type=\"managed\">\n" +
	"prose\n" +
	"<C type=\"project\">\n" +
	"c text\n" +
	"<D type=\"project\">\n" +
	"d text\n" +
	"</D>\n" +
	"</C>\n" +
	"<Note2 type=\"custom\">\n" +
	"note\n" +
	"</Note2>\n" +
	"</ClosingProcedure>\n" +
	"<Note3 type=\"custom\">\n" +
	"top note\n" +
	"</Note3>\n" +
	"</Constraints>\n"

func TestBuildGenericAgent_BothChoices_EveryProjectNamePresentAndNoCustomRemains(t *testing.T) {
	src := regionsSource(combinedRegionsBody)
	wantNames := projectNames(t, src)
	if len(wantNames) != 4 {
		t.Fatalf("fixture should hold four project regions, got %v", wantNames)
	}
	for _, empty := range []bool{false, true} {
		in := standardPromoteInput(src)
		in.EmptyProjectRegions = empty

		out, err := buildGenericAgent(in)
		if err != nil {
			t.Fatalf("empty=%v: buildGenericAgent failed: %v", empty, err)
		}

		gotNames := projectNames(t, out)
		if len(gotNames) != len(wantNames) {
			t.Fatalf("empty=%v: project names = %v, want %v", empty, gotNames, wantNames)
		}
		for i := range wantNames {
			if gotNames[i] != wantNames[i] {
				t.Errorf("empty=%v: project names = %v, want %v", empty, gotNames, wantNames)
				break
			}
		}
		parsed, err := docformat.Parse(out)
		if err != nil {
			t.Fatalf("empty=%v: parse failed: %v", empty, err)
		}
		if customs := parsed.Body().CustomRegions(); len(customs) != 0 {
			t.Errorf("empty=%v: %d custom regions remain in the generic", empty, len(customs))
		}
	}
}

func TestBuildGenericAgent_Empty_NoProjectRegionHoldsAnythingButNestedProjectRegions(t *testing.T) {
	in := standardPromoteInput(regionsSource(combinedRegionsBody))
	in.EmptyProjectRegions = true

	out, err := buildGenericAgent(in)
	if err != nil {
		t.Fatalf("buildGenericAgent failed: %v", err)
	}

	parsed, err := docformat.Parse(out)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	for _, node := range parsed.Body().Injections() {
		content := node.Content()
		for _, child := range node.Children() {
			content = bytes.ReplaceAll(content, child.Bytes(), nil)
		}
		if len(bytes.TrimSpace(content)) > 0 {
			t.Errorf("project region %q holds content besides nested regions: %q", node.Name(), content)
		}
	}
}

// Regression/invariant check: managed regions are already cleared today; this asserts nothing
// but kept project regions remain, it does not itself require projects to be kept.
func TestBuildGenericAgent_Keep_ManagedRegionsHoldOnlyKeptProjectRegions(t *testing.T) {
	in := standardPromoteInput(regionsSource(combinedRegionsBody))

	out, err := buildGenericAgent(in)
	if err != nil {
		t.Fatalf("buildGenericAgent failed: %v", err)
	}

	parsed, err := docformat.Parse(out)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	managed := parsed.Body().DeployedRegions()
	if len(managed) == 0 {
		t.Fatal("no managed region in output")
	}
	for _, node := range managed {
		content := node.Content()
		for _, child := range node.Children() {
			content = bytes.ReplaceAll(content, child.Bytes(), nil)
		}
		if len(bytes.TrimSpace(content)) > 0 {
			t.Errorf("managed region %q holds more than its kept project regions: %q", node.Name(), content)
		}
	}
}

func TestBuildGenericAgent_WithCustomAndNestedRegions_DoesNotMutateSourceBytes(t *testing.T) {
	for _, empty := range []bool{false, true} {
		src := regionsSource(combinedRegionsBody)
		before := append([]byte(nil), src...)
		in := standardPromoteInput(src)
		in.EmptyProjectRegions = empty

		if _, err := buildGenericAgent(in); err != nil {
			t.Fatalf("empty=%v: buildGenericAgent failed: %v", empty, err)
		}

		if !bytes.Equal(src, before) {
			t.Errorf("empty=%v: source bytes changed", empty)
		}
	}
}

func TestBuildGenericAgent_CustomInsideProjectInsideManaged_Keep_RemovedFromKeptContent(t *testing.T) {
	body := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"managed prose\n" +
		"<Proj type=\"project\">\n" +
		"before\n" +
		"<Note type=\"custom\">\n" +
		"secret\n" +
		"</Note>\n" +
		"after\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"
	want := "<Constraints type=\"core\">\n" +
		"<ClosingProcedure type=\"managed\">\n" +
		"<Proj type=\"project\">\n" +
		"before\n" +
		"after\n" +
		"</Proj>\n" +
		"</ClosingProcedure>\n" +
		"</Constraints>\n"

	got := generateRegionsBody(t, body, false)

	assertRegionsBody(t, got, want)
}
