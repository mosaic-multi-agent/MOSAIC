package transform_test

// carry_frontmatter_test.go covers the IgnoreDeployedFrontmatter and AppendTools options of
// transform.Request. IgnoreDeployedFrontmatter computes the output frontmatter as if no
// deployed file existed while body regions still come from the deployed file. AppendTools adds
// target-native tool names to the main tools field after tool resolution.

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/docformat"
	"mosaic-deploy/internal/catalog/catalogpaths"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/builtin/claudecode"
	"mosaic-deploy/internal/harness/registry"
	"mosaic-deploy/internal/transform"
)

// ignoreDeployed is a deployed file whose frontmatter holds a user-added tool, a key no
// descriptor knows and a stamp, and whose body holds a project region with user content.
const ignoreDeployed = `---
mosaic_id: 100
mosaic_version: 1.0.0
legacy_source_key: from-another-harness
tools: [alpha, beta, user-extra]
---
Body.

<Identity type="core">
## Identity

<ContextLimits type="project">
DEPLOYED-PROJECT-REGION-TEXT
</ContextLimits>

</Identity>
`

// ignoreSource is a generic source declaring the project region the deployed file filled.
const ignoreSource = `---
id: 100
version: 1.0.0
description: Ignore test agent
tools: [generic_a, generic_b]
---
Body.

<Identity type="core">
## Identity

<ContextLimits type="project">
</ContextLimits>

</Identity>
`

func ignoreRequest(t *testing.T, ignore bool) transform.Request {
	t.Helper()
	return transform.Request{
		Source:                    []byte(ignoreSource),
		Deployed:                  []byte(ignoreDeployed),
		Kind:                      domain.ArtifactAgent,
		Key:                       "ignore-agent",
		Module:                    newDescriptorModule(t, preservationListDescriptorYAML, "inline:ignore"),
		Model:                     domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:                     domain.ScopeProject,
		IgnoreDeployedFrontmatter: ignore,
	}
}

func applyAndParse(t *testing.T, req transform.Request) (*docformat.Document, string) {
	t.Helper()
	res, err := transform.Apply(req)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	doc, err := docformat.Parse(res.Output)
	if err != nil {
		t.Fatalf("parse output: %v\n%s", err, res.Output)
	}
	return doc, string(res.Output)
}

func TestIgnoreDeployedFrontmatter_DeployedUserToolNotMerged(t *testing.T) {
	doc, out := applyAndParse(t, ignoreRequest(t, true))

	if toolListContains(doc, "tools", "user-extra") {
		t.Errorf("deployed tool names must not be merged when deployed frontmatter is ignored:\n%s", out)
	}
	got := listFieldScalars(t, doc, "tools")
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want the source-resolved %v, got %v", want, got)
	}
}

func TestIgnoreDeployedFrontmatter_UntouchedDeployedKeyNotCopied(t *testing.T) {
	_, out := applyAndParse(t, ignoreRequest(t, true))

	if strings.Contains(out, "legacy_source_key") {
		t.Errorf("deployed-only frontmatter key must not be copied:\n%s", out)
	}
}

func TestIgnoreDeployedFrontmatter_BodyRegionsStillComeFromDeployed(t *testing.T) {
	_, out := applyAndParse(t, ignoreRequest(t, true))

	if !strings.Contains(out, "DEPLOYED-PROJECT-REGION-TEXT") {
		t.Errorf("project region content must still be preserved from the deployed file:\n%s", out)
	}
}

func TestIgnoreDeployedFrontmatter_Off_DeployedToolsStillPreserved(t *testing.T) {
	doc, out := applyAndParse(t, ignoreRequest(t, false))

	if !toolListContains(doc, "tools", "user-extra") {
		t.Errorf("default behaviour must keep deployed tool names:\n%s", out)
	}
}

// appendRequest builds a request on the list-shape descriptor with source tools [generic_a].
func appendRequest(t *testing.T, yaml string, appendTools []string) transform.Request {
	t.Helper()
	return transform.Request{
		Source:      []byte(preservationSourceAB),
		Kind:        domain.ArtifactAgent,
		Key:         "append-agent",
		Module:      newDescriptorModule(t, yaml, "inline:append"),
		Model:       domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:       domain.ScopeProject,
		AppendTools: appendTools,
	}
}

func TestAppendTools_List_AppendedAfterResolvedTools_InOrder(t *testing.T) {
	doc, _ := applyAndParse(t, appendRequest(t, preservationListDescriptorYAML, []string{"native-one", "native-two"}))

	got := listFieldScalars(t, doc, "tools")
	if want := []string{"alpha", "beta", "native-one", "native-two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want %v, got %v", want, got)
	}
}

func TestAppendTools_List_NameAlreadyPresentNotRepeated(t *testing.T) {
	doc, _ := applyAndParse(t, appendRequest(t, preservationListDescriptorYAML, []string{"beta", "native-one", "native-one"}))

	got := listFieldScalars(t, doc, "tools")
	if want := []string{"alpha", "beta", "native-one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want %v, got %v", want, got)
	}
}

