package app_test

// deployed_tools_helpers_test.go holds the shared fixtures and assertions of the deployed
// tool conversion tests: built-in descriptors loaded from their YAML, an input builder with
// option overrides, and slice/report comparison helpers.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

const (
	harnessClaude = "claudecode/claude-code.yaml"
	harnessOpen   = "opencode/opencode.yaml"
	harnessCLI    = "ghcpcli/ghcp-cli.yaml"
	harnessVSCode = "vscodeghcp/vscode-ghcp.yaml"
)

// loadToolHarness loads a built-in harness descriptor from its YAML file.
func loadToolHarness(t *testing.T, rel string) *domain.HarnessDescriptor {
	t.Helper()
	d, err := descriptor.Load("../harness/builtin/" + rel)
	if err != nil {
		t.Fatalf("load %s: %v", rel, err)
	}
	return d
}

// toolInputOpt adjusts a DeployedToolsInput.
type toolInputOpt func(*app.DeployedToolsInput)

func withRole(r domain.AgentRole) toolInputOpt {
	return func(in *app.DeployedToolsInput) { in.Role = r }
}

func withPlaceholderAgent() toolInputOpt {
	return func(in *app.DeployedToolsInput) { in.PlaceholderAgent = true }
}

func withCatalogTools(tools []string) toolInputOpt {
	return func(in *app.DeployedToolsInput) { in.CatalogTools = tools }
}

// newToolInput builds a subagent input converting tools from the source to the target harness.
func newToolInput(t *testing.T, srcRel, tgtRel string, tools []string, opts ...toolInputOpt) app.DeployedToolsInput {
	t.Helper()
	in := app.DeployedToolsInput{
		Source: loadToolHarness(t, srcRel),
		Target: loadToolHarness(t, tgtRel),
		Tools:  allowToolGrants(tools),
		Role:   domain.RoleSubagent,
	}
	for _, o := range opts {
		o(&in)
	}
	return in
}

// allowToolGrants wraps names as allow grants.
func allowToolGrants(names []string) []domain.ToolGrant {
	grants := make([]domain.ToolGrant, len(names))
	for i, n := range names {
		grants[i] = domain.ToolGrant{Name: n, Disposition: domain.Allow}
	}
	return grants
}

// allowGrant and denyGrant build one grant.
func allowGrant(name string) domain.ToolGrant {
	return domain.ToolGrant{Name: name, Disposition: domain.Allow}
}

func denyGrant(name string) domain.ToolGrant {
	return domain.ToolGrant{Name: name, Disposition: domain.Deny}
}

// withDeclaredServers sets the declared servers of the source MCP configuration.
func withDeclaredServers(servers ...string) toolInputOpt {
	return func(in *app.DeployedToolsInput) { in.DeclaredServers = servers }
}

// withSplitAnswers sets the answered splits (source name -> server).
func withSplitAnswers(answers map[string]string) toolInputOpt {
	return func(in *app.DeployedToolsInput) { in.SplitAnswers = answers }
}

// convertGrants runs ConvertDeployedTools on grants (allow and deny) of the source harness.
func convertGrants(t *testing.T, srcRel, tgtRel string, grants []domain.ToolGrant, opts ...toolInputOpt) app.ConvertedTools {
	t.Helper()
	in := newToolInput(t, srcRel, tgtRel, nil, opts...)
	in.Tools = grants
	for _, o := range opts {
		o(&in)
	}
	return app.ConvertDeployedTools(in)
}

// convertTools runs ConvertDeployedTools on a freshly built input.
func convertTools(t *testing.T, srcRel, tgtRel string, tools []string, opts ...toolInputOpt) app.ConvertedTools {
	t.Helper()
	return app.ConvertDeployedTools(newToolInput(t, srcRel, tgtRel, tools, opts...))
}

// assertStrings compares two string slices, treating nil and empty as equal.
func assertStrings(t *testing.T, what string, want, got []string) {
	t.Helper()
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("%s: want %v, got %v", what, want, got)
	}
}

// withSkill returns the expected Generic list for a non-orchestrator converted from a
// harness that grants skills implicitly (Claude Code, VS Code GHCP) to one that writes
// skill access explicitly (OpenCode, GHCP CLI): the listed generics plus a trailing skill.
func withSkill(generics ...string) []string {
	return append(append([]string(nil), generics...), "skill")
}

// assertCustomTools compares the custom tool map, treating nil and empty as equal.
func assertCustomTools(t *testing.T, want, got map[string]string) {
	t.Helper()
	if len(want) == 0 && len(got) == 0 {
		return
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("CustomTools: want %v, got %v", want, got)
	}
}

// droppedNames returns the names of the dropped tools in report order.
func droppedNames(c app.ConvertedTools) []string {
	names := make([]string, 0, len(c.Dropped))
	for _, d := range c.Dropped {
		names = append(names, d.Name)
	}
	return names
}

// findConversion returns the conversion entry of a source name.
func findConversion(t *testing.T, c app.ConvertedTools, source string) app.ToolConversion {
	t.Helper()
	for _, cv := range c.Conversions {
		if cv.Source == source {
			return cv
		}
	}
	t.Fatalf("no conversion entry for %q; entries: %+v", source, c.Conversions)
	return app.ToolConversion{}
}

// assertRule asserts the conversion entry of a source name carries the given rule.
func assertRule(t *testing.T, c app.ConvertedTools, source string, rule app.ToolConversionRule) {
	t.Helper()
	if got := findConversion(t, c, source); got.Rule != rule {
		t.Errorf("rule for %q: want %q, got %q", source, rule, got.Rule)
	}
}

// assertConversionSources asserts the conversion entries list exactly these source names, in order.
func assertConversionSources(t *testing.T, c app.ConvertedTools, want []string) {
	t.Helper()
	got := make([]string, 0, len(c.Conversions))
	for _, cv := range c.Conversions {
		got = append(got, cv.Source)
	}
	assertStrings(t, "Conversions sources", want, got)
}

// syntheticHarness returns a minimal list-shape descriptor with the given universe
// names and mapping destinations (generic -> main destination names).
func syntheticHarness(id string, universe []string, mappings map[string][]string, order []string) *domain.HarnessDescriptor {
	d := &domain.HarnessDescriptor{ID: id, Tools: domain.ToolSpec{Shape: domain.ShapeList}}
	for _, n := range universe {
		d.Tools.Universe = append(d.Tools.Universe, domain.HarnessTool{Name: n})
	}
	for _, g := range order {
		d.Tools.Mappings = append(d.Tools.Mappings, domain.ToolMapping{
			Generic:      g,
			Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: mappings[g]}},
		})
	}
	return d
}
