package app_test

// promote_tool_question_test.go covers the generic tool name question as asked by both promote
// paths: its single-name Validate hook, its wording and its unchanged identity fields. The test
// doubles do not run Validate, so the tests capture the asked question and call it directly.

import (
	"strconv"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

// askedPromoteToolQuestions returns the question as asked by each path, keyed by path name.
func askedPromoteToolQuestions(t *testing.T) map[string]domain.TextQuestion {
	t.Helper()
	out := map[string]domain.TextQuestion{}

	_, capture := promoteTools(t, interactiontest.NewBuilder(), "my-custom-tool")
	for _, q := range capture.asked {
		if q.ID == domain.QPromoteCustomTool {
			out["single-file"] = q
		}
	}

	ls, spy := newToolBatch(t)
	mustRunBatch(t, ls.batchDeps(t, spy), batchToolFolder(t))
	if asks := spy.asked(domain.QPromoteCustomTool); len(asks) > 0 {
		out["batch"] = asks[0].Text
	}

	for _, path := range []string{"single-file", "batch"} {
		if _, ok := out[path]; !ok {
			t.Fatalf("%s path did not ask the promote tool question", path)
		}
	}
	return out
}

// requireValidate fails the test cleanly when the question has no Validate hook.
func requireValidate(t *testing.T, path string, q domain.TextQuestion) func(string) error {
	t.Helper()
	if q.Validate == nil {
		t.Fatalf("%s: question has no Validate hook", path)
	}
	return q.Validate
}

// TestPromoteToolQuestion_Validate_RefusesListsAndInnerWhitespaceInBothPaths verifies both paths
// refuse a comma or inner whitespace, naming the refused answer, and accept every other entry
// including empty and whitespace-only text.
func TestPromoteToolQuestion_Validate_RefusesListsAndInnerWhitespaceInBothPaths(t *testing.T) {
	cases := []struct {
		input   string
		refused bool
	}{
		{"a, b", true},
		{"a,b", true},
		{"a,", true},
		{",", true},
		{"bad name", true},
		{"bad\tname", true},
		{"  a b  ", true},
		{"file_read", false},
		{"  file_read  ", false},
		{"mcp/github/get_pr", false},
		{"mcp/github*", false},
		{"", false},
		{"   ", false},
		{"\t", false},
	}
	for path, q := range askedPromoteToolQuestions(t) {
		validate := requireValidate(t, path, q)
		for _, c := range cases {
			err := validate(c.input)
			if !c.refused {
				if err != nil {
					t.Errorf("%s: Validate(%q) = %v, want accepted", path, c.input, err)
				}
				continue
			}
			if err == nil {
				t.Errorf("%s: Validate(%q) accepted, want refused", path, c.input)
				continue
			}
			if want := strconv.Quote(strings.TrimSpace(c.input)); !strings.Contains(err.Error(), want) {
				t.Errorf("%s: Validate(%q) error %q does not name the refused answer %s", path, c.input, err, want)
			}
		}
	}
}

// TestPromoteToolQuestion_Wording_StatesOneNameAndSharingInBothPaths verifies the question tells
// the user it takes one generic tool name and that several harness tools may share a name, and
// keeps its identity fields and skip options.
func TestPromoteToolQuestion_Wording_StatesOneNameAndSharingInBothPaths(t *testing.T) {
	for path, q := range askedPromoteToolQuestions(t) {
		if !strings.Contains(strings.ToLower(q.Prompt), "one generic tool name") {
			t.Errorf("%s: Prompt = %q, want it to say the question takes one generic tool name", path, q.Prompt)
		}
		if !strings.Contains(strings.ToLower(q.Detail), "same name") {
			t.Errorf("%s: Detail = %q, want it to say several harness tools can share the same name", path, q.Detail)
		}
		if q.Subject == "" || q.Title != "Generic tool name for harness tool "+q.Subject {
			t.Errorf("%s: Subject = %q, Title = %q, want the unchanged title for the subject", path, q.Subject, q.Title)
		}
		if !q.AllowSkip || !q.AllowSkipAll {
			t.Errorf("%s: AllowSkip = %v, AllowSkipAll = %v, want both", path, q.AllowSkip, q.AllowSkipAll)
		}
	}
}
