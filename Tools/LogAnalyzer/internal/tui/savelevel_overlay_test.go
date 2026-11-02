package tui

// Tests for the level question as the TUI shows it: an overlay whose options carry
// the file each would write, whose cursor starts on the default option, and whose
// answer flows back to the waiting flow.

import (
	"strings"
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
)

const (
	levelGlobalFile  = "/home/u/.config/MOSAIC/LogAnalyzer/pricing.yaml"
	levelProjectFile = "/work/MosaicLogAnalyzer/config/pricing.yaml"
)

// levelQuestion is the question the app flows ask; globalDisabled marks the
// global option unavailable and makes project the default.
func levelQuestion(globalDisabled bool) interaction.ChoiceQuestion {
	global := interaction.Option{ID: "global", Label: "All projects", Description: levelGlobalFile}
	def := "global"
	if globalDisabled {
		global = interaction.Option{ID: "global", Label: "All projects", Disabled: true, DisabledReason: "user config folder not found"}
		def = "project"
	}
	return interaction.ChoiceQuestion{
		Question: interaction.Question{
			ID: app.QuestionConfigLevel, Title: "Save location", Prompt: "Save to all projects or this project?",
			AllowSkip: true, DefaultOptionID: def,
		},
		Options: []interaction.Option{global, {ID: "project", Label: "This project", Description: levelProjectFile}},
	}
}

// askLevel delivers the level question to a model and returns the model showing
// it and the channel the answer arrives on.
func askLevel(t *testing.T, q interaction.ChoiceQuestion) (Model, chan answerMsg) {
	t.Helper()
	reply := make(chan answerMsg, 1)
	m := newNavModel()
	m.width, m.height = 120, 30
	updated, _ := m.Update(Pending{id: q.ID, kind: pendingSelectOne, choiceQ: q, reply: reply})
	return asModel(updated), reply
}

// answerOf returns the select answer waiting on reply, failing when none was sent.
func answerOf(t *testing.T, reply chan answerMsg) interaction.ChoiceAnswer {
	t.Helper()
	select {
	case a := <-reply:
		return a.choiceAns
	default:
		t.Fatal("no answer was sent to the waiting flow")
		return interaction.ChoiceAnswer{}
	}
}

func TestLevelQuestion_OverlayShowsThePromptBothLabelsAndTheirFiles(t *testing.T) {
	m, _ := askLevel(t, levelQuestion(false))

	view := m.View()

	for _, want := range []string{"Save to all projects or this project?", "All projects", "This project", levelGlobalFile, levelProjectFile} {
		if !strings.Contains(view, want) {
			t.Errorf("the overlay must show %q\n%s", want, view)
		}
	}
}

func TestLevelQuestion_EnterOnTheDefaultAnswersGlobal(t *testing.T) {
	m, reply := askLevel(t, levelQuestion(false))

	send(m, "enter")

	if ans := answerOf(t, reply); ans.Status != interaction.Answered || ans.OptionID != "global" {
		t.Errorf("answer = %+v, want global", ans)
	}
}

func TestLevelQuestion_DownThenEnterAnswersProject(t *testing.T) {
	m, reply := askLevel(t, levelQuestion(false))

	send(m, "down", "enter")

	if ans := answerOf(t, reply); ans.Status != interaction.Answered || ans.OptionID != "project" {
		t.Errorf("answer = %+v, want project", ans)
	}
}

func TestLevelQuestion_WhenGlobalIsUnavailableTheCursorStartsOnProject(t *testing.T) {
	m, reply := askLevel(t, levelQuestion(true))

	send(m, "enter")

	if ans := answerOf(t, reply); ans.Status != interaction.Answered || ans.OptionID != "project" {
		t.Errorf("answer = %+v, want project to be the default when global is disabled", ans)
	}
}

func TestLevelQuestion_SkipEntryAnswersSkipped(t *testing.T) {
	m, reply := askLevel(t, levelQuestion(false))

	send(m, "down", "down", "enter")

	if ans := answerOf(t, reply); ans.Status != interaction.SkippedOne {
		t.Errorf("answer = %+v, want skipped", ans)
	}
}

func TestLevelQuestion_EscCancelsTheQuestion(t *testing.T) {
	m, reply := askLevel(t, levelQuestion(false))

	send(m, "esc")

	if ans := answerOf(t, reply); ans.Status != interaction.Cancelled {
		t.Errorf("answer = %+v, want cancelled", ans)
	}
}
