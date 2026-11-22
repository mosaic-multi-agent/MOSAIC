package builtin_test

// mcp_entries_test.go verifies that a custom tool answer made of several MCP entries is written
// by each built-in harness descriptor in its own whole-server or single-tool form, resolves as
// custom (never unmapped), and that Codex, which declares no MCP forms, keeps its output.
//
// Coverage:
//   - claude-code, ghcp-cli, vscode-ghcp, opencode: every entry of a mixed answer appears in
//     the agent's tools in that harness's form, in answer order.
//   - opencode-v2: every entry appears as an allow rule after the built-in rules.
//   - Server names that normalise are normalised in the single-tool form too.
//   - Codex: one neutral name per entry in the resolution; the rendered fields equal those of
//     a single-server answer.

import (
	"reflect"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/harness/descriptor"
)

// entriesAnswer is the mixed answer written on every harness: two tools of one server and a
// whole second server.
var entriesAnswer = []domain.MCPEntry{
	{Server: "github", Tool: "get_pr"},
	{Server: "github", Tool: "list_comments"},
	{Server: "jira"},
}

// entriesHarness is one list or permission shape built-in harness with the names it must write
// for entriesAnswer.
type entriesHarness struct {
	dir, file string
	want      []string
	// dotted is the name written for the entry {Server: "my.server", Tool: "get_pr"}.
	dotted string
}

var entriesHarnesses = []entriesHarness{
	{"claudecode", "claude-code.yaml",
		[]string{"mcp__github__get_pr", "mcp__github__list_comments", "mcp__jira__*"}, "mcp__my_server__get_pr"},
	{"opencode", "opencode.yaml",
		[]string{"github_get_pr", "github_list_comments", "jira_*"}, "my_server_get_pr"},
	{"ghcpcli", "ghcp-cli.yaml",
		[]string{"github/get_pr", "github/list_comments", "jira/*"}, "my.server/get_pr"},
	{"vscodeghcp", "vscode-ghcp.yaml",
		[]string{"github/get_pr", "github/list_comments", "jira/*"}, "my.server/get_pr"},
}

// mapEntries maps the generic "user_feedback" with the given entries on the descriptor.
func mapEntries(t *testing.T, d *domain.HarnessDescriptor, entries []domain.MCPEntry) domain.ToolResult {
	t.Helper()
	res, err := descriptor.MapTools(d, domain.ToolRequest{
		AgentKey:    "mcp-user",
		Generic:     []string{"user_feedback"},
		CustomNames: domain.CustomToolAnswers{"user_feedback": entries},
	})
	if err != nil {
		t.Fatalf("MapTools: %v", err)
	}
	if len(res.Resolutions) != 1 {
		t.Fatalf("want one resolution, got %d", len(res.Resolutions))
	}
	return res
}

// toolsFieldNames lists the names the tools field carries, in field order.
func toolsFieldNames(d *domain.HarnessDescriptor, res domain.ToolResult) []string {
	var names []string
	for _, f := range res.Fields {
		if f.Key != d.Frontmatter.ToolsKey {
			continue
		}
		switch f.Value.Kind {
		case domain.KindMapping:
			for _, p := range f.Value.Pairs {
				names = append(names, p.Key)
			}
		case domain.KindList:
			for _, it := range f.Value.Items {
				names = append(names, it.Scalar)
			}
		}
	}
	return names
}

// containsInOrder reports whether want appears in names as a subsequence.
func containsInOrder(names, want []string) bool {
	i := 0
	for _, n := range names {
		if i < len(want) && n == want[i] {
			i++
		}
	}
	return i == len(want)
}

