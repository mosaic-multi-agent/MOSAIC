package config_test

// toolsettings_stamp_test.go verifies the tool settings stamp: a hash of the harness's
// effective tool spec and its merged tool_destinations. It changes when any tool data that
// shapes a written tools field changes, in the descriptor or in the configuration, and is equal
// otherwise.

import (
	"regexp"
	"testing"

	"mosaic-deploy/internal/config"
	"mosaic-deploy/internal/domain"
)

const stampHarness = "stamp-harness"

var stampFormat = regexp.MustCompile(`^[0-9a-f]{64}$`)

// stampSpec returns a fully populated tool spec; every call returns an independent value.
func stampSpec() domain.ToolSpec {
	return domain.ToolSpec{
		Shape: domain.ShapeList,
		Universe: []domain.HarnessTool{
			{Name: "Read", Unused: domain.Deny, Action: "read"},
			{Name: "Edit", Unused: domain.Allow, ByConvention: true},
			{Name: "AskUserQuestion", Unused: domain.Deny},
		},
		Mappings: []domain.ToolMapping{
			mainMapping("file_read", "Read"),
			{Generic: "web", Destinations: []domain.ToolDestination{
				{Kind: domain.DestField, Field: "web_tools", Format: domain.FormatScalar, Separator: ";", Names: []string{"fetch"}},
			}},
		},
		MCP: domain.MCPNameForms{
			ServerForm: "mcp__{server}__*", ToolForm: "mcp__{server}__{tool}",
			ExtraServerReadForms: []string{"mcp__{server}"}, NormalizeServerName: true,
		},
		ReportUnknownNames:     true,
		CustomToolDestinations: []domain.ToolDestination{{Kind: domain.DestMain}},
		PlaceholderExpansion:   []string{"Read", "AskUserQuestion"},
		Rules: domain.RuleFrame{
			Leading:  []domain.PermissionRule{{Action: "*", Resource: "*", Effect: domain.RuleEffect("deny")}},
			Trailing: []domain.PermissionRule{{Action: "read", Resource: "*", Effect: domain.RuleEffect("allow")}},
		},
	}
}

// stampFor calls config.ToolSettingsStamp and fails the test, instead of aborting the whole
// test binary, when the call panics.
func stampFor(t *testing.T, harnessID string, tools domain.ToolSpec, project, user config.ToolDestinationsByHarness) (stamp string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ToolSettingsStamp panicked: %v", r)
		}
	}()
	return config.ToolSettingsStamp(harnessID, tools, project, user)
}

func stampOf(t *testing.T, tools domain.ToolSpec) string {
	return stampFor(t, stampHarness, tools, nil, nil)
}

func TestToolSettingsStamp_AnyToolSpecFieldChange_ChangesStamp(t *testing.T) {
	mutations := map[string]func(*domain.ToolSpec){
		"shape":                       func(s *domain.ToolSpec) { s.Shape = domain.ShapePermission },
		"universe tool added":         func(s *domain.ToolSpec) { s.Universe = append(s.Universe, domain.HarnessTool{Name: "Bash"}) },
		"universe tool removed":       func(s *domain.ToolSpec) { s.Universe = s.Universe[:2] },
		"universe name":               func(s *domain.ToolSpec) { s.Universe[0].Name = "Read2" },
		"universe unused disposition": func(s *domain.ToolSpec) { s.Universe[2].Unused = domain.Allow },
		"universe by convention":      func(s *domain.ToolSpec) { s.Universe[0].ByConvention = true },
		"universe action":             func(s *domain.ToolSpec) { s.Universe[0].Action = "view" },
		"universe order":              func(s *domain.ToolSpec) { s.Universe[0], s.Universe[1] = s.Universe[1], s.Universe[0] },
		"mapping removed":             func(s *domain.ToolSpec) { s.Mappings = s.Mappings[1:] },
		"mapping generic":             func(s *domain.ToolSpec) { s.Mappings[0].Generic = "file_view" },
		"mapping order":               func(s *domain.ToolSpec) { s.Mappings[0], s.Mappings[1] = s.Mappings[1], s.Mappings[0] },
		"destination kind":            func(s *domain.ToolSpec) { s.Mappings[0].Destinations[0].Kind = domain.DestField },
		"destination field":           func(s *domain.ToolSpec) { s.Mappings[1].Destinations[0].Field = "other" },
		"destination format":          func(s *domain.ToolSpec) { s.Mappings[1].Destinations[0].Format = domain.FormatListFlow },
		"destination separator":       func(s *domain.ToolSpec) { s.Mappings[1].Destinations[0].Separator = "," },
		"destination names":           func(s *domain.ToolSpec) { s.Mappings[0].Destinations[0].Names = []string{"Read", "View"} },
		"mcp server form":             func(s *domain.ToolSpec) { s.MCP.ServerForm = "{server}/*" },
		"mcp tool form":               func(s *domain.ToolSpec) { s.MCP.ToolForm = "{server}/{tool}" },
		"mcp extra read forms":        func(s *domain.ToolSpec) { s.MCP.ExtraServerReadForms = nil },
		"mcp normalize server name":   func(s *domain.ToolSpec) { s.MCP.NormalizeServerName = false },
		"report unknown names":        func(s *domain.ToolSpec) { s.ReportUnknownNames = false },
		"custom tool destinations":    func(s *domain.ToolSpec) { s.CustomToolDestinations = nil },
		"placeholder expansion":       func(s *domain.ToolSpec) { s.PlaceholderExpansion = []string{"Read"} },
		"rules leading":               func(s *domain.ToolSpec) { s.Rules.Leading[0].Effect = domain.RuleEffect("allow") },
		"rules trailing":              func(s *domain.ToolSpec) { s.Rules.Trailing = nil },
	}
	base := stampOf(t, stampSpec())
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			spec := stampSpec()

			mutate(&spec)

			if got := stampOf(t, spec); got == base {
				t.Errorf("stamp did not change when %s changed", name)
			}
		})
	}
}

