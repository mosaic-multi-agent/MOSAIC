package builtin_test

// opencodev2_rules_render_test.go specifies the V2-native permissions that Deploy renders for
// an agent's declared generic tools through the embedded opencode-v2 descriptor: exactly the
// declared tools are allowed, everything else is denied by a leading rule, outside-workspace
// access asks, and no rule carries an absolute path.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

var (
	denyAll          = ruleT{"*", "*", "deny"}
	askOutsideTheDir = ruleT{"external_directory", "*", "ask"}
)

func TestOpenCodeV2Descriptor_UsesTheRulesShapeWithFixedDenyAndAskRules(t *testing.T) {
	d := loadV2(t)

	if d.Tools.Shape != domain.ShapeRules {
		t.Errorf("Shape = %q, want %q", d.Tools.Shape, domain.ShapeRules)
	}
	wantLeading := []domain.PermissionRule{{Action: "*", Resource: "*", Effect: domain.EffectDeny}}
	wantTrailing := []domain.PermissionRule{{Action: "external_directory", Resource: "*", Effect: domain.EffectAsk}}
	if !reflect.DeepEqual(d.Tools.Rules.Leading, wantLeading) {
		t.Errorf("Leading = %v, want %v", d.Tools.Rules.Leading, wantLeading)
	}
	if !reflect.DeepEqual(d.Tools.Rules.Trailing, wantTrailing) {
		t.Errorf("Trailing = %v, want %v", d.Tools.Rules.Trailing, wantTrailing)
	}
}

func TestOpenCodeV2Descriptor_FileWritingToolsAreGovernedByTheEditAction(t *testing.T) {
	d := loadV2(t)

	for _, u := range d.Tools.Universe {
		want := ""
		if u.Name == "write" || u.Name == "patch" {
			want = "edit"
		}
		if u.Action != want {
			t.Errorf("%s: Action = %q, want %q", u.Name, u.Action, want)
		}
	}
}

func TestOpenCodeV2Descriptor_LossyEditCaseUsesTheDefaultRecoverability(t *testing.T) {
	d := loadV2(t)

	if d.ToolInfoUnrecoverable {
		t.Error("tool_info_recoverable stays absent: the lossy edit case is handled by reading edit back as write, edit and patch")
	}
}

func TestOpenCodeV2Rules_EachDeclaredToolAllowsExactlyItsAction(t *testing.T) {
	want := map[string][]string{
		"file_read":        {"read"},
		"file_write":       {"edit"},
		"file_edit":        {"edit"},
		"file_search":      {"glob"},
		"content_search":   {"grep"},
		"terminal":         {"shell"},
		"user_interaction": {"question"},
		"subagent":         {"subagent"},
		"skill":            {"skill"},
		"web_fetch":        {"webfetch"},
		"web_search":       {"websearch"},
	}
	for generic, wantAllowed := range want {
		t.Run(generic, func(t *testing.T) {
			rules := renderGenerics(t, generic)

			got := allowedActions(rules)

			if !sameSet(got, wantAllowed) {
				t.Errorf("allowed = %v, want exactly %v; effects: %v", got, wantAllowed, effects(rules, everyKnownAction))
			}
		})
	}
}

func TestOpenCodeV2Rules_FileWriteAndFileEditBothGrantEveryFileWritingAction(t *testing.T) {
	// V2 grants write, edit and patch together under the edit action, so an agent declaring
	// either generic can write and edit on Claude and GPT models alike.
	for _, generic := range []string{"file_write", "file_edit"} {
		rules := renderGenerics(t, generic)

		if effectFor(rules, "edit") != "allow" {
			t.Errorf("%s: edit must be allowed, rules: %v", generic, rules)
		}
	}
}

func TestOpenCodeV2Rules_AllDeclaredToolsTogetherAllowEverythingButTheUnmappedDefaults(t *testing.T) {
	rules := renderGenerics(t, "file_read", "file_write", "file_edit", "file_search", "content_search",
		"terminal", "user_interaction", "subagent", "skill", "web_fetch", "web_search")

	got := allowedActions(rules)

	want := []string{"read", "edit", "glob", "grep", "shell", "webfetch", "websearch", "question", "skill", "subagent"}
	if !sameSet(got, want) {
		t.Errorf("allowed = %v, want %v", got, want)
	}
}

func TestOpenCodeV2Rules_UndeclaredToolsAreDeniedIncludingDefaultOnBuiltIns(t *testing.T) {
	rules := renderGenerics(t, "file_read")

	for _, action := range []string{
		"execute", "websearch", "experimental_image_generate", "browser", "shell", "edit",
		"opencode_list_mcp_resources", "opencode_read_mcp_resource", "some_future_action",
	} {
		if got := effectFor(rules, action); got != "deny" {
			t.Errorf("%s resolves to %q, want deny; rules: %v", action, got, rules)
		}
	}
}