func TestCustomEntries_EachEntryIsWrittenInTheHarnessForm(t *testing.T) {
	for _, h := range entriesHarnesses {
		t.Run(h.dir, func(t *testing.T) {
			d := loadEmbeddedDescriptor(t, h.file)

			res := mapEntries(t, d, entriesAnswer)

			r := res.Resolutions[0]
			if r.Outcome != domain.ToolCustom {
				t.Fatalf("outcome = %q, want %q", r.Outcome, domain.ToolCustom)
			}
			if !reflect.DeepEqual(r.HarnessTools, h.want) {
				t.Errorf("HarnessTools = %v, want %v", r.HarnessTools, h.want)
			}
			names := toolsFieldNames(d, res)
			if !containsInOrder(names, h.want) {
				t.Errorf("tools field must carry %v in answer order; got %v", h.want, names)
			}
		})
	}
}

func TestCustomEntries_PermissionShapeAllowsEveryEntry(t *testing.T) {
	d := loadEmbeddedDescriptor(t, "opencode.yaml")

	res := mapEntries(t, d, entriesAnswer)

	allowed := map[string]string{}
	for _, f := range res.Fields {
		if f.Key == d.Frontmatter.ToolsKey {
			for _, p := range f.Value.Pairs {
				allowed[p.Key] = p.Value.Scalar
			}
		}
	}
	for _, n := range []string{"github_get_pr", "github_list_comments", "jira_*"} {
		if allowed[n] != "allow" {
			t.Errorf("%q must be allowed in the permission map; got %v", n, allowed)
		}
	}
}

func TestCustomEntries_NormalisedServerNameIsUsedInTheSingleToolForm(t *testing.T) {
	for _, h := range entriesHarnesses {
		t.Run(h.dir, func(t *testing.T) {
			d := loadEmbeddedDescriptor(t, h.file)

			res := mapEntries(t, d, []domain.MCPEntry{{Server: "my.server", Tool: "get_pr"}})

			got := res.Resolutions[0].HarnessTools
			if !reflect.DeepEqual(got, []string{h.dotted}) {
				t.Errorf("HarnessTools = %v, want [%s]", got, h.dotted)
			}
		})
	}
}

func TestCustomEntries_OpenCodeV2WritesEveryEntryAsAnAllowRule(t *testing.T) {
	rules := renderV2(t, domain.ToolRequest{
		AgentKey: "a", Generic: []string{"file_read", "ask_user"},
		CustomNames: domain.CustomToolAnswers{"ask_user": entriesAnswer},
	})

	want := []ruleT{
		denyAll, {"read", "*", "allow"},
		{"github_get_pr", "*", "allow"}, {"github_list_comments", "*", "allow"}, {"jira_*", "*", "allow"},
		askOutsideTheDir,
	}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %v, want %v", rules, want)
	}
}

func TestCustomEntries_OpenCodeV2ResolvesTheAnswerAsCustom(t *testing.T) {
	d := loadV2(t)

	res := mapEntries(t, d, entriesAnswer)

	r := res.Resolutions[0]
	want := []string{"github_get_pr", "github_list_comments", "jira_*"}
	if r.Outcome != domain.ToolCustom || !reflect.DeepEqual(r.HarnessTools, want) {
		t.Errorf("resolution = %s %v, want custom %v", r.Outcome, r.HarnessTools, want)
	}
}

func TestCustomEntries_CodexKeepsOneNeutralNamePerEntryAndItsFieldsUnchanged(t *testing.T) {
	d := loadEmbeddedDescriptor(t, "codex.yaml")

	multi := mapEntries(t, d, entriesAnswer)
	single := mapEntries(t, d, []domain.MCPEntry{{Server: "github"}})

	r := multi.Resolutions[0]
	want := []string{"github/get_pr", "github/list_comments", "jira"}
	if r.Outcome != domain.ToolCustom || !reflect.DeepEqual(r.HarnessTools, want) {
		t.Errorf("resolution = %s %v, want custom %v", r.Outcome, r.HarnessTools, want)
	}
	if !reflect.DeepEqual(multi.Fields, single.Fields) {
		t.Errorf("codex fields must not depend on the answer:\n multi  %+v\n single %+v", multi.Fields, single.Fields)
	}
	if got := single.Resolutions[0].HarnessTools; !reflect.DeepEqual(got, []string{"github"}) {
		t.Errorf("a single whole-server answer keeps today's raw name; got %v", got)
	}
}
