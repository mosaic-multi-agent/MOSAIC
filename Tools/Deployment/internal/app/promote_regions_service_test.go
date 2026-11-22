package app_test

// promote_regions_service_test.go covers how Service.Promote handles project and custom
// regions of the source agent:
//   - project region content is kept by default and emptied when the request asks;
//   - every custom region of the source, at any depth, is removed from the written generic
//     and reported by name in document order, in dry runs too;
//   - the source file on disk is never changed.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/app/interactiontest"
)

const regionsAgentBody = "<Identity type=\"core\">\n" +
	"You are the regions agent.\n" +
	"<IdentityExtension type=\"project\">\n" +
	"Keep this project text.\n" +
	"<TopNote type=\"custom\">\n" +
	"top custom text\n" +
	"</TopNote>\n" +
	"</IdentityExtension>\n" +
	"</Identity>\n" +
	"<CommunicationProtocol type=\"managed\">\n" +
	"Protocol content.\n" +
	"<ManagedNote type=\"custom\">\n" +
	"managed custom text\n" +
	"</ManagedNote>\n" +
	"<NestedExtension type=\"project\">\n" +
	"Nested project text.\n" +
	"</NestedExtension>\n" +
	"</CommunicationProtocol>\n" +
	"<Capabilities type=\"core\">\n" +
	"Capability prose.\n" +
	"<LateNote type=\"custom\">\n" +
	"late custom text\n" +
	"</LateNote>\n" +
	"</Capabilities>\n"

const regionsAgentFrontmatter = "---\n" +
	"transform_version: \"2.1.0\"\n" +
	"injections_version: \"3.0.0\"\n" +
	"id: \"77\"\n" +
	"version: \"1.5.0\"\n" +
	"name: regions-agent\n" +
	"description: An agent with regions\n" +
	"role: subagent\n" +
	"recommended_tier: MEDIUM\n" +
	"tier_rationale: moderate capability needed for this task\n" +
	"tools: [file_read, bash]\n" +
	"required_skills: [lean-tdd]\n" +
	"---\n"

