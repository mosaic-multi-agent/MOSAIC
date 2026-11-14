package app_test

// thirdformat_transform_source_parity_test.go specifies that transform and rebuild behave
// identically when the SOURCE harness uses the test-only agent format and when it uses Markdown:
// the same plan actions and reasons, the same reported stripped fields and notes, the same tool
// conversion, the same decoded content in the destination harness and the same manifest records
// (apart from the content hash, the deployment time and the agent file extension). The
// destination is the same counterpart harness for both twins, one whose tools field is
// list-shaped so tool conversion runs.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// twinTransformPair builds both twins with the counterpart harness registered.
func twinTransformPair(t *testing.T) twinPair {
	t.Helper()
	return newTestTwinPairWith(t, []domain.HarnessModule{twinCounterpart(t)})
}

// twinCounterpartID is the harness id of the counterpart both twins transform to or from.
func twinCounterpartID(t *testing.T) string {
	t.Helper()
	return twinCounterpart(t).Ref().ID
}

// twinDeployCounterpartStep runs Deploy workspace for the parity workflow on the counterpart
// harness, conflicts skipped.
func twinDeployCounterpartStep(t *testing.T, side twinSide) (domain.RunSummary, error) {
	t.Helper()
	return side.Service.DeployNew(context.Background(), app.DeployRequest{
		HarnessID: twinCounterpartID(t), WorkspacePath: side.Workspace, Scope: domain.ScopeProject,
		WorkflowIDs: []string{"parity-flow"}, UtilityAgentIDs: []string{"parity-helper"},
		AgentModels: twinModels(), AutoConfirmPlan: true, ConflictDefault: domain.DecisionSkip})
}

// twinTransformStep returns the transform step from the harness src to the harness tgt.
func twinTransformStep(src, tgt string, overwrite bool) twinFlowStep {
	return func(t *testing.T, side twinSide) (twinFlowResult, error) {
		t.Helper()
		res, err := side.Service.TransformHarness(context.Background(), app.TransformHarnessRequest{
			SourceHarnessID: src, TargetHarnessID: tgt, Workspace: side.Workspace,
			TargetModel: parityModel, AutoConfirmPlan: true, Overwrite: overwrite})
		if err != nil {
			return twinFlowResult{}, err
		}
		return twinFlowResult{Transform: twinShieldCounterpartExt(t, res)}, nil
	}
}

// twinTransformFile returns the agent outcome of a transform observation.
func twinTransformFile(t *testing.T, obs twinObservation, key string) app.TransformFileOutcome {
	t.Helper()
	if obs.Flow == nil || obs.Flow.Transform == nil {
		t.Fatal("observation holds no transform result")
	}
	for _, f := range obs.Flow.Transform.Files {
		if f.Kind == domain.ArtifactAgent && f.Key == key {
			return f
		}
	}
	t.Fatalf("transform reported no outcome for agent %s", key)
	return app.TransformFileOutcome{}
}

// twinCounterpartRel is the workspace-relative path of a deployed agent of the counterpart.
func twinCounterpartRel(t *testing.T, side twinSide, key string) string {
	t.Helper()
	rel, err := side.Counterparts[0].TargetPath(domain.TargetPathRequest{Kind: domain.ArtifactAgent, Key: key, Scope: domain.ScopeProject})
	if err != nil {
		t.Fatalf("counterpart target path of %s: %v", key, err)
	}
	return rel
}

// twinPlantCounterpartKey adds a user-owned frontmatter key to a deployed counterpart agent.
func twinPlantCounterpartKey(t *testing.T, side twinSide, key string) {
	t.Helper()
	rel := twinCounterpartRel(t, side, key)
	text := string(gamRead(t, side.Workspace, rel))
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("counterpart agent %s has no frontmatter", key)
	}
	gamWrite(t, side.Workspace, rel, "---\n"+twinUserKey+": "+twinUserValue+"\n"+text[len("---\n"):])
}

// twinSourceWithUserContent deploys the twin workspace and plants user content into the runner.
func twinSourceWithUserContent(t *testing.T, p twinPair) {
	t.Helper()
	runTwinStep(t, p, twinDeployNewStep)
	for _, side := range twinSides(p) {
		editTwinAgent(t, side, owRunner, twinPlantUserContent(t))
	}
}

