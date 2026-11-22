package app

// promote_nonrecoverable_tools.go holds the tool-name vocabulary and the question flow used
// when the source harness does not record tools and the operator supplies them.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mosaic-deploy/internal/domain"
)

// knownGenericToolVocab is the accepted vocabulary for QPromoteNonRecoverableTools.
// It covers the minimal read-only grant and the escalating tools so that operators
// who know their agent's tool requirements can supply them. Names outside this set
// are filtered out rather than accepted silently, so an unfamiliar name never reaches
// the promoted file unvetted.
var knownGenericToolVocab = map[string]bool{
	"file_read":      true,
	"file_search":    true,
	"content_search": true,
	"file_write":     true,
	"file_edit":      true,
	"terminal":       true,
	"subagent":       true,
	"bash":           true,
}

// knownGenericToolVocabSorted returns a sorted comma-separated list of the known
// generic tool vocabulary for display in re-ask prompts.
func knownGenericToolVocabSorted() string {
	names := make([]string, 0, len(knownGenericToolVocab))
	for name := range knownGenericToolVocab {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// parseGenericToolNames splits a comma-separated answer, normalises each entry
// (trim surrounding whitespace and lower-case), filters to knownGenericToolVocab,
// and returns the accepted deduplicated list. An empty or all-blank input returns
// a nil slice.
func parseGenericToolNames(text string) []string {
	parts := strings.Split(text, ",")
	var result []string
	seen := make(map[string]bool)
	for _, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == "" {
			continue
		}
		if knownGenericToolVocab[name] && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

// resolveNonRecoverablePromoteTools asks the user for a comma-separated list of
// generic tool names when the source harness cannot recover tool information. It
// normalises each entry (whitespace trim, lower-case) and filters against
// knownGenericToolVocab, accepting only recognised names. When the user provides
// a non-blank answer but every name is outside the vocabulary, the question is
// re-asked with the vocabulary listed; this repeats up to maxNonRecoverableReasks
// additional times before falling through to the empty-list warning path. A blank
// answer, a non-interactive session, or an unanswered question all return an
// explicit empty (non-nil) slice plus a NoticeWarning.
//
// maxNonRecoverableReasks limits re-ask iterations to keep the function
// terminating in tests that script a permanently-bad answer.
const maxNonRecoverableReasks = 3

func (s *service) resolveNonRecoverablePromoteTools(ctx context.Context, subject string) []string {
	tools, _ := s.askNonRecoverablePromoteTools(ctx, subject)
	return tools
}

// askNonRecoverablePromoteTools is resolveNonRecoverablePromoteTools that also reports whether
// the first question ended in a cancel or an interaction error, so a caller that treats a
// cancel as an abort can tell it from a skip. The returned tools are the same either way.
func (s *service) askNonRecoverablePromoteTools(ctx context.Context, subject string) (tools []string, cancelled bool) {
	title := "Source harness does not record tools. Enter comma-separated generic tool names (or leave blank for none):"

	for attempt := 0; attempt <= maxNonRecoverableReasks; attempt++ {
		q := domain.TextQuestion{
			Question: domain.Question{
				ID:        domain.QPromoteNonRecoverableTools,
				Subject:   subject,
				Title:     title,
				AllowSkip: true,
			},
		}
		ans, err := s.deps.Interaction.AskText(ctx, q)

		if err != nil || ans.Status != domain.Answered {
			// Non-interactive or unanswered: empty tools list with a warning.
			cancelled = err != nil || ans.Status == domain.Cancelled
			s.deps.Interaction.Notify(ctx, domain.Notice{
				Level:   domain.NoticeWarning,
				Message: fmt.Sprintf("agent %q: source harness does not record tool information; tools list left empty", subject),
			})
			return []string{}, cancelled
		}

		if strings.TrimSpace(ans.Text) == "" {
			// Blank answer: explicit empty tools list.
			return []string{}, cancelled
		}

		// Normalise (trim + lower-case) and filter to known vocabulary.
		result := parseGenericToolNames(ans.Text)
		if len(result) > 0 {
			return result, false
		}

		// All provided names were outside the known vocabulary. Re-ask with the
		// vocabulary listed so the user knows which names are accepted.
		title = fmt.Sprintf(
			"Unknown tool name(s). Known generic tools: %s. Enter comma-separated names (or leave blank for none):",
			knownGenericToolVocabSorted(),
		)
	}

	// Re-ask limit reached: emit warning and return empty list.
	s.deps.Interaction.Notify(ctx, domain.Notice{
		Level:   domain.NoticeWarning,
		Message: fmt.Sprintf("agent %q: no recognised tool names provided; tools list left empty", subject),
	})
	return []string{}, cancelled
}