// promoteRegionsAgent writes the regions agent source into a temp dir, promotes it with
// the given option and returns the result, the source bytes and path, and the destination.
func promoteRegionsAgent(t *testing.T, body string, empty, dryRun bool) (app.PromoteResult, string) {
	t.Helper()
	mosaicRoot := writeMosaicRoot(t)
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "regions-agent.md")
	if err := os.WriteFile(srcPath, []byte(regionsAgentFrontmatter+body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	deps, _ := newPromoteDeps(t, interactiontest.NewBuilder().Build(), mosaicRoot)
	result, err := app.New(deps).Promote(context.Background(), app.PromoteRequest{
		FilePath:            srcPath,
		Category:            "TestCategory",
		HarnessID:           "stub-harness",
		DryRun:              dryRun,
		EmptyProjectRegions: empty,
	})
	if err != nil {
		t.Fatalf("Promote: unexpected error: %v", err)
	}
	return result, srcPath
}

func readPromotedRegionsDoc(t *testing.T, result app.PromoteResult) *docformat.Document {
	t.Helper()
	data, err := os.ReadFile(result.DestinationPath)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	doc, err := docformat.Parse(data)
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	return doc
}

func TestPromote_Default_KeepsProjectRegionContent(t *testing.T) {
	result, _ := promoteRegionsAgent(t, regionsAgentBody, false, false)

	doc := readPromotedRegionsDoc(t, result)

	got := map[string]string{}
	for _, n := range doc.Body().Injections() {
		got[n.Name()] = string(bytes.TrimSpace(n.Content()))
	}
	if got["IdentityExtension"] != "Keep this project text." {
		t.Errorf("IdentityExtension content = %q, want the source text kept", got["IdentityExtension"])
	}
	if got["NestedExtension"] != "Nested project text." {
		t.Errorf("NestedExtension (inside a managed region) content = %q, want the source text kept",
			got["NestedExtension"])
	}
}

func TestPromote_EmptyProjectRegions_LeavesEveryProjectRegionEmpty(t *testing.T) {
	result, _ := promoteRegionsAgent(t, regionsAgentBody, true, false)

	doc := readPromotedRegionsDoc(t, result)

	injections := doc.Body().Injections()
	names := map[string]bool{}
	for _, n := range injections {
		names[n.Name()] = true
		if len(bytes.TrimSpace(n.Content())) != 0 {
			t.Errorf("project region %q is not empty: %q", n.Name(), n.Content())
		}
	}
	for _, want := range []string{"IdentityExtension", "NestedExtension"} {
		if !names[want] {
			t.Errorf("project region %q missing from the generic", want)
		}
	}
}

func TestPromote_WrittenGeneric_HasNoCustomRegion(t *testing.T) {
	for _, empty := range []bool{false, true} {
		result, _ := promoteRegionsAgent(t, regionsAgentBody, empty, false)

		doc := readPromotedRegionsDoc(t, result)

		if customs := doc.Body().CustomRegions(); len(customs) != 0 {
			t.Errorf("empty=%v: %d custom regions remain in the written generic", empty, len(customs))
		}
	}
}

func TestPromote_ReportsRemovedCustomRegionsByNameInDocumentOrder(t *testing.T) {
	result, _ := promoteRegionsAgent(t, regionsAgentBody, false, false)

	want := []string{"TopNote", "ManagedNote", "LateNote"}
	if !reflect.DeepEqual(result.RemovedCustomRegions, want) {
		t.Errorf("RemovedCustomRegions = %v, want %v", result.RemovedCustomRegions, want)
	}
}

func TestPromote_DryRun_ReportsRemovedCustomRegionsAndWritesNothing(t *testing.T) {
	result, _ := promoteRegionsAgent(t, regionsAgentBody, false, true)

	want := []string{"TopNote", "ManagedNote", "LateNote"}
	if !reflect.DeepEqual(result.RemovedCustomRegions, want) {
		t.Errorf("RemovedCustomRegions = %v, want %v", result.RemovedCustomRegions, want)
	}
	if _, err := os.Stat(result.DestinationPath); err == nil {
		t.Errorf("dry run wrote %s", result.DestinationPath)
	}
}

func TestPromote_NoCustomRegionInSource_ReportsNone(t *testing.T) {
	body := "<Identity type=\"core\">\nProse.\n</Identity>\n"

	result, _ := promoteRegionsAgent(t, body, false, false)

	if len(result.RemovedCustomRegions) != 0 {
		t.Errorf("RemovedCustomRegions = %v, want none", result.RemovedCustomRegions)
	}
}

func TestPromote_SourceWithCustomRegions_SourceFileUnchangedUnderBothChoices(t *testing.T) {
	for _, empty := range []bool{false, true} {
		_, srcPath := promoteRegionsAgent(t, regionsAgentBody, empty, false)

		got, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read source: %v", err)
		}

		if want := regionsAgentFrontmatter + regionsAgentBody; string(got) != want {
			t.Errorf("empty=%v: source file changed by promote", empty)
		}
	}
}

const regionsNestedNamesBody = "<Identity type=\"core\">\n" +
	"<Ext type=\"project\">\n" +
	"<OuterNote type=\"custom\">\n" +
	"<InnerNote type=\"custom\">\n" +
	"inner\n" +
	"</InnerNote>\n" +
	"</OuterNote>\n" +
	"</Ext>\n" +
	"<SecondNote type=\"custom\">\n" +
	"first\n" +
	"</SecondNote>\n" +
	"</Identity>\n" +
	"<Capabilities type=\"core\">\n" +
	"<ThirdNote type=\"custom\">\n" +
	"second\n" +
	"</ThirdNote>\n" +
	"</Capabilities>\n"

func TestPromote_RemovedCustomRegions_NestedNamesInDocumentOrder(t *testing.T) {
	result, _ := promoteRegionsAgent(t, regionsNestedNamesBody, false, false)

	want := []string{"OuterNote", "InnerNote", "SecondNote", "ThirdNote"}
	if !reflect.DeepEqual(result.RemovedCustomRegions, want) {
		t.Errorf("RemovedCustomRegions = %v, want %v", result.RemovedCustomRegions, want)
	}
}

func TestPromote_DryRun_SourceFileUnchangedUnderBothChoices(t *testing.T) {
	for _, empty := range []bool{false, true} {
		_, srcPath := promoteRegionsAgent(t, regionsAgentBody, empty, true)

		got, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("read source: %v", err)
		}

		if want := regionsAgentFrontmatter + regionsAgentBody; string(got) != want {
			t.Errorf("empty=%v: source file changed by dry run", empty)
		}
	}
}