func TestOpenCodeV2Rules_ToolsOfUndeclaredMCPServersAreDenied(t *testing.T) {
	rules := renderV2(t, domain.ToolRequest{
		AgentKey: "a", Generic: []string{"file_read", "ask_user"}, CustomNames: domain.CustomToolAnswers{"ask_user": {{Server: "hitl"}}},
	})

	if got := effectFor(rules, "someother_lookup"); got != "deny" {
		t.Errorf("a tool of an undeclared server resolves to %q, want deny", got)
	}
	if got := effectFor(rules, "hitlish_tool"); got != "deny" {
		t.Errorf("a server whose name merely starts like the declared one resolves to %q, want deny", got)
	}
}

func TestOpenCodeV2Rules_NoUndeclaredActionFallsBackToAsk(t *testing.T) {
	for _, generics := range [][]string{nil, {"file_read"}, {"file_write", "terminal", "subagent"}} {
		rules := renderGenerics(t, generics...)

		for _, action := range everyKnownAction {
			if effectFor(rules, action) == "ask" {
				t.Errorf("generics %v: %s resolves to ask; nothing may fall back to ask by accident", generics, action)
			}
		}
	}
}

func TestOpenCodeV2Rules_DeclaredCustomMCPToolIsAllowedAfterTheBuiltIns(t *testing.T) {
	rules := renderV2(t, domain.ToolRequest{
		AgentKey: "a", Generic: []string{"file_read", "ask_user"}, CustomNames: domain.CustomToolAnswers{"ask_user": {{Server: "hitl"}}},
	})

	if got := effectFor(rules, "hitl_ask_user"); got != "allow" {
		t.Errorf("a tool of the declared custom server resolves to %q, want allow; rules: %v", got, rules)
	}
	want := []ruleT{denyAll, {"read", "*", "allow"}, {"hitl_*", "*", "allow"}, askOutsideTheDir}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %v, want %v", rules, want)
	}
}

func TestOpenCodeV2Rules_MappedToolOutsideTheUniverseBecomesAnAllowRuleNamedByItself(t *testing.T) {
	// A tool_destinations style entry names a tool the universe does not know.
	d := loadV2(t)
	d.Tools.Mappings = append(d.Tools.Mappings, domain.ToolMapping{
		Generic:      "lookup",
		Destinations: []domain.ToolDestination{{Kind: domain.DestMain, Names: []string{"srv_lookup"}}},
	})

	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_read", "lookup"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	rules := readRules(t, res.Fields[0].Value)

	want := []ruleT{denyAll, {"read", "*", "allow"}, {"srv_lookup", "*", "allow"}, askOutsideTheDir}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %v, want %v", rules, want)
	}
}

func TestOpenCodeV2Rules_UnmappedAndSkippedGenericToolsGrantNothing(t *testing.T) {
	rules := renderV2(t, domain.ToolRequest{
		AgentKey: "a", Generic: []string{"file_read", "no_such_generic", "web_search"},
		SkippedTools: map[string]bool{"web_search": true},
	})

	want := []ruleT{denyAll, {"read", "*", "allow"}, askOutsideTheDir}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %v, want %v", rules, want)
	}
}

func TestOpenCodeV2Rules_OutsideWorkspaceAccessAsksAndComesLast(t *testing.T) {
	for _, generics := range [][]string{nil, {"file_read"}, {"file_write", "terminal"}} {
		rules := renderGenerics(t, generics...)

		if got := effectFor(rules, "external_directory"); got != "ask" {
			t.Errorf("generics %v: external_directory resolves to %q, want ask", generics, got)
		}
		if last := rules[len(rules)-1]; last != askOutsideTheDir {
			t.Errorf("generics %v: last rule = %v, want %v", generics, last, askOutsideTheDir)
		}
		if rules[0] != denyAll {
			t.Errorf("generics %v: first rule = %v, want %v", generics, rules[0], denyAll)
		}
	}
}

func TestOpenCodeV2Rules_NoDeclaredToolsRendersOnlyTheFixedRules(t *testing.T) {
	rules := renderGenerics(t)

	want := []ruleT{denyAll, askOutsideTheDir}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %v, want %v", rules, want)
	}
}

func TestOpenCodeV2Rules_EveryRuleIsWholeToolWithNoAbsolutePath(t *testing.T) {
	rules := renderV2(t, domain.ToolRequest{
		AgentKey: "a", Generic: []string{"file_read", "file_write", "terminal", "ask_user"},
		CustomNames: domain.CustomToolAnswers{"ask_user": {{Server: "hitl"}}},
	})

	for _, r := range rules {
		if r.resource != "*" {
			t.Errorf("rule %v: resource must be the whole-tool %q", r, "*")
		}
		if strings.HasPrefix(r.resource, "/") || strings.HasPrefix(r.resource, "~") || strings.Contains(r.resource, ":") {
			t.Errorf("rule %v: resource looks like an absolute path", r)
		}
	}
}

