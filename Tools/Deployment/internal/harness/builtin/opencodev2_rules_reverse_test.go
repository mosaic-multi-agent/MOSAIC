package builtin_test

// opencodev2_rules_reverse_test.go specifies reading a deployed V2 permissions value back to
// harness tool names, dispositions and generic tools through the embedded opencode-v2
// descriptor, including the lossy edit case: V2's edit action cannot tell file write from file
// edit, so an allowed edit reads back as both.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// rulesValue builds a deployed permissions value from rules, in the document layout.
func rulesValue(rules ...ruleT) domain.FieldValue {
	items := make([]domain.FieldValue, len(rules))
	for i, r := range rules {
		items[i] = domain.MappingValue([]domain.FieldPair{
			{Key: "action", Value: domain.ScalarValue(r.action, domain.QuotePlain)},
			{Key: "resource", Value: domain.ScalarValue(r.resource, domain.QuotePlain)},
			{Key: "effect", Value: domain.ScalarValue(r.effect, domain.QuotePlain)},
		})
	}
	return domain.ListValue(items, domain.ListBlock)
}

func allow(action string) ruleT { return ruleT{action, "*", "allow"} }
func deny(action string) ruleT  { return ruleT{action, "*", "deny"} }
func ask(action string) ruleT   { return ruleT{action, "*", "ask"} }

// genericsOf reverse maps harness tool names to the set of generic tools, in first-seen order.
func genericsOf(d *domain.HarnessDescriptor, names []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range descriptor.ReverseMapTools(d, names) {
		if r.Outcome == domain.ToolMapped && !seen[r.Generic] {
			seen[r.Generic] = true
			out = append(out, r.Generic)
		}
	}
	return out
}

func TestOpenCodeV2Reverse_RenderedRulesReadBackAsTheDeclaredTools(t *testing.T) {
	d := loadV2(t)
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"terminal", "file_read", "file_search"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	entries := descriptor.ExtractToolEntries(d, res.Fields[0].Value)

	if want := []string{"read", "glob", "shell"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
	if got := genericsOf(d, entries); !sameSet(got, []string{"file_read", "file_search", "terminal"}) {
		t.Errorf("generic tools = %v, want file_read, file_search, terminal", got)
	}
}

func TestOpenCodeV2Reverse_FixedRulesNeverBecomeTools(t *testing.T) {
	d := loadV2(t)
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_read"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	entries := descriptor.ExtractToolEntries(d, res.Fields[0].Value)
	grants := descriptor.ExtractToolGrants(d, res.Fields[0].Value)

	if !reflect.DeepEqual(entries, []string{"read"}) {
		t.Errorf("entries = %v, want [read]; the leading deny and the outside-workspace rule are not tools", entries)
	}
	if !reflect.DeepEqual(grants, []domain.ToolGrant{{Name: "read", Disposition: domain.Allow}}) {
		t.Errorf("grants = %v, want only read allowed", grants)
	}
}

func TestOpenCodeV2Reverse_AllowedEditActionReadsBackAsWriteEditAndPatch(t *testing.T) {
	d := loadV2(t)

	entries := descriptor.ExtractToolEntries(d, rulesValue(denyAllRule(), allow("edit")))

	if want := []string{"write", "edit", "patch"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v in universe order", entries, want)
	}
}

func TestOpenCodeV2Reverse_LossyEditRecoversBothFileWriteAndFileEdit(t *testing.T) {
	d := loadV2(t)
	// An agent declaring only file_write is deployed as the edit action.
	res, err := descriptor.MapTools(d, domain.ToolRequest{AgentKey: "a", Generic: []string{"file_write"}})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}

	entries := descriptor.ExtractToolEntries(d, res.Fields[0].Value)

	if got := genericsOf(d, entries); !sameSet(got, []string{"file_write", "file_edit"}) {
		t.Errorf("generic tools = %v, want both file_write and file_edit (the documented widening)", got)
	}
}

func TestOpenCodeV2Reverse_GrantsOfEditAreTheThreeFileWritingTools(t *testing.T) {
	d := loadV2(t)

	grants := descriptor.ExtractToolGrants(d, rulesValue(denyAllRule(), allow("edit")))

	want := []domain.ToolGrant{
		{Name: "write", Disposition: domain.Allow},
		{Name: "edit", Disposition: domain.Allow},
		{Name: "patch", Disposition: domain.Allow},
	}
	if !reflect.DeepEqual(grants, want) {
		t.Errorf("grants = %v, want %v", grants, want)
	}
}

