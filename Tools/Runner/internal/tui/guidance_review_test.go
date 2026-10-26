package tui

// The run-start guidance review: the user sees the pre-consultation result in
// full (also when empty) with Accept / Edit, and on resume with Accept / Edit /
// Regenerate; Edit asks for each field through multi-line entry.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

func guidanceReviewQuestion(detail string, withRegenerate bool) interaction.ChoiceQuestion {
	opts := []interaction.Option{
		{ID: domain.GuidanceOptionAccept, Label: "Accept"},
		{ID: domain.GuidanceOptionEdit, Label: "Edit"},
	}
	if withRegenerate {
		opts = append(opts, interaction.Option{ID: domain.GuidanceOptionRegenerate, Label: "Regenerate"})
	}
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID:     domain.QuestionGuidanceReview,
			Title:  "Review standing guidance",
			Prompt: "Review the guidance",
			Detail: detail,
		},
		Options: opts,
	}
}

func guidanceDetail(task, constraints string) string {
	if task == "" {
		task = "(empty)"
	}
	if constraints == "" {
		constraints = "(empty)"
	}
	return "Appended to task_description:\n" + task + "\n\nAppended to constraints:\n" + constraints
}

// openGuidanceReview shows the review over the progress screen in a window of
// the given size.
func openGuidanceReview(t *testing.T, w, h int, q interaction.ChoiceQuestion) (*rootModel, chan answerMsg) {
	t.Helper()
	m := newTestModel()
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	reply := make(chan answerMsg, 1)
	m.Update(questionMsg{kind: questionSelectOne, choiceQ: q, reply: reply})
	if m.screen != screenQuestion {
		t.Fatalf("precondition: review not open, screen = %v", m.screen)
	}
	return m, reply
}

func TestGuidanceReview_ShowsEveryDetailLineBetweenPromptAndOptions(t *testing.T) {
	q := guidanceReviewQuestion(guidanceDetail("alpha one\nalpha two\n\nalpha four", "beta one"), false)
	m, _ := openGuidanceReview(t, 100, 40, q)

	view := m.View()

	prev := strings.Index(view, "Review the guidance")
	for _, want := range []string{"Appended to task_description:", "alpha one", "alpha two", "alpha four",
		"Appended to constraints:", "beta one", "Accept", "Edit"} {
		at := strings.Index(view, want)
		if at < 0 {
			t.Fatalf("view lacks %q:\n%s", want, view)
		}
		if at < prev {
			t.Errorf("%q appears before an earlier part of the review, want prompt, detail, then options:\n%s", want, view)
		}
		prev = at
	}
	if strings.Contains(view, "Regenerate") {
		t.Error("Regenerate is offered for a freshly generated result")
	}
}

func TestGuidanceReview_EmptyResultIsShownAsEmptyWithAcceptAndEdit(t *testing.T) {
	m, reply := openGuidanceReview(t, 100, 40, guidanceReviewQuestion(guidanceDetail("", ""), false))

	view := m.View()
	if strings.Count(view, "(empty)") != 2 {
		t.Errorf("view shows %d empty markers, want 2 (one per field):\n%s", strings.Count(view, "(empty)"), view)
	}

	sendKey(m, tea.KeyDown)
	sendKey(m, tea.KeyEnter)
	if got := takeReplyChoice(t, reply); got != domain.GuidanceOptionEdit {
		t.Errorf("second option = %q, want %q so guidance can be added to an empty result", got, domain.GuidanceOptionEdit)
	}
}

func takeReplyChoice(t *testing.T, reply chan answerMsg) string {
	t.Helper()
	ans := takeTextReply(t, reply).choiceAns
	if ans.Status != interaction.Answered {
		t.Fatalf("choice Status = %q, want %q", ans.Status, interaction.Answered)
	}
	return ans.OptionID
}

func TestGuidanceReview_ResumeOffersAcceptEditRegenerate(t *testing.T) {
	cases := []struct {
		name string
		keys []tea.KeyType
		want string
	}{
		{"accept", []tea.KeyType{tea.KeyEnter}, domain.GuidanceOptionAccept},
		{"edit", []tea.KeyType{tea.KeyDown, tea.KeyEnter}, domain.GuidanceOptionEdit},
		{"regenerate", []tea.KeyType{tea.KeyDown, tea.KeyDown, tea.KeyEnter}, domain.GuidanceOptionRegenerate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, reply := openGuidanceReview(t, 100, 40, guidanceReviewQuestion(guidanceDetail("saved", "text"), true))
			if view := m.View(); !strings.Contains(view, "Regenerate") || !strings.Contains(view, "saved") {
				t.Fatalf("resume review lacks Regenerate or the saved text:\n%s", view)
			}

			sendKeys(m, c.keys...)

			if got := takeReplyChoice(t, reply); got != c.want {
				t.Errorf("answer = %q, want %q", got, c.want)
			}
		})
	}
}

