package app_test

// thirdformat_transform_destination_parity_test.go specifies that transform and rebuild behave
// identically when the DESTINATION harness uses the test-only agent format and when it uses
// Markdown, from the same source harness: the same plan, report, tool conversion and decoded
// content, and, for an existing destination file, the same preserved user-owned keys. The prior
// bytes of the existing file reach the encoder, so format-native content of the test format (its
// native member) survives an overwrite.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	twinNativeKey   = "nativeKey"
	twinNativeValue = "nativeValue"
)

// twinAddNative returns the test-format file raw with a native member added.
func twinAddNative(t *testing.T, raw []byte) []byte {
	t.Helper()
	s := strings.TrimRight(string(raw), "\n")
	if !strings.HasSuffix(s, "}") {
		t.Fatalf("test-format file is not a JSON object:\n%s", raw)
	}
	return []byte(strings.TrimSuffix(s, "}") + ",\n  \"native\": {\"" + twinNativeKey + "\": \"" + twinNativeValue + "\"}\n}\n")
}

// twinPlantNative adds a native member to the deployed twin agent key of the test-format side.
func twinPlantNative(t *testing.T, side twinSide, key string) {
	t.Helper()
	rel := twinAgentRel(t, side, key)
	gamWrite(t, side.Workspace, rel, string(twinAddNative(t, gamRead(t, side.Workspace, rel))))
}

// twinCounterpartSource deploys the counterpart harness to both twins' workspaces.
func twinCounterpartSource(t *testing.T) twinPair {
	t.Helper()
	p := twinTransformPair(t)
	runTwinStep(t, p, twinDeployCounterpartStep)
	return p
}

// twinExistingDestination deploys the twin as well and plants user content in the runner.
func twinExistingDestination(t *testing.T) twinPair {
	t.Helper()
	p := twinCounterpartSource(t)
	runTwinStep(t, p, twinDeployNewStep)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinPlantUserContent(t))
	}
	twinPlantNative(t, p.Test, owRunner)
	return p
}

func TestTwinParity_TransformToTwin_FreshDestination(t *testing.T) {
	// Arrange
	p := twinCounterpartSource(t)
	step := twinTransformStep(twinCounterpartID(t), twinHarnessID, false)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform into a fresh test-format destination", test, md)
	twinRequireTransformed(t, "test format", test)
	twinRequireTransformed(t, "markdown", md)
	for _, key := range owAllKeys {
		if _, ok := test.Agents[key]; !ok {
			t.Errorf("agent %s was not written to the test-format destination", key)
		}
	}
}

func TestTwinParity_TransformToTwin_ExistingDestinationOverwritten(t *testing.T) {
	// Arrange
	p := twinExistingDestination(t)
	step := twinTransformStep(twinCounterpartID(t), twinHarnessID, true)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform over an existing test-format destination", test, md)
	twinRequireTransformed(t, "test format", test)
	twinRequireTransformed(t, "markdown", md)
	// Transform with overwrite replaces the destination from the source for both formats; it does
	// not carry destination user keys (verified: Markdown drops them too). User-key survival is an
	// update-flow property, asserted by the edit-detection and refresh parity tests; only parity
	// and native-member survival are required here.
}

func TestTwinParity_TransformToTwin_ExistingDestination_NativeContentSurvives(t *testing.T) {
	// Arrange
	p := twinExistingDestination(t)
	step := twinTransformStep(twinCounterpartID(t), twinHarnessID, true)

	// Act
	runTwinFlowStep(t, p, step)

	// Assert: the prior bytes reached the test-format encoder.
	raw := string(gamRead(t, p.Test.Workspace, twinAgentRel(t, p.Test, owRunner)))
	if !strings.Contains(raw, twinNativeValue) {
		t.Errorf("format-native content of the existing destination was lost by the overwrite:\n%s", raw)
	}
}

func TestTwinParity_TransformToTwin_ExistingDestinationKeptWithoutOverwrite(t *testing.T) {
	// Arrange
	p := twinExistingDestination(t)
	before := string(gamRead(t, p.Test.Workspace, twinAgentRel(t, p.Test, owRunner)))
	step := twinTransformStep(twinCounterpartID(t), twinHarnessID, false)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform into an existing test-format destination without overwrite", test, md)
	if got := twinTransformFile(t, test, owRunner).Status; got != app.StatusSkipped {
		t.Errorf("status of %s = %q; want skipped when the destination exists", owRunner, got)
	}
	if after := string(gamRead(t, p.Test.Workspace, twinAgentRel(t, p.Test, owRunner))); after != before {
		t.Errorf("a skipped destination file was changed")
	}
}

// twinRebuildToTwin rebuilds the Claude Code fixture into the twin harness of format f. With
// existing set, the destination is an existing twin file (user-owned key, old instructions and,
// for the test format, a native member) and the operation is an update.
func twinRebuildToTwin(t *testing.T, f twinFormat, existing bool) (app.RebuildResult, []byte) {
	t.Helper()
	side := twinSide{Format: f, Module: newTestTwinHarness(t, f)}
	in := rbInput(t, rbPair{"claude-to-twin", hClaude, hClaude}, false)
	in.TargetModule = side.Module
	in.Op = agentformat.OpCreate
	if existing {
		old := "---\n" + twinUserKey + ": " + twinUserValue + "\n---\nOLD-DESTINATION-INSTRUCTIONS\n"
		raw, _, err := twinTranslator(t, side).Encode([]byte(old), agentformat.ArtifactContext{
			AgentKey: "code-reviewer", Kind: domain.ArtifactAgent, Op: agentformat.OpCreate})
		if err != nil {
			t.Fatalf("encode the existing destination: %v", err)
		}
		if f == twinTestFormat {
			raw = twinAddNative(t, raw)
		}
		in.Op, in.DestinationRaw = agentformat.OpUpdate, raw
	}
	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("rebuild into the %s twin: %v", side.name(), err)
	}
	return res, twinDecodeOutput(t, f, res.Output)
}

func TestTwinParity_RebuildToTwin_Create_DecodedOutputAndReportEqualMarkdownTwin(t *testing.T) {
	// Act
	test, testCanonical := twinRebuildToTwin(t, twinTestFormat, false)
	md, mdCanonical := twinRebuildToTwin(t, twinMarkdown, false)

	// Assert
	if len(md.Conversions) == 0 {
		t.Fatal("fixture yields no tool conversions; the parity check would be vacuous")
	}
	twinAssertRebuildParity(t, test, md, testCanonical, mdCanonical)
}

func TestTwinParity_RebuildToTwin_Overwrite_PreservesNativeContentAndEqualsMarkdownTwin(t *testing.T) {
	// Act
	test, testCanonical := twinRebuildToTwin(t, twinTestFormat, true)
	md, mdCanonical := twinRebuildToTwin(t, twinMarkdown, true)

	// Assert
	twinAssertRebuildParity(t, test, md, testCanonical, mdCanonical)
	// Rebuild itself does not carry destination user keys for Markdown either (user-key survival
	// is a Deploy/Transform flow property, asserted by the flow-level parity tests), so only
	// parity of the decoded output is required here.
	if strings.Contains(string(testCanonical), "OLD-DESTINATION-INSTRUCTIONS") {
		t.Errorf("overwrite must carry the source instructions, not the destination's:\n%s", testCanonical)
	}
	if !strings.Contains(string(test.Output), twinNativeValue) {
		t.Errorf("format-native content of the existing destination was lost:\n%s", test.Output)
	}
}
