package app_test

// rebuild_codex_target_test.go specifies the deployed-agent rebuild when the TARGET is the
// Codex harness (TOML agent files). The rebuild request carries an explicit operation and, on
// overwrite, the existing destination file's bytes; the source stays a Markdown deployed file.
// The result is assembled from the written TOML, not parsed as Markdown.
//
// Chosen behaviour for user-owned TOML keys of an existing destination: they are preserved
// when the destination is overwritten, as on every other Codex write path.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const (
	codexUserKey   = "user_note"
	codexUserValue = "keep-me-please"
	// codexOldInstructions is the instruction text of the destination file before overwrite;
	// it is distinct from every text of the Markdown source fixture.
	codexOldInstructions = "OLD-DESTINATION-INSTRUCTIONS"
)

// existingCodexDestination is a deployed Codex agent file with a user-owned key and
// instructions that differ from the Markdown source.
func existingCodexDestination() []byte {
	return []byte("# mosaic_version: 0.9\n\n" +
		"sandbox_mode = \"read-only\"\n" +
		codexUserKey + " = \"" + codexUserValue + "\"\n" +
		"developer_instructions = \"" + codexOldInstructions + "\\n\"\n")
}

// codexTargetInput builds a rebuild of the Claude Code fixture into the real Codex module.
func codexTargetInput(t *testing.T, op agentformat.Operation, dest []byte, opts ...rbOpt) app.RebuildInput {
	t.Helper()
	opts = append([]rbOpt{withModelSelection(codexAppModel)}, opts...)
	in := rbInput(t, rbPair{"claude-to-codex", hClaude, hClaude}, false, opts...)
	in.TargetModule = codexAppModule(t)
	in.Op = op
	in.DestinationRaw = dest
	return in
}

// decodeCodexOutput decodes written Codex TOML to the canonical Markdown document.
func decodeCodexOutput(t *testing.T, out []byte) []byte {
	t.Helper()
	tr, err := agentformat.LookupString("codex-toml")
	if err != nil {
		t.Fatalf("lookup codex-toml: %v", err)
	}
	canonical, _, err := tr.Decode(out, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
	if err != nil {
		t.Fatalf("decode written codex file: %v\n%s", err, out)
	}
	return canonical
}

// codexInstructions returns the developer_instructions of written Codex TOML.
func codexInstructions(t *testing.T, out []byte) string {
	t.Helper()
	s, _ := parseTomlMap(t, out)["developer_instructions"].(string)
	return s
}

func TestRebuildToCodex_Create_WritesTomlWithSourceInstructions(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpCreate, nil)

	res, err := app.RebuildDeployedAgent(in)

	if err != nil {
		t.Fatalf("RebuildDeployedAgent to Codex on create: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(res.Output)), "---") {
		t.Fatalf("Codex output must be TOML, not Markdown frontmatter:\n%s", res.Output)
	}
	if got := codexInstructions(t, res.Output); !strings.Contains(got, "BODY-IDENTITY-TEXT") ||
		!strings.Contains(got, "CUSTOM-REGION-TEXT") {
		t.Errorf("developer_instructions lacks the source body and custom region:\n%s", got)
	}
}

func TestRebuildToCodex_Create_DecodesToOwnedKeysOfTheSource(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpCreate, nil)

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	doc, perr := docformat.Parse(decodeCodexOutput(t, res.Output))
	if perr != nil {
		t.Fatalf("parse decoded canonical: %v", perr)
	}
	fm := doc.Frontmatter()
	if v, _ := fm.Get("name"); v.Scalar != "code-reviewer" {
		t.Errorf("decoded name = %q, want code-reviewer", v.Scalar)
	}
	if v, _ := fm.Get("model"); v.Scalar != codexAppModel.ModelID {
		t.Errorf("decoded model = %q, want %q", v.Scalar, codexAppModel.ModelID)
	}
}

func TestRebuildToCodex_Create_ReportsNoToolsAndDroppedSourceFields(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpCreate, nil)

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	if len(res.Tools) != 0 {
		t.Errorf("Tools = %v, want none: a Codex file has no tools key", res.Tools)
	}
	// DroppedFields lists exactly the source keys absent from the decoded written file.
	doc, perr := docformat.Parse(decodeCodexOutput(t, res.Output))
	if perr != nil {
		t.Fatalf("parse decoded canonical: %v", perr)
	}
	written := map[string]bool{}
	for _, k := range doc.Frontmatter().Keys() {
		written[k] = true
	}
	srcDoc, _ := docformat.Parse(in.Source)
	var want []string
	for _, k := range srcDoc.Frontmatter().Keys() {
		if !written[k] {
			want = append(want, k)
		}
	}
	if !equalStrings(sortedCopy(res.DroppedFields...), sortedCopy(want...)) {
		t.Errorf("DroppedFields = %v, want the source keys absent from the written file: %v", res.DroppedFields, want)
	}
	if !containsString(res.DroppedFields, "tools") {
		t.Errorf("DroppedFields = %v, want tools (Codex expresses tool access as sandbox_mode)", res.DroppedFields)
	}
}