func TestOpenCodeV2Reverse_LastWholeToolRuleOfAnActionDecides(t *testing.T) {
	d := loadV2(t)
	value := rulesValue(denyAllRule(), allow("edit"), allow("read"), deny("edit"), deny("shell"), allow("shell"))

	entries := descriptor.ExtractToolEntries(d, value)
	grants := descriptor.ExtractToolGrants(d, value)

	if want := []string{"read", "shell"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
	wantGrants := []domain.ToolGrant{
		{Name: "write", Disposition: domain.Deny},
		{Name: "edit", Disposition: domain.Deny},
		{Name: "patch", Disposition: domain.Deny},
		{Name: "read", Disposition: domain.Allow},
		{Name: "shell", Disposition: domain.Allow},
	}
	if !reflect.DeepEqual(grants, wantGrants) {
		t.Errorf("grants = %v, want %v (actions in order of their first rule)", grants, wantGrants)
	}
}

func TestOpenCodeV2Reverse_AskRulesGrantNothing(t *testing.T) {
	d := loadV2(t)
	value := rulesValue(denyAllRule(), ask("shell"), allow("read"))

	entries := descriptor.ExtractToolEntries(d, value)
	grants := descriptor.ExtractToolGrants(d, value)

	if !reflect.DeepEqual(entries, []string{"read"}) {
		t.Errorf("entries = %v, want [read]", entries)
	}
	if !reflect.DeepEqual(grants, []domain.ToolGrant{{Name: "read", Disposition: domain.Allow}}) {
		t.Errorf("grants = %v, want only read allowed", grants)
	}
}

func TestOpenCodeV2Reverse_RulesNarrowerThanTheWholeToolAreIgnored(t *testing.T) {
	d := loadV2(t)
	value := rulesValue(
		denyAllRule(),
		ruleT{"read", "src/*", "allow"},
		allow("glob"),
		ruleT{"glob", "docs/*", "deny"},
	)

	entries := descriptor.ExtractToolEntries(d, value)

	if !reflect.DeepEqual(entries, []string{"glob"}) {
		t.Errorf("entries = %v, want [glob]: a path-scoped rule is neither a grant nor a deny of the tool", entries)
	}
}

func TestOpenCodeV2Reverse_CustomMCPEntryReadsBackAsItself(t *testing.T) {
	d := loadV2(t)
	value := rulesValue(denyAllRule(), allow("read"), allow("hitl_*"), allow("srv_lookup"), deny("blocked_*"))

	entries := descriptor.ExtractToolEntries(d, value)
	grants := descriptor.ExtractToolGrants(d, value)

	if want := []string{"read", "hitl_*", "srv_lookup"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
	wantGrants := []domain.ToolGrant{
		{Name: "read", Disposition: domain.Allow},
		{Name: "hitl_*", Disposition: domain.Allow},
		{Name: "srv_lookup", Disposition: domain.Allow},
		{Name: "blocked_*", Disposition: domain.Deny},
	}
	if !reflect.DeepEqual(grants, wantGrants) {
		t.Errorf("grants = %v, want %v", grants, wantGrants)
	}
	if server, ok := descriptor.ReverseCustomTool(d, "hitl_*"); !ok || server != "hitl" {
		t.Errorf("ReverseCustomTool(hitl_*) = %q, %v; want the hitl server", server, ok)
	}
}

func TestOpenCodeV2Reverse_DeployedFixedActionsAreNeverTools(t *testing.T) {
	d := loadV2(t)
	// A user who allowed outside-workspace access: still not a generic tool.
	value := rulesValue(allow("external_directory"), allow("read"))

	entries := descriptor.ExtractToolEntries(d, value)

	if !reflect.DeepEqual(entries, []string{"read"}) {
		t.Errorf("entries = %v, want [read]", entries)
	}
}

func TestOpenCodeV2Reverse_ToolsWithoutAGenericMappingStayVisibleAsUniverseTools(t *testing.T) {
	d := loadV2(t)

	entries := descriptor.ExtractToolEntries(d, rulesValue(denyAllRule(), allow("execute")))

	if !reflect.DeepEqual(entries, []string{"execute"}) {
		t.Errorf("entries = %v, want [execute]", entries)
	}
	if got := genericsOf(d, entries); len(got) != 0 {
		t.Errorf("execute has no generic tool, got %v", got)
	}
}

func TestOpenCodeV2Reverse_EntriesAreDeduplicatedInFirstSeenOrder(t *testing.T) {
	d := loadV2(t)
	value := rulesValue(allow("shell"), allow("read"), allow("shell"), allow("hitl_*"), allow("hitl_*"))

	entries := descriptor.ExtractToolEntries(d, value)

	if want := []string{"shell", "read", "hitl_*"}; !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
}

func TestOpenCodeV2Reverse_ValueWithoutRulesGivesNothing(t *testing.T) {
	d := loadV2(t)

	for name, v := range map[string]domain.FieldValue{
		"zero":    {},
		"scalar":  domain.ScalarValue("read", domain.QuotePlain),
		"mapping": domain.MappingValue([]domain.FieldPair{{Key: "read", Value: domain.ScalarValue("allow", domain.QuotePlain)}}),
		"empty":   rulesValue(),
	} {
		if got := descriptor.ExtractToolEntries(d, v); len(got) != 0 {
			t.Errorf("%s: entries = %v, want none", name, got)
		}
		if got := descriptor.ExtractToolGrants(d, v); len(got) != 0 {
			t.Errorf("%s: grants = %v, want none", name, got)
		}
	}
}

func denyAllRule() ruleT { return ruleT{"*", "*", "deny"} }
