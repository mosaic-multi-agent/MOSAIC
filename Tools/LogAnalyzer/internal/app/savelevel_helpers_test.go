package app_test

// Helpers shared by the tests of saving to a chosen configuration level.

import (
	"testing"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// levelQuestions returns the configuration-level questions asked, in order.
func levelQuestions(ia *scriptedInteraction) []interaction.ChoiceQuestion {
	var out []interaction.ChoiceQuestion
	for _, q := range ia.selectOneCalls {
		if q.ID == app.QuestionConfigLevel {
			out = append(out, q)
		}
	}
	return out
}

// firstIndexOf returns the position of the first asked question with id, or -1.
func firstIndexOf(asked []interaction.QuestionID, id interaction.QuestionID) int {
	for i, a := range asked {
		if a == id {
			return i
		}
	}
	return -1
}

// optionByID returns the option with the given id.
func optionByID(t *testing.T, q interaction.ChoiceQuestion, id string) interaction.Option {
	t.Helper()
	for _, o := range q.Options {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("question has no option %q; options: %+v", id, q.Options)
	return interaction.Option{}
}

// assertLevelQuestionShape checks the question every save flow asks: the two
// options with their labels and the file each would write, global as default,
// and skippable.
func assertLevelQuestionShape(t *testing.T, q interaction.ChoiceQuestion, paths domain.ConfigPaths) {
	t.Helper()
	if len(q.Options) != 2 {
		t.Fatalf("options = %+v, want exactly the global and the project option", q.Options)
	}
	global, project := optionByID(t, q, "global"), optionByID(t, q, "project")
	if global.Label != "All projects" || project.Label != "This project" {
		t.Errorf("labels = %q / %q, want %q / %q", global.Label, project.Label, "All projects", "This project")
	}
	if global.Description != paths.Global.Path {
		t.Errorf("global description = %q, want the global file %q", global.Description, paths.Global.Path)
	}
	if project.Description != paths.Project.Path {
		t.Errorf("project description = %q, want the project file %q", project.Description, paths.Project.Path)
	}
	if q.DefaultOptionID != "global" {
		t.Errorf("DefaultOptionID = %q, want global", q.DefaultOptionID)
	}
	if !q.AllowSkip {
		t.Error("the level question must be skippable")
	}
	if q.Prompt == "" {
		t.Error("the level question must have a prompt")
	}
}

// assertGlobalUnavailableShape checks the question when the global folder cannot
// be resolved: the global option is disabled with a reason and project is default.
func assertGlobalUnavailableShape(t *testing.T, q interaction.ChoiceQuestion) {
	t.Helper()
	global := optionByID(t, q, "global")
	if !global.Disabled || global.DisabledReason == "" {
		t.Errorf("global option = %+v, want Disabled with a reason", global)
	}
	if project := optionByID(t, q, "project"); project.Disabled {
		t.Error("the project option must stay selectable")
	}
	if q.DefaultOptionID != "project" {
		t.Errorf("DefaultOptionID = %q, want project when global is unavailable", q.DefaultOptionID)
	}
}