func TestToolSettingsStamp_EqualInputs_GiveEqualStamp(t *testing.T) {
	project := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "a")}}
	user := config.ToolDestinationsByHarness{stampHarness: {mainMapping("h", "b")}}

	first := stampFor(t, stampHarness, stampSpec(), project, user)
	second := stampFor(t, stampHarness, stampSpec(), project, user)

	if first != second {
		t.Errorf("equal inputs gave %q and %q", first, second)
	}
}

func TestToolSettingsStamp_ConfigDestinationsChange_ChangesStamp(t *testing.T) {
	spec := stampSpec()
	none := stampFor(t, stampHarness, spec, nil, nil)
	cases := map[string]func(*testing.T) string{
		"project declares a mapping": func(t *testing.T) string {
			return stampFor(t, stampHarness, spec,
				config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "a")}}, nil)
		},
		"user declares a mapping": func(t *testing.T) string {
			return stampFor(t, stampHarness, spec, nil,
				config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "a")}})
		},
		"project mapping names change": func(t *testing.T) string {
			return stampFor(t, stampHarness, spec,
				config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "b")}}, nil)
		},
	}
	seen := map[string]string{}
	for name, stamp := range cases {
		t.Run(name, func(t *testing.T) {
			got := stamp(t)
			if got == none {
				t.Errorf("stamp did not change when config declares tool_destinations (%s)", name)
			}
			seen[name] = got
		})
	}
	if a, b := seen["project declares a mapping"], seen["project mapping names change"]; a != "" && a == b {
		t.Error("stamp did not change when the names of a config mapping changed")
	}
}

func TestToolSettingsStamp_UserMappingOverridesProjectPerGenericTool(t *testing.T) {
	project := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "project-name")}}
	user := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "user-name")}}
	userOnly := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "user-name")}}

	merged := stampFor(t, stampHarness, stampSpec(), project, user)
	want := stampFor(t, stampHarness, stampSpec(), nil, userOnly)

	if merged != want {
		t.Error("project and user declare one generic tool: the user's mapping must win, giving the stamp of the user mapping alone")
	}
}

func TestToolSettingsStamp_OtherHarnessDeclarations_DoNotAffectStamp(t *testing.T) {
	base := stampFor(t, stampHarness, stampSpec(), nil, nil)
	other := config.ToolDestinationsByHarness{"another-harness": {mainMapping("g", "a")}}

	got := stampFor(t, stampHarness, stampSpec(), other, other)

	if got != base {
		t.Error("tool_destinations of another harness changed this harness's stamp")
	}
}

func TestToolSettingsStamp_HarnessIDIsNotHashed(t *testing.T) {
	spec := stampSpec()

	a := stampFor(t, "harness-a", spec, nil, nil)
	b := stampFor(t, "harness-b", spec, nil, nil)

	if a != b {
		t.Error("the harness id changed the stamp although no config entry is selected by it")
	}
}

func TestToolSettingsStamp_NilAndEmptyInputs_GiveEqualStamp(t *testing.T) {
	nilSpec := domain.ToolSpec{Shape: domain.ShapeList}
	emptySpec := domain.ToolSpec{
		Shape: domain.ShapeList, Universe: []domain.HarnessTool{}, Mappings: []domain.ToolMapping{},
		CustomToolDestinations: []domain.ToolDestination{}, PlaceholderExpansion: []string{},
		MCP:   domain.MCPNameForms{ExtraServerReadForms: []string{}},
		Rules: domain.RuleFrame{Leading: []domain.PermissionRule{}, Trailing: []domain.PermissionRule{}},
	}

	withNil := stampFor(t, stampHarness, nilSpec, nil, nil)
	withEmpty := stampFor(t, stampHarness, emptySpec,
		config.ToolDestinationsByHarness{}, config.ToolDestinationsByHarness{stampHarness: {}})

	if withNil != withEmpty {
		t.Errorf("nil and empty inputs gave different stamps %q and %q", withNil, withEmpty)
	}
}

func TestToolSettingsStamp_Result_IsNeverEmptyAndIs64LowercaseHexChars(t *testing.T) {
	cases := map[string]string{
		"zero spec, no config": stampFor(t, stampHarness, domain.ToolSpec{}, nil, nil),
		"full spec, no config": stampFor(t, stampHarness, stampSpec(), nil, nil),
		"full spec, config": stampFor(t, stampHarness, stampSpec(),
			config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "a")}}, nil),
	}
	for name, got := range cases {
		if !stampFormat.MatchString(got) {
			t.Errorf("%s: stamp %q is not 64 lowercase hexadecimal characters", name, got)
		}
	}
}

func TestToolSettingsStamp_DoesNotMutateItsArguments(t *testing.T) {
	spec := stampSpec()
	project := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "a")}}
	user := config.ToolDestinationsByHarness{stampHarness: {mainMapping("g", "b"), mainMapping("h", "c")}}

	stampFor(t, stampHarness, spec, project, user)

	if len(project[stampHarness]) != 1 || project[stampHarness][0].Destinations[0].Names[0] != "a" {
		t.Errorf("project declarations were modified: %+v", project)
	}
	if len(user[stampHarness]) != 2 || user[stampHarness][0].Destinations[0].Names[0] != "b" {
		t.Errorf("user declarations were modified: %+v", user)
	}
	if want := stampSpec(); len(spec.Universe) != len(want.Universe) || len(spec.Mappings) != len(want.Mappings) {
		t.Error("the tool spec was modified")
	}
}
