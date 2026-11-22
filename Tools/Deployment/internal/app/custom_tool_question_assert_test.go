package app_test

// custom_tool_question_assert_test.go holds the assertions on the custom tool question shared by
// the deploy-family and the transform tests. Nothing here is a test itself.

import (
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/customtool"
	"mosaic-deploy/internal/domain"
)

// assertListSyntaxQuestion checks that the question teaches the list syntax: entries separated
// by commas, "server" for a whole server, "server/tool" for one tool, a valid mixed example,
// and no request for harness-specific names. It stays skippable.
func assertListSyntaxQuestion(t *testing.T, q domain.TextQuestion) {
	t.Helper()
	text := q.Title + "\n" + q.Prompt + "\n" + q.Detail
	lower := strings.ToLower(text)
	for _, want := range []string{"comma", "server/tool"} {
		if !strings.Contains(lower, want) {
			t.Errorf("question text does not state the syntax (missing %q): %q", want, text)
		}
	}
	if !hasMixedExample(text) {
		t.Errorf("question text holds no example that fits the syntax and mixes a whole server with a single tool: %q", text)
	}
	if strings.Contains(text, "mcp__") || strings.Contains(text, "bare MCP server name") {
		t.Errorf("question text still asks for harness-specific or bare-server input: %q", text)
	}
	if !q.AllowSkip || !q.AllowSkipAll {
		t.Error("the question must stay skippable (AllowSkip and AllowSkipAll)")
	}
}

// hasMixedExample reports whether any line of text, or the part of a line after a colon, parses
// as a list that holds both a whole server and a single tool. Where the example sits in the
// question text is not part of the contract.
func hasMixedExample(text string) bool {
	var candidates []string
	for _, line := range strings.Split(text, "\n") {
		candidates = append(candidates, line)
		if i := strings.LastIndex(line, ":"); i >= 0 {
			candidates = append(candidates, line[i+1:])
		}
	}
	for _, c := range candidates {
		c = strings.Trim(strings.TrimSpace(c), "`\"'.")
		if !strings.Contains(c, ",") || !strings.Contains(c, "/") {
			continue
		}
		entries, err := customtool.Parse(c)
		if err != nil {
			continue
		}
		if slices.ContainsFunc(entries, func(e domain.MCPEntry) bool { return e.Tool == "" }) &&
			slices.ContainsFunc(entries, func(e domain.MCPEntry) bool { return e.Tool != "" }) {
			return true
		}
	}
	return false
}

// assertListSyntaxValidate checks that validate refuses answers that do not fit the list syntax
// (naming the entry) and accepts the ones that do, a blank answer included.
func assertListSyntaxValidate(t *testing.T, validate func(string) error) {
	t.Helper()
	for _, bad := range []string{"github/a/b", "a,,b", "/tool", "server/", "bad name", "a, b/c d"} {
		if err := validate(bad); err == nil {
			t.Errorf("Validate(%q) = nil, want a refusal", bad)
		}
	}
	if err := validate("github/a/b"); err == nil || !strings.Contains(err.Error(), "github/a/b") {
		t.Errorf("Validate(%q) = %v, want an error naming the entry", "github/a/b", err)
	}
	for _, good := range []string{"", "   ", "jira", "github/get_pr, jira", "github/*"} {
		if err := validate(good); err != nil {
			t.Errorf("Validate(%q) = %v, want accepted", good, err)
		}
	}
}
