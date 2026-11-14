package app_test

// thirdformat_promote_parity_test.go specifies that Promote of a harness-only agent file behaves
// identically when the file is in the test-only agent format and when it is Markdown: the same
// key (derived by stripping the format's own extension), destination, tool conversion, stripped
// fields and the same generic file written. Each twin has its own MOSAIC root.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

// twinPromoteKey is the agent key of the promoted fixture.
const twinPromoteKey = "code-reviewer"

// twinPromoteRoots holds the MOSAIC root of each twin side.
type twinPromoteRoots map[twinFormat]string

// twinPromotePair builds both twins, each over its own writable MOSAIC root and catalog.
func twinPromotePair(t *testing.T) (twinPair, twinPromoteRoots) {
	t.Helper()
	roots := twinPromoteRoots{}
	perSide := func(f twinFormat, _ string) func(*app.Deps) {
		root := writeMosaicRoot(t)
		roots[f] = root
		return func(d *app.Deps) {
			d.MosaicRoot = root
			d.Catalog = &stubCatalog{root: root, catalogRoot: filepath.Join(root, "Catalog")}
		}
	}
	return newTestTwinPairWith(t, nil, perSide), roots
}

// twinPromoteStep returns the Promote step of the opencode-shaped fixture, written in the
// side's own agent format, optionally as a dry run.
func twinPromoteStep(roots twinPromoteRoots, dryRun bool) twinFlowStep {
	return func(t *testing.T, side twinSide) (twinFlowResult, error) {
		t.Helper()
		raw, _, err := twinTranslator(t, side).Encode(rbFixture(t, hOpen+"-full.md"), agentformat.ArtifactContext{
			AgentKey: twinPromoteKey, Kind: domain.ArtifactAgent, Op: agentformat.OpCreate})
		if err != nil {
			t.Fatalf("encode the promote source in the %s twin format: %v", side.name(), err)
		}
		src := filepath.Join(t.TempDir(), twinPromoteKey+twinExtOf(side.Format))
		if err := os.WriteFile(src, raw, 0o644); err != nil {
			t.Fatalf("write promote source: %v", err)
		}
		res, err := side.Service.Promote(context.Background(), app.PromoteRequest{
			FilePath: src, Category: "TestCategory", HarnessID: twinHarnessID, DryRun: dryRun})
		if err != nil {
			return twinFlowResult{}, err
		}
		out := twinFlowResult{Promote: &res}
		out.Promote.SourcePath = twinPromoteKey + twinExtPlaceholder
		dest := res.DestinationPath
		if filepath.IsAbs(dest) {
			if rel, relErr := filepath.Rel(roots[side.Format], dest); relErr == nil {
				out.Promote.DestinationPath = filepath.ToSlash(rel)
			}
		} else {
			dest = filepath.Join(roots[side.Format], filepath.FromSlash(dest))
		}
		if !dryRun {
			if out.PromotedFile, err = os.ReadFile(dest); err != nil {
				t.Fatalf("read the promoted file on the %s twin: %v", side.name(), err)
			}
		}
		return out, nil
	}
}

func TestTwinParity_Promote_TestFormatSourceEqualsMarkdownSource(t *testing.T) {
	// Arrange
	p, roots := twinPromotePair(t)

	// Act
	test, md := runTwinFlowStep(t, p, twinPromoteStep(roots, false))

	// Assert
	assertTwinParity(t, "promote of a harness-only agent", test, md)
	if got := test.Flow.Promote.Key; got != twinPromoteKey {
		t.Errorf("key = %q; want %q (the format's own extension is stripped)", got, twinPromoteKey)
	}
	if len(md.Flow.Promote.Tools) == 0 {
		t.Error("the fixture yields no promoted tools; the tool conversion comparison would be vacuous")
	}
}

func TestTwinParity_Promote_WritesTheSameGenericFile(t *testing.T) {
	// Arrange
	p, roots := twinPromotePair(t)

	// Act
	test, md := runTwinFlowStep(t, p, twinPromoteStep(roots, false))

	// Assert
	if len(md.Flow.PromotedFile) == 0 {
		t.Fatal("markdown twin wrote no promoted file")
	}
	if string(test.Flow.PromotedFile) != string(md.Flow.PromotedFile) {
		t.Errorf("promoted generic file differs:\ntest format\n%s\nmarkdown\n%s", test.Flow.PromotedFile, md.Flow.PromotedFile)
	}
}

func TestTwinParity_Promote_DryRunReportEqualsMarkdownSource(t *testing.T) {
	// Arrange
	p, roots := twinPromotePair(t)

	// Act
	test, md := runTwinFlowStep(t, p, twinPromoteStep(roots, true))

	// Assert
	assertTwinParity(t, "promote dry run", test, md)
	if !test.Flow.Promote.DryRun {
		t.Error("dry run result does not report DryRun")
	}
}

func TestTwinParity_Promote_StrippedFieldsEqualMarkdownSource(t *testing.T) {
	// Arrange
	p, roots := twinPromotePair(t)

	// Act
	test, md := runTwinFlowStep(t, p, twinPromoteStep(roots, false))

	// Assert: regression coverage; the Markdown side must report the fixture's stripped field
	// (mode) so the comparison is never vacuous.
	if len(md.Flow.Promote.StrippedFields) == 0 {
		t.Fatal("markdown promote reports no stripped fields; the comparison would be vacuous")
	}
	if !reflect.DeepEqual(test.Flow.Promote.StrippedFields, md.Flow.Promote.StrippedFields) {
		t.Errorf("stripped fields differ: test format %+v, markdown %+v",
			test.Flow.Promote.StrippedFields, md.Flow.Promote.StrippedFields)
	}
}