// twinRequireTransformed fails unless every sample agent of the observation was transformed.
func twinRequireTransformed(t *testing.T, label string, obs twinObservation) {
	t.Helper()
	for _, key := range owAllKeys {
		if f := twinTransformFile(t, obs, key); f.Status != app.StatusTransformed {
			t.Errorf("%s: agent %s status %q (reason %q); want transformed", label, key, f.Status, f.Reason)
		}
	}
}

func TestTwinParity_TransformFromTwin_FreshDestination(t *testing.T) {
	// Arrange
	p := twinTransformPair(t)
	twinSourceWithUserContent(t, p)
	step := twinTransformStep(twinHarnessID, twinCounterpartID(t), false)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform from the twin into a fresh destination", test, md)
	twinRequireTransformed(t, "test format", test)
	twinRequireTransformed(t, "markdown", md)
	if len(test.Counterparts) == 0 {
		t.Error("no agent reached the counterpart harness")
	}
}

func TestTwinParity_TransformFromTwin_ReportsNoStrippedFieldsForUserKeys(t *testing.T) {
	// Arrange
	p := twinTransformPair(t)
	twinSourceWithUserContent(t, p)
	step := twinTransformStep(twinHarnessID, twinCounterpartID(t), false)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert: a Markdown source reports nothing as stripped, so the test format reports nothing.
	if got := twinTransformFile(t, md, owRunner).StrippedFields; len(got) != 0 {
		t.Fatalf("markdown source reported stripped fields %+v; the comparison would be vacuous", got)
	}
	if got := twinTransformFile(t, test, owRunner).StrippedFields; len(got) != 0 {
		t.Errorf("test-format source reported stripped fields %+v; want none, as for Markdown", got)
	}
}

func TestTwinParity_TransformFromTwin_ExistingDestinationOverwritten(t *testing.T) {
	// Arrange
	p := twinTransformPair(t)
	twinSourceWithUserContent(t, p)
	runTwinStep(t, p, twinDeployCounterpartStep)
	for _, side := range twinSides(p) {
		twinPlantCounterpartKey(t, side, owRunner)
	}
	step := twinTransformStep(twinHarnessID, twinCounterpartID(t), true)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform from the twin over an existing destination", test, md)
	twinRequireTransformed(t, "test format", test)
	twinRequireTransformed(t, "markdown", md)
}

func TestTwinParity_TransformFromTwin_ExistingDestinationKeptWithoutOverwrite(t *testing.T) {
	// Arrange
	p := twinTransformPair(t)
	twinSourceWithUserContent(t, p)
	runTwinStep(t, p, twinDeployCounterpartStep)
	step := twinTransformStep(twinHarnessID, twinCounterpartID(t), false)

	// Act
	test, md := runTwinFlowStep(t, p, step)

	// Assert
	assertTwinParity(t, "transform from the twin without overwrite", test, md)
	if got := twinTransformFile(t, test, owRunner).Status; got != app.StatusSkipped {
		t.Errorf("status of %s = %q; want skipped when the destination exists", owRunner, got)
	}
}

// twinRebuildFromTwin rebuilds the opencode-shaped fixture, held in the twin's own agent
// format, into the counterpart harness.
func twinRebuildFromTwin(t *testing.T, f twinFormat, edit func([]byte) []byte) app.RebuildResult {
	t.Helper()
	side := twinSide{Format: f, Module: newTestTwinHarness(t, f)}
	canonical := rbFixture(t, hOpen+"-full.md")
	if edit != nil {
		canonical = edit(canonical)
	}
	raw, _, err := twinTranslator(t, side).Encode(canonical, agentformat.ArtifactContext{
		AgentKey: "code-reviewer", Kind: domain.ArtifactAgent, Op: agentformat.OpCreate})
	if err != nil {
		t.Fatalf("encode the source in the %s twin format: %v", side.name(), err)
	}
	in := rbInput(t, rbPair{"twin-to-claude", hOpen, hClaude}, false)
	in.Source = raw
	in.SourceModule = side.Module
	in.TargetModule = twinCounterpart(t)
	in.FileName = "code-reviewer" + twinExtOf(f)
	in.AgentKey = "code-reviewer"
	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("rebuild from the %s twin: %v", side.name(), err)
	}
	return res
}