func TestOpenCodeV2Rules_BuiltInRulesFollowUniverseOrderWhateverTheDeclarationOrder(t *testing.T) {
	a := renderGenerics(t, "terminal", "file_write", "file_read")
	b := renderGenerics(t, "file_read", "file_write", "terminal")

	want := []ruleT{denyAll, {"read", "*", "allow"}, {"edit", "*", "allow"}, {"shell", "*", "allow"}, askOutsideTheDir}
	if !reflect.DeepEqual(a, want) || !reflect.DeepEqual(b, want) {
		t.Errorf("rules must follow universe order, got\n%v\nand\n%v\nwant\n%v", a, b, want)
	}
}

func TestOpenCodeV2Rules_ActionSharedByTwoDeclaredToolsIsRenderedOnce(t *testing.T) {
	rules := renderGenerics(t, "file_write", "file_edit")

	n := 0
	for _, r := range rules {
		if r.action == "edit" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the edit action must appear in exactly one rule, got %d: %v", n, rules)
	}
}

func TestOpenCodeV2Rules_OutputIsDeterministic(t *testing.T) {
	req := domain.ToolRequest{
		AgentKey: "a", Generic: []string{"terminal", "ask_user", "file_read", "subagent"},
		CustomNames: domain.CustomToolAnswers{"ask_user": {{Server: "hitl"}}},
	}

	first, second := renderV2(t, req), renderV2(t, req)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("equal requests rendered differently:\n%v\n%v", first, second)
	}
}

func TestOpenCodeV2Rules_RenderedValueQuotesStarLeadingScalars(t *testing.T) {
	d := loadV2(t)
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_read"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	value := res.Fields[0].Value
	if value.Kind != domain.KindList || len(value.Items) == 0 || len(value.Items[0].Pairs) < 2 {
		t.Fatalf("want a rule list whose first item is a rule, got %+v", value)
	}
	first := value.Items[0]

	if first.Pairs[0].Value.Quote != domain.QuoteDouble || first.Pairs[1].Value.Quote != domain.QuoteDouble {
		t.Errorf("* action and resource must be double quoted, got %v and %v",
			first.Pairs[0].Value.Quote, first.Pairs[1].Value.Quote)
	}
}

func TestOpenCodeV2Rules_OnlyTheV2PermissionsFieldIsEmitted(t *testing.T) {
	d := loadV2(t)
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_read"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	for _, f := range res.Fields {
		if f.Key == "permission" || f.Key == "tools" {
			t.Errorf("field %q is V1 frontmatter that V2 must not be left to translate", f.Key)
		}
	}
	if len(res.Fields) != 1 || res.Fields[0].Key != "permissions" {
		t.Errorf("want exactly the permissions field, got %+v", res.Fields)
	}
}

func TestOpenCodeV2Rules_ResolutionsAreComputedAsForOtherShapes(t *testing.T) {
	d := loadV2(t)

	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_write", "mystery"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	if len(res.Resolutions) != 2 {
		t.Fatalf("want one resolution per generic tool, got %+v", res.Resolutions)
	}
	if r := res.Resolutions[0]; r.Outcome != domain.ToolMapped || !reflect.DeepEqual(r.HarnessTools, []string{"write", "patch"}) {
		t.Errorf("file_write resolution = %+v, want mapped to [write patch]", r)
	}
	if r := res.Resolutions[1]; r.Outcome != domain.ToolUnmapped {
		t.Errorf("mystery resolution outcome = %q, want %q", r.Outcome, domain.ToolUnmapped)
	}
}

func TestOpenCodeV2Rules_PlaceholderExpandsToACompleteRuleSet(t *testing.T) {
	d := loadV2(t)

	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "orchestrator", Placeholder: "{tool-permissions}"})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	if res.Resolutions == nil || len(res.Resolutions) != 0 {
		t.Errorf("Resolutions must be empty and non-nil, got %#v", res.Resolutions)
	}
	if len(res.Fields) == 0 || res.Fields[0].Key != "permissions" {
		t.Fatalf("first field must be permissions, got %+v", res.Fields)
	}
	rules := readRules(t, res.Fields[0].Value)
	want := []ruleT{
		denyAll,
		{"read", "*", "allow"}, {"edit", "*", "allow"}, {"glob", "*", "allow"}, {"grep", "*", "allow"},
		{"shell", "*", "allow"}, {"question", "*", "allow"}, {"subagent", "*", "allow"},
		askOutsideTheDir,
	}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("placeholder rules = %v, want %v", rules, want)
	}
}

func TestOpenCodeV2Rules_PlaceholderLeavesEveryOtherToolDenied(t *testing.T) {
	d := loadV2(t)
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "orchestrator", Placeholder: "{tool-permissions}"})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	rules := readRules(t, res.Fields[0].Value)

	for _, action := range []string{"execute", "websearch", "webfetch", "skill", "browser", "experimental_image_generate", "undeclared_lookup"} {
		if got := effectFor(rules, action); got != "deny" {
			t.Errorf("%s resolves to %q under the placeholder rules, want deny", action, got)
		}
	}
	if effectFor(rules, "subagent") != "allow" {
		t.Error("the orchestrator must be able to dispatch every deployed agent: subagent allowed for resource *")
	}
}