func TestRebuildToCodex_Create_ToolConversionMatchesMarkdownTarget(t *testing.T) {
	codexRes, err := app.RebuildDeployedAgent(codexTargetInput(t, agentformat.OpCreate, nil))
	if err != nil {
		t.Fatalf("RebuildDeployedAgent to Codex: %v", err)
	}

	mdRes, _ := rbRebuild(t, rbInput(t, claudeToOpencode, false))

	if len(mdRes.Conversions) == 0 {
		t.Fatal("fixture yields no tool conversions; the parity check would be vacuous")
	}
	if !reflect.DeepEqual(codexRes.Conversions, mdRes.Conversions) || !reflect.DeepEqual(codexRes.Dropped, mdRes.Dropped) {
		t.Errorf("source-side tool conversion differs by target:\ncodex %+v / %+v\nmarkdown %+v / %+v",
			codexRes.Conversions, codexRes.Dropped, mdRes.Conversions, mdRes.Dropped)
	}
}

func TestRebuildToCodex_Create_StampsCarryTargetVersions(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: codexAppModule(t), orchVersion: stampOrchVersion}
	in := codexTargetInput(t, agentformat.OpCreate, nil)
	in.TargetModule = mod

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	if res.Stamps.Version != "2.3.0" || res.Stamps.InjectionsVersion != stampSharedVersion {
		t.Errorf("stamps = %+v, want source version 2.3.0 and injections version %s", res.Stamps, stampSharedVersion)
	}
}

func TestRebuildToCodex_Orchestrator_RegionTagsCarryPerRegionVersions(t *testing.T) {
	mod := &rebuildRegionModule{HarnessModule: codexAppModule(t), orchText: stampOrchText, orchOK: true, orchVersion: stampOrchVersion}
	in := codexTargetInput(t, agentformat.OpCreate, nil, withFileName("orchestrator.md"), withoutRoleStamp, withSecondHarnessRegion)
	in.TargetModule = mod

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	assertRegionStamps(t, "codex rebuild", decodeCodexOutput(t, res.Output))
}

func TestRebuildToCodex_Overwrite_UpdatesWithoutOperationError(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpUpdate, existingCodexDestination())

	res, err := app.RebuildDeployedAgent(in)

	if err != nil {
		t.Fatalf("overwrite of an existing Codex destination failed: %v", err)
	}
	got := codexInstructions(t, res.Output)
	if !strings.Contains(got, "BODY-IDENTITY-TEXT") || strings.Contains(got, codexOldInstructions) {
		t.Errorf("overwrite must carry the SOURCE instructions, not the destination's:\n%s", got)
	}
}

func TestRebuildToCodex_Overwrite_PreservesUserOwnedDestinationKey(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpUpdate, existingCodexDestination())

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	if got := parseTomlMap(t, res.Output)[codexUserKey]; got != codexUserValue {
		t.Errorf("user-owned key %q = %v, want %q preserved from the existing destination\n%s",
			codexUserKey, got, codexUserValue, res.Output)
	}
}

func TestRebuildToCodex_Overwrite_ResultIsAssembledFromTomlOutput(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpUpdate, existingCodexDestination())

	res, err := app.RebuildDeployedAgent(in)
	if err != nil {
		t.Fatalf("RebuildDeployedAgent: %v", err)
	}

	if res.Role != domain.RoleSubagent || len(res.Tools) != 0 || !containsString(res.DroppedFields, "tools") {
		t.Errorf("result = role %q tools %v dropped %v, want subagent, no tools, tools dropped",
			res.Role, res.Tools, res.DroppedFields)
	}
}

func TestRebuildToCodex_Overwrite_WithoutDestinationBytesIsMissingPriorBytes(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpUpdate, nil)

	_, err := app.RebuildDeployedAgent(in)

	if !errors.Is(err, agentformat.ErrMissingPriorBytes) {
		t.Errorf("error = %v, want the operation passed through so the encoder reports ErrMissingPriorBytes", err)
	}
}

func TestRebuildToCodex_Overwrite_UndecodableDestinationIsMalformedDeployed(t *testing.T) {
	in := codexTargetInput(t, agentformat.OpUpdate, []byte("this is = = not [valid toml\n"))

	_, err := app.RebuildDeployedAgent(in)

	if !errors.Is(err, agentformat.ErrMalformedDeployed) {
		t.Errorf("error = %v, want ErrMalformedDeployed for an undecodable destination", err)
	}
}

// Markdown targets ignore the operation and the destination bytes: regression coverage.
func TestRebuildToMarkdown_OpAndDestinationBytesDoNotChangeOutput(t *testing.T) {
	_, plain := rbRebuild(t, rbInput(t, claudeToOpencode, false))
	in := rbInput(t, claudeToOpencode, false)
	in.Op = agentformat.OpUpdate
	in.DestinationRaw = []byte("anything at all, not even Markdown\n")

	_, withDest := rbRebuild(t, in)

	if plain != withDest {
		t.Errorf("Markdown target output changed with Op/DestinationRaw set:\n%s\n---\n%s", plain, withDest)
	}
}