func TestTwinParity_RebuildFromTwin_ResultEqualsMarkdownTwin(t *testing.T) {
	// Act
	test := twinRebuildFromTwin(t, twinTestFormat, nil)
	md := twinRebuildFromTwin(t, twinMarkdown, nil)

	// Assert
	if len(md.Conversions) == 0 {
		t.Fatal("fixture yields no tool conversions; the parity check would be vacuous")
	}
	twinAssertRebuildParity(t, test, md, test.Output, md.Output)
}

func TestTwinParity_RebuildFromTwin_StrippedFieldReportEqualsMarkdownTwin(t *testing.T) {
	// Arrange
	withUserKey := twinSetField(twinUserKey, twinUserValue)

	// Act
	test := twinRebuildFromTwin(t, twinTestFormat, withUserKey)
	md := twinRebuildFromTwin(t, twinMarkdown, withUserKey)

	// Assert
	if len(md.StrippedFields) != 0 {
		t.Fatalf("markdown source reported stripped fields %+v; want none", md.StrippedFields)
	}
	if len(test.StrippedFields) != 0 {
		t.Errorf("test-format source reported stripped fields %+v; want none, as for Markdown", test.StrippedFields)
	}
	twinAssertRebuildParity(t, test, md, test.Output, md.Output)
}

// twinShieldCounterpartExt keeps the parity normalisation from rewriting paths of the
// counterpart harness: the Markdown twin shares the counterpart's agent extension, so a path
// "<key>.md" in the counterpart's agents folder would otherwise be read as a twin agent path.
// Counterpart agent paths get the placeholder "<counterpart ext>" instead.
func twinShieldCounterpartExt(t *testing.T, res app.TransformHarnessResult) *app.TransformHarnessResult {
	t.Helper()
	cp := twinCounterpart(t).Descriptor()
	dir := cp.Paths.Agents.Project
	ext := cp.Extensions[domain.ArtifactAgent]
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal transform result: %v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("unmarshal transform result: %v", err)
	}
	shield := func(s string) string {
		if !strings.Contains(filepath.ToSlash(s), dir) {
			return s
		}
		for _, key := range owAllKeys {
			s = strings.ReplaceAll(s, key+ext, key+"<counterpart ext>")
		}
		return s
	}
	raw, err = json.Marshal(twinMapStrings(tree, shield))
	if err != nil {
		t.Fatalf("marshal shielded transform result: %v", err)
	}
	var out app.TransformHarnessResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal shielded transform result: %v", err)
	}
	return &out
}

// twinMapStrings applies f to every string of a JSON tree.
func twinMapStrings(v any, f func(string) string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = twinMapStrings(e, f)
		}
	case []any:
		for i, e := range x {
			x[i] = twinMapStrings(e, f)
		}
	case string:
		return f(x)
	}
	return v
}

// twinAssertRebuildParity fails when two rebuild results differ in anything but the byte
// encoding of the output: the written agents are compared in decoded canonical form.
func twinAssertRebuildParity(t *testing.T, test, md app.RebuildResult, testCanonical, mdCanonical []byte) {
	t.Helper()
	a, aok := parseTwinCanonical(testCanonical)
	b, bok := parseTwinCanonical(mdCanonical)
	if !aok || !bok {
		t.Fatalf("decoded rebuild output has no parseable frontmatter\ntest format\n%s\nmarkdown\n%s", testCanonical, mdCanonical)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("decoded rebuild output differs:\ntest format %+v\nmarkdown    %+v", a, b)
	}
	test.Output, md.Output = nil, nil
	if !reflect.DeepEqual(test, md) {
		t.Errorf("rebuild report differs between the formats:\ntest format %+v\nmarkdown    %+v", test, md)
	}
}

// twinDecodeOutput decodes a rebuilt twin agent file to its canonical bytes.
func twinDecodeOutput(t *testing.T, f twinFormat, out []byte) []byte {
	t.Helper()
	side := twinSide{Format: f, Module: newTestTwinHarness(t, f)}
	canonical, _, err := twinTranslator(t, side).Decode(out, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
	if err != nil {
		t.Fatalf("decode the rebuilt %s twin file: %v\n%s", side.name(), err, out)
	}
	return canonical
}