func TestAppendTools_Nil_AddsNothing(t *testing.T) {
	doc, _ := applyAndParse(t, appendRequest(t, preservationListDescriptorYAML, nil))

	got := listFieldScalars(t, doc, "tools")
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want %v, got %v", want, got)
	}
}

func TestAppendTools_NotReportedInReportTools(t *testing.T) {
	res, err := transform.Apply(appendRequest(t, preservationListDescriptorYAML, []string{"native-one"}))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	doc, err := docformat.Parse(res.Output)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if !toolListContains(doc, "tools", "native-one") {
		t.Errorf("native-one must be appended to the tools field:\n%s", res.Output)
	}
	if len(res.Report.Tools) != 2 {
		t.Errorf("Report.Tools must list only the two resolved tools, got %d: %+v", len(res.Report.Tools), res.Report.Tools)
	}
}

func TestAppendTools_Permission_AddsNameAsAllow(t *testing.T) {
	doc, out := applyAndParse(t, appendRequest(t, preservationPermissionDescriptorYAML, []string{"native-one"}))

	if !permissionToolContains(doc, "tools", "native-one") {
		t.Errorf("permission shape must get \"native-one: allow\":\n%s", out)
	}
	if !permissionToolContains(doc, "tools", "alpha") {
		t.Errorf("resolved tool alpha must stay allowed:\n%s", out)
	}
}

func TestAppendTools_Permission_ExistingDeniedPairSetToAllow(t *testing.T) {
	// The source allows only alpha, so beta is an existing denied pair.
	req := appendRequest(t, preservationPermissionDescriptorYAML, []string{"beta"})
	req.Source = []byte(strings.Replace(preservationPermissionSource, "[generic_a, generic_b]", "[generic_a]", 1))

	doc, out := applyAndParse(t, req)

	if !permissionToolContains(doc, "tools", "beta") {
		t.Errorf("an existing denied pair named in AppendTools must become allow:\n%s", out)
	}
	if n := strings.Count(out, "beta:"); n != 1 {
		t.Errorf("no second pair may be emitted for beta, got %d occurrences:\n%s", n, out)
	}
}

func TestAppendTools_AfterPlaceholderExpansion(t *testing.T) {
	req := appendRequest(t, placeholderNoExpansionDescriptorYAML, []string{"native-one", "alpha"})
	req.Source = []byte(preservationPlaceholderSource)

	doc, out := applyAndParse(t, req)

	got := listFieldScalars(t, doc, "tools")
	if len(got) == 0 || got[len(got)-1] != "native-one" {
		t.Errorf("native-one must follow the placeholder expansion, got %v\n%s", got, out)
	}
	seen := map[string]int{}
	for _, name := range got {
		seen[name]++
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("tool %q appears %d times: %v", name, n, got)
		}
	}
}

// scalarToolsRequest builds an AppendTools request on the real Claude Code module, whose
// tools field is a comma-separated scalar. The source declares file_read and content_search.
func scalarToolsRequest(t *testing.T, appendTools []string) transform.Request {
	t.Helper()
	catalogRoot := filepath.Join("..", "..", "testdata", "frozen-catalog", "Catalog")
	mod, err := claudecode.New(registry.BuiltinOptions{ContentDir: func(dir string) (string, error) {
		return catalogpaths.HarnessInjectionsDir(catalogRoot, dir), nil
	}})
	if err != nil {
		t.Fatalf("build claude-code module: %v", err)
	}
	src := "---\nid: 100\nversion: 1.0.0\ndescription: Scalar append agent\ntools: [file_read, content_search]\n---\nBody.\n"
	return transform.Request{
		Source:      []byte(src),
		Kind:        domain.ArtifactAgent,
		Key:         "scalar-agent",
		Module:      mod,
		Model:       domain.ModelSelection{Origin: domain.OriginUnresolved},
		Scope:       domain.ScopeProject,
		AppendTools: appendTools,
	}
}

func scalarToolNames(t *testing.T, doc *docformat.Document) []string {
	t.Helper()
	v, ok := doc.Frontmatter().Get("tools")
	if !ok || v.Kind != domain.KindScalar {
		t.Fatalf("tools must be a comma scalar, got present=%v kind=%v", ok, v.Kind)
	}
	var names []string
	for _, part := range strings.Split(v.Scalar, ",") {
		names = append(names, strings.TrimSpace(part))
	}
	return names
}

func TestAppendTools_CommaScalar_NameIsOneMoreItem(t *testing.T) {
	doc, out := applyAndParse(t, scalarToolsRequest(t, []string{"native-one"}))

	got := scalarToolNames(t, doc)
	if want := []string{"Read", "Grep", "native-one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want %v, got %v\n%s", want, got, out)
	}
}

func TestAppendTools_CommaScalar_NameAlreadyPresentNotRepeated(t *testing.T) {
	doc, out := applyAndParse(t, scalarToolsRequest(t, []string{"Grep", "native-one", "native-one"}))

	got := scalarToolNames(t, doc)
	if want := []string{"Read", "Grep", "native-one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("tools: want %v, got %v\n%s", want, got, out)
	}
}
