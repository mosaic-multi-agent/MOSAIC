package customtool_test

// customtool_test.go verifies the neutral custom tool answer syntax: comma-separated entries,
// each "server" (every tool of the server) or "server/tool" (one tool); "server/*" is the same
// as "server".
//
// Coverage:
//   - Mixed lists across several servers keep their order; "server/*" reads as "server";
//     exact duplicates are dropped; whitespace around entries and around "/" is trimmed.
//   - A blank answer is no answer, not an error.
//   - Each refusal (empty entry, more than one "/", empty part, disallowed character) names the
//     position, the entry and the reason; the first failing entry is the one reported.
//   - Text and EntryText render the neutral form and round-trip with Parse.

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

func TestParse_AcceptsMixedListsAcrossServersInAnswerOrder(t *testing.T) {
	cases := []struct {
		name, text string
		want       []domain.MCPEntry
	}{
		{"single server", "github", []domain.MCPEntry{{Server: "github"}}},
		{"single tool", "github/get_pr", []domain.MCPEntry{{Server: "github", Tool: "get_pr"}}},
		{"mixed across servers", "github/get_pr, github/list_comments, jira", []domain.MCPEntry{
			{Server: "github", Tool: "get_pr"}, {Server: "github", Tool: "list_comments"}, {Server: "jira"},
		}},
		{"order is the answer's order", "jira, github/get_pr, github", []domain.MCPEntry{
			{Server: "jira"}, {Server: "github", Tool: "get_pr"}, {Server: "github"},
		}},
		{"star tool is the whole server", "github/*", []domain.MCPEntry{{Server: "github"}}},
		{"name characters", "my-server_1.x/tool.v-2_a", []domain.MCPEntry{{Server: "my-server_1.x", Tool: "tool.v-2_a"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := customtool.Parse(tc.text)

			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.text, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParse_DropsExactDuplicatesKeepingTheFirst(t *testing.T) {
	cases := []struct {
		name, text string
		want       []domain.MCPEntry
	}{
		{"same tool twice", "github/get_pr, jira, github/get_pr", []domain.MCPEntry{
			{Server: "github", Tool: "get_pr"}, {Server: "jira"},
		}},
		{"server and server star", "github/*, jira, github", []domain.MCPEntry{{Server: "github"}, {Server: "jira"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := customtool.Parse(tc.text)

			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.text, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.text, got, tc.want)
			}
		})
	}
}

func TestParse_KeepsAToolEntryNextToItsWholeServer(t *testing.T) {
	got, err := customtool.Parse("github, github/get_pr")

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []domain.MCPEntry{{Server: "github"}, {Server: "github", Tool: "get_pr"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse = %+v, want %+v", got, want)
	}
}

func TestParse_TrimsWhitespaceAroundEntriesAndSlash(t *testing.T) {
	got, err := customtool.Parse("  github / get_pr ,\tjira  ,  github/*  ")

	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []domain.MCPEntry{{Server: "github", Tool: "get_pr"}, {Server: "jira"}, {Server: "github"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse = %+v, want %+v", got, want)
	}
}

func TestParse_BlankAnswerIsNoAnswer(t *testing.T) {
	for _, text := range []string{"", "   ", "\t\n"} {
		got, err := customtool.Parse(text)

		if err != nil {
			t.Errorf("Parse(%q) error = %v, want none", text, err)
		}
		if len(got) != 0 {
			t.Errorf("Parse(%q) = %+v, want no entries", text, got)
		}
	}
}

func TestParse_RefusesAnEntryThatDoesNotFitAndNamesItAndWhy(t *testing.T) {
	const allowed = `; allowed are letters, digits, "_", "-" and "."`
	cases := []struct {
		name, text string
		index      int
		entry      string
		reason     string
	}{
		{"empty entry in the middle", "a,,b", 2, "", "empty entry"},
		{"trailing comma", "a,", 2, "", "empty entry"},
		{"leading comma", ",a", 1, "", "empty entry"},
		{"whitespace-only entry", "a, ,b", 2, "", "empty entry"},
		{"more than one slash", "github/a/b", 1, "github/a/b", `more than one "/"`},
		{"empty server part", "/tool", 1, "/tool", "empty server name"},
		{"empty tool part", "server/", 1, "server/", "empty tool name"},
		{"empty tool part after trimming", "server/  ", 1, "server/", "empty tool name"},
		{"slash alone", "/", 1, "/", "empty server name"},
		{"space in server", "my server", 1, "my server", `server name holds " "` + allowed},
		{"at sign in server", "gith@b", 1, "gith@b", `server name holds "@"` + allowed},
		{"star inside server", "*/x", 1, "*/x", `server name holds "*"` + allowed},
		{"space in tool", "github/get pr", 1, "github/get pr", `tool name holds " "` + allowed},
		{"star inside tool name", "github/get*", 1, "github/get*", `tool name holds "*"` + allowed},
		{"non-ascii letter", "github/café", 1, "github/café", `tool name holds "é"` + allowed},
		{"first offending character", "a/b$c%d", 1, "a/b$c%d", `tool name holds "$"` + allowed},
		{"slash count wins over characters", "a b/c/d", 1, "a b/c/d", `more than one "/"`},
		{"first failing entry is reported", "ok, bad name, also@bad", 2, "bad name", `server name holds " "` + allowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := customtool.Parse(tc.text)

			var entryErr *customtool.EntryError
			if !errors.As(err, &entryErr) {
				t.Fatalf("Parse(%q) error = %v, want *EntryError", tc.text, err)
			}
			if got != nil {
				t.Errorf("Parse(%q) = %+v, want no entries on refusal", tc.text, got)
			}
			if entryErr.Index != tc.index || entryErr.Entry != tc.entry || entryErr.Reason != tc.reason {
				t.Errorf("EntryError = {%d %q %q}, want {%d %q %q}",
					entryErr.Index, entryErr.Entry, entryErr.Reason, tc.index, tc.entry, tc.reason)
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.entry) || !strings.Contains(msg, tc.reason) {
				t.Errorf("message %q must name the entry %q and the reason %q", msg, tc.entry, tc.reason)
			}
		})
	}
}

func TestEntryError_MessageNamesPositionEntryAndReason(t *testing.T) {
	_, err := customtool.Parse("jira, github/a/b")

	var entryErr *customtool.EntryError
	if !errors.As(err, &entryErr) {
		t.Fatalf("error = %v, want *EntryError", err)
	}
	if want := `entry 2 "github/a/b": more than one "/"`; err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestValidate_IsParseReducedToItsError(t *testing.T) {
	for _, ok := range []string{"", "  ", "github/get_pr, jira", "github/*"} {
		if err := customtool.Validate(ok); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"a,,b", "a/b/c", "my server"} {
		var entryErr *customtool.EntryError
		if err := customtool.Validate(bad); !errors.As(err, &entryErr) {
			t.Errorf("Validate(%q) = %v, want *EntryError", bad, err)
		}
	}
}

func TestEntryText_RendersServerOrServerSlashTool(t *testing.T) {
	if got := customtool.EntryText(domain.MCPEntry{Server: "jira"}); got != "jira" {
		t.Errorf("EntryText whole server = %q, want %q", got, "jira")
	}
	if got := customtool.EntryText(domain.MCPEntry{Server: "github", Tool: "get_pr"}); got != "github/get_pr" {
		t.Errorf("EntryText tool = %q, want %q", got, "github/get_pr")
	}
}

func TestText_JoinsEntriesWithCommaAndSpace(t *testing.T) {
	got := customtool.Text([]domain.MCPEntry{
		{Server: "github", Tool: "get_pr"}, {Server: "github", Tool: "list_comments"}, {Server: "jira"},
	})

	if want := "github/get_pr, github/list_comments, jira"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

func TestText_RoundTripsWithParse(t *testing.T) {
	parsed, err := customtool.Parse(" github / get_pr,jira , github/* ,jira")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	again, err := customtool.Parse(customtool.Text(parsed))

	if err != nil {
		t.Fatalf("Parse(Text(parsed)): %v", err)
	}
	if !reflect.DeepEqual(again, parsed) {
		t.Errorf("round trip = %+v, want %+v", again, parsed)
	}
}
