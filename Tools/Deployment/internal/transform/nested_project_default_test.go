package transform_test

// nested_project_default_test.go covers a generic source whose managed region contains a
// project region with content. Apply must accept the source and deploy the nested content as the
// deploy-once default, exactly like a top-level project region: a fresh deploy carries it, an
// update keeps whatever the deployed file already holds, and an update whose deployed file lacks
// the region receives the source default.

import (
	"bytes"
	"errors"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

const (
	nestedDefaultKey      = "nested-default-test"
	nestedDefaultRegion   = "NestedInjection"
	nestedSourceContent   = "Default nested content authored by the agent designer.\nSecond default line.\n"
	nestedDeployedContent = "Content the project already deployed into the nested region.\nKeep me.\n"
	fixtureCanonicalText  = "Fixture harness constraint: always use the approved tool list.\n" +
		"Do not invoke tools not declared in the agent's tools field.\n"
)

const nestedDefaultSourceHead = `---
id: 610
version: 2.0.0
name: nested-default-test
description: Agent for nested project default testing
model: {model-identifier}
tools: [file_read]
recommended_tier: LOW
tier_rationale: nested default testing
required_skills: []
---

<Identity type="core">
# NestedDefault Agent

You are the NestedDefault agent.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

`

const nestedDefaultDeployedHead = `---
id: 610
version: 2.0.0
transform_version: 3.0.0
injections_version: 1.2.0
description: Agent for nested project default testing
mode: subagent
model: claude/claude-sonnet
tools: [read-file]
---

<Identity type="core">
# NestedDefault Agent

You are the NestedDefault agent.
</Identity>

<Constraints type="core">
## Constraints

Always be helpful.

`

// nestedDefaultSource builds a source whose HarnessConstraints managed region holds inner.
func nestedDefaultSource(inner string) []byte {
	return []byte(nestedDefaultSourceHead +
		"<HarnessConstraints type=\"managed\">\n" + inner + "</HarnessConstraints>\n</Constraints>\n")
}

// nestedRegion returns a nested project region with the given content.
func nestedRegion(content string) string {
	return "<" + nestedDefaultRegion + " type=\"project\">\n" + content + "</" + nestedDefaultRegion + ">\n"
}

// nestedDefaultDeployed builds a deployed file whose HarnessConstraints region holds inner.
func nestedDefaultDeployed(inner string) []byte {
	return []byte(nestedDefaultDeployedHead +
		"<HarnessConstraints type=\"managed\">\n" + inner + "</HarnessConstraints>\n</Constraints>\n")
}

func applyNestedDefault(t *testing.T, source, deployed []byte) (transform.Result, error) {
	t.Helper()
	return transform.Apply(transform.Request{
		Source:   source,
		Deployed: deployed,
		Kind:     domain.ArtifactAgent,
		Key:      nestedDefaultKey,
		Module:   newInjectionFixtureModule(t),
		Model:    fixtureModel(),
		Scope:    domain.ScopeProject,
	})
}

func mustApplyNestedDefault(t *testing.T, source, deployed []byte) transform.Result {
	t.Helper()
	result, err := applyNestedDefault(t, source, deployed)
	if err != nil {
		t.Fatalf("Apply returned an error for a source with a nested project region: %v", err)
	}
	return result
}

// assertNestedContent asserts the output holds the nested region inside HarnessConstraints with
// exactly want as its content, and that the region appears once.
func assertNestedContent(t *testing.T, output []byte, want string) {
	t.Helper()
	doc, err := docformat.Parse(output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	node, ok := doc.Body().Injection(nestedDefaultRegion)
	if !ok {
		t.Fatalf("%s absent from output", nestedDefaultRegion)
	}
	if !bytes.Equal(node.Content(), []byte(want)) {
		t.Errorf("%s content mismatch:\nwant %q\ngot  %q", nestedDefaultRegion, want, node.Content())
	}
	parent := node.Parent()
	if parent == nil || parent.Kind() != docformat.NodeDeployed || parent.Name() != "HarnessConstraints" {
		t.Errorf("%s must stay inside <HarnessConstraints type=\"managed\">", nestedDefaultRegion)
	}
	if n := bytes.Count(output, []byte("<"+nestedDefaultRegion+" ")); n != 1 {
		t.Errorf("%s open tag appears %d times in output, want 1", nestedDefaultRegion, n)
	}
}

func nestedOutcome(result transform.Result) *transform.RegionOutcome {
	return findOutcomeByName(result.Report.Regions, nestedDefaultRegion)
}

func assertDefaultGap(t *testing.T, result transform.Result, want bool) {
	t.Helper()
	gap := findProjectGapOfKind(result, domain.GapDefaultContentDeployed)
	if !want {
		if gap != nil {
			t.Errorf("unexpected default-content gap: %+v", *gap)
		}
		return
	}
	if gap == nil {
		t.Fatalf("expected a default-content gap; gaps: %+v", result.Report.Gaps)
	}
	if gap.Subject != nestedDefaultRegion || gap.Owner != nestedDefaultKey {
		t.Errorf("default-content gap Subject/Owner = %q/%q, want %q/%q",
			gap.Subject, gap.Owner, nestedDefaultRegion, nestedDefaultKey)
	}
}

func assertOutcomeAction(t *testing.T, result transform.Result, want transform.RegionAction) {
	t.Helper()
	outcome := nestedOutcome(result)
	if outcome == nil {
		t.Fatalf("no region outcome for %s", nestedDefaultRegion)
	}
	if outcome.Action != want {
		t.Errorf("%s action = %q, want %q", nestedDefaultRegion, outcome.Action, want)
	}
}

func TestNestedProjectDefault_SourceWithContent_IsAccepted(t *testing.T) {
	_, err := applyNestedDefault(t, nestedDefaultSource(nestedRegion(nestedSourceContent)), nil)

	if errors.Is(err, transform.ErrSourceDeployedRegionNotEmpty) {
		t.Fatalf("a nested project region with content must not make the managed region count as populated: %v", err)
	}
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestNestedProjectDefault_FreshDeploy_CarriesSourceContent(t *testing.T) {
	result := mustApplyNestedDefault(t, nestedDefaultSource(nestedRegion(nestedSourceContent)), nil)

	assertNestedContent(t, result.Output, nestedSourceContent)
	assertOutcomeAction(t, result, transform.RegionDefaulted)
	assertDefaultGap(t, result, true)
}

func TestNestedProjectDefault_FreshDeploy_ManagedRegionStillGetsCanonicalContent(t *testing.T) {
	result := mustApplyNestedDefault(t, nestedDefaultSource(nestedRegion(nestedSourceContent)), nil)

	canonicalIdx := bytes.Index(result.Output, []byte(fixtureCanonicalText))
	nestedIdx := bytes.Index(result.Output, []byte("<"+nestedDefaultRegion+" "))
	if canonicalIdx < 0 {
		t.Fatal("regenerated managed region lacks the harness canonical content")
	}
	if nestedIdx < 0 || canonicalIdx > nestedIdx {
		t.Error("the nested region must be present and follow the canonical content")
	}
}

func TestNestedProjectDefault_Update_DeployedContentWins(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := nestedDefaultDeployed(fixtureCanonicalText + "\n" + nestedRegion(nestedDeployedContent))

	result := mustApplyNestedDefault(t, source, deployed)

	assertNestedContent(t, result.Output, nestedDeployedContent)
	assertOutcomeAction(t, result, transform.RegionPreserved)
	assertDefaultGap(t, result, false)
	if bytes.Contains(result.Output, []byte("Default nested content authored")) {
		t.Error("source default leaked into the output although the deployed file holds the region")
	}
}

func TestNestedProjectDefault_Update_DeployedLacksRegion_WritesSourceDefault(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := nestedDefaultDeployed(fixtureCanonicalText)

	result := mustApplyNestedDefault(t, source, deployed)

	assertNestedContent(t, result.Output, nestedSourceContent)
	assertOutcomeAction(t, result, transform.RegionDefaulted)
	assertDefaultGap(t, result, true)
}

func TestNestedProjectDefault_Update_DeployedWhitespaceOnlyRegion_StillWins(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := nestedDefaultDeployed(fixtureCanonicalText + "\n" + nestedRegion("\n"))

	result := mustApplyNestedDefault(t, source, deployed)

	assertNestedContent(t, result.Output, "\n")
	assertDefaultGap(t, result, false)
}

func TestNestedProjectDefault_Update_DeployedHasRegionTopLevel_DeployedContentWins(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := bytes.Replace(nestedDefaultDeployed(fixtureCanonicalText),
		[]byte("</Constraints>\n"), []byte(nestedRegion(nestedDeployedContent)+"</Constraints>\n"), 1)

	result := mustApplyNestedDefault(t, source, deployed)

	assertNestedContent(t, result.Output, nestedDeployedContent)
	assertDefaultGap(t, result, false)
}

func TestNestedProjectDefault_WhitespaceOnlySource_StaysEmptyAndReportedAdded(t *testing.T) {
	result := mustApplyNestedDefault(t, nestedDefaultSource(nestedRegion("")), nil)

	doc, err := docformat.Parse(result.Output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	node, ok := doc.Body().Injection(nestedDefaultRegion)
	if !ok {
		t.Fatalf("%s absent from output", nestedDefaultRegion)
	}
	if len(bytes.TrimSpace(node.Content())) != 0 {
		t.Errorf("%s must stay empty, got %q", nestedDefaultRegion, node.Content())
	}
	assertOutcomeAction(t, result, transform.RegionAdded)
	assertDefaultGap(t, result, false)
}

func TestNestedProjectDefault_ManagedRegionWithProse_StillRejected(t *testing.T) {
	source := nestedDefaultSource("Prose that belongs to nobody.\n" + nestedRegion(nestedSourceContent))

	_, err := applyNestedDefault(t, source, nil)

	if !errors.Is(err, transform.ErrSourceDeployedRegionNotEmpty) {
		t.Fatalf("want error wrapping ErrSourceDeployedRegionNotEmpty for prose in a managed region, got: %v", err)
	}
}

func nestedCustomRegion(content string) string {
	return "<SourceNote type=\"custom\">\n" + content + "</SourceNote>\n"
}

func TestNestedProjectDefault_ManagedRegionWithNestedCustomContent_StillRejected(t *testing.T) {
	source := nestedDefaultSource(nestedCustomRegion("Custom content that belongs to nobody.\n") +
		nestedRegion(nestedSourceContent))

	_, err := applyNestedDefault(t, source, nil)

	if !errors.Is(err, transform.ErrSourceDeployedRegionNotEmpty) {
		t.Fatalf("want error wrapping ErrSourceDeployedRegionNotEmpty for a nested custom region with content, got: %v", err)
	}
}

func TestNestedProjectDefault_WhitespaceOnlyNestedCustomRegion_IsDisregarded(t *testing.T) {
	source := nestedDefaultSource(nestedCustomRegion("\n") + nestedRegion(nestedSourceContent))

	result := mustApplyNestedDefault(t, source, nil)

	assertNestedContent(t, result.Output, nestedSourceContent)
	assertOutcomeAction(t, result, transform.RegionDefaulted)
}

func TestNestedProjectDefault_TwoNestedRegions_ContentDefaultedEmptyAddedOrderKept(t *testing.T) {
	second := "<SecondInjection type=\"project\">\n</SecondInjection>\n"
	source := nestedDefaultSource(nestedRegion(nestedSourceContent) + second)

	result := mustApplyNestedDefault(t, source, nil)

	assertNestedContent(t, result.Output, nestedSourceContent)
	assertOutcomeAction(t, result, transform.RegionDefaulted)
	secondOutcome := findOutcomeByName(result.Report.Regions, "SecondInjection")
	if secondOutcome == nil || secondOutcome.Action != transform.RegionAdded {
		t.Errorf("SecondInjection outcome = %+v, want action %q", secondOutcome, transform.RegionAdded)
	}
	firstIdx := bytes.Index(result.Output, []byte("<"+nestedDefaultRegion+" "))
	secondIdx := bytes.Index(result.Output, []byte("<SecondInjection "))
	if firstIdx < 0 || secondIdx < 0 || firstIdx > secondIdx {
		t.Error("nested regions must keep source order in the output")
	}
}

func TestNestedProjectDefault_Update_UnchangedCanonical_NoContentChangedGap(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := nestedDefaultDeployed(fixtureCanonicalText + "\n" + nestedRegion(nestedDeployedContent))

	result := mustApplyNestedDefault(t, source, deployed)

	if gap := findProjectGapOfKind(result, domain.GapDeployedRegionContentChanged); gap != nil {
		t.Errorf("nested project content must not count as managed canonical content; got gap %+v", *gap)
	}
}

func TestNestedProjectDefault_Update_ChangedCanonical_ReportsContentChangedGap(t *testing.T) {
	source := nestedDefaultSource(nestedRegion(nestedSourceContent))
	deployed := nestedDefaultDeployed("Stale canonical text from an older harness release.\n\n" +
		nestedRegion(nestedDeployedContent))

	result := mustApplyNestedDefault(t, source, deployed)

	gap := findProjectGapOfKind(result, domain.GapDeployedRegionContentChanged)
	if gap == nil {
		t.Fatalf("a changed managed canonical text with a nested project region must report a content-changed gap; gaps: %+v",
			result.Report.Gaps)
	}
	if gap.Subject != "HarnessConstraints" {
		t.Errorf("content-changed gap Subject = %q, want HarnessConstraints", gap.Subject)
	}
	assertNestedContent(t, result.Output, nestedDeployedContent)
}