func TestGuidanceReview_RegenerateLeadsToReviewOfTheNewResult(t *testing.T) {
	m, reply := openGuidanceReview(t, 100, 40, guidanceReviewQuestion(guidanceDetail("old saved text", ""), true))
	sendKeys(m, tea.KeyDown, tea.KeyDown, tea.KeyEnter)
	if got := takeReplyChoice(t, reply); got != domain.GuidanceOptionRegenerate {
		t.Fatalf("precondition: answer = %q, want regenerate", got)
	}

	next := make(chan answerMsg, 1)
	m.Update(questionMsg{kind: questionSelectOne, choiceQ: guidanceReviewQuestion(guidanceDetail("fresh generated text", ""), false), reply: next})

	view := m.View()
	if !strings.Contains(view, "fresh generated text") || strings.Contains(view, "old saved text") {
		t.Errorf("the second review does not show only the new result:\n%s", view)
	}
	if strings.Contains(view, "Regenerate") {
		t.Error("the review of a regenerated result still offers Regenerate")
	}
}

func TestGuidanceReview_LongLinesWrapWithoutLosingWords(t *testing.T) {
	long := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen"
	blob := strings.Repeat("Z", 95)
	m, _ := openGuidanceReview(t, 40, 40, guidanceReviewQuestion(guidanceDetail(long+"\n"+blob, ""), false))

	view := m.View()

	for _, w := range strings.Fields(long) {
		if !strings.Contains(view, w) {
			t.Errorf("wrapped detail lost the word %q:\n%s", w, view)
		}
	}
	if got := strings.Count(view, "Z"); got != 95 {
		t.Errorf("view holds %d of the 95 characters of an over-wide word, want all", got)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("line of width %d exceeds the 40-column overlay: %q", w, line)
		}
	}
}

func TestGuidanceReview_TallDetailScrollsAndKeepsOptionsVisible(t *testing.T) {
	const total = 40
	lines := make([]string, total)
	for i := range lines {
		lines[i] = fmt.Sprintf("detail-row-%02d", i+1)
	}
	m, _ := openGuidanceReview(t, 80, 16, guidanceReviewQuestion(strings.Join(lines, "\n"), true))

	seen := map[string]bool{}
	record := func() string {
		view := m.View()
		if n := strings.Count(view, "\n") + 1; n > 16 {
			t.Errorf("view is %d lines tall in a 16-line window", n)
		}
		for _, o := range []string{"Accept", "Edit", "Regenerate"} {
			if !strings.Contains(view, o) {
				t.Errorf("option %q not visible while scrolling:\n%s", o, view)
			}
		}
		for _, l := range lines {
			if strings.Contains(view, l) {
				seen[l] = true
			}
		}
		return view
	}

	first := record()
	if !strings.Contains(first, lines[0]) {
		t.Errorf("the detail region does not start at its first line:\n%s", first)
	}
	if strings.Contains(first, lines[total-1]) {
		t.Fatal("a 40-line detail fits a 16-line window; the overlay is not bounding its height")
	}
	if !strings.Contains(first, "pgup/pgdn scroll") {
		t.Errorf("help line lacks the scroll hint:\n%s", first)
	}
	for i := 0; i < total && !seen[lines[total-1]]; i++ {
		sendKey(m, tea.KeyPgDown)
		record()
	}
	for _, l := range lines {
		if !seen[l] {
			t.Errorf("detail line %q was never reachable with pgdown", l)
		}
	}
	for i := 0; i < total; i++ {
		sendKey(m, tea.KeyPgUp)
	}
	if view := record(); !strings.Contains(view, lines[0]) {
		t.Errorf("pgup does not scroll back to the first detail line:\n%s", view)
	}
}

func TestGuidanceReview_ShortDetailHasNoScrollHint(t *testing.T) {
	m, _ := openGuidanceReview(t, 100, 40, guidanceReviewQuestion(guidanceDetail("a", "b"), false))

	if view := m.View(); strings.Contains(view, "pgup/pgdn scroll") {
		t.Errorf("scroll hint shown for a detail that fits:\n%s", view)
	}
}

// Edit asks for the task_description field and then the constraints field
// through multi-line entry, each pre-filled with the current text; what the
// user enters reaches the session with its line breaks.
func TestGuidanceReview_EditFieldsAreMultilineAndReachTheSessionIntact(t *testing.T) {
	for _, tc := range []struct {
		id      interaction.QuestionID
		current string
		typed   []string
		want    string
	}{
		{domain.QuestionGuidanceTask, "keep this", []string{"", "", "new paragraph", "second line"}, "keep this\n\nnew paragraph\nsecond line"},
		{domain.QuestionGuidanceConstraints, "", []string{"first", "", "third"}, "first\n\nthird"},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			q := interaction.TextQuestion{
				Question: interaction.Question{ID: tc.id, Title: "Edit guidance"},
				Default:  tc.current,
			}
			m, reply := openMultilineQuestion(t, q)

			typeLines(m, tc.typed...)
			assertNoTextReply(t, reply, "before ctrl+s")
			sendKey(m, tea.KeyCtrlS)

			if got := takeTextReply(t, reply).textAns.Text; got != tc.want {
				t.Errorf("Text = %q, want %q", got, tc.want)
			}
		})
	}
}
