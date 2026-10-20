package screens_test

// stack_notice_test.go verifies the migration notice on the catalog-stack screen: text set with
// SetNotice is shown, an empty text clears it, and it survives editing the stack.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
)

const noticeText = "NOTICE-SENTINEL custom catalogs no longer get default agents"

func TestStackScreen_SetNotice_ShowsTheNotice(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{UserLayers: catalogDirs(t, 1)})

	s.SetNotice(noticeText)

	if !strings.Contains(s.View(), noticeText) {
		t.Errorf("the notice is not shown:\n%s", s.View())
	}
}

func TestStackScreen_NoNotice_ByDefault(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{UserLayers: catalogDirs(t, 1)})

	if strings.Contains(s.View(), "NOTICE-SENTINEL") {
		t.Errorf("a notice is shown although none was set:\n%s", s.View())
	}
}

func TestStackScreen_SetNotice_EmptyTextClearsTheNotice(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{UserLayers: catalogDirs(t, 1)})
	s.SetNotice(noticeText)
	if !strings.Contains(s.View(), noticeText) {
		t.Fatalf("precondition: the notice is not shown:\n%s", s.View())
	}

	s.SetNotice("")

	if strings.Contains(s.View(), "NOTICE-SENTINEL") {
		t.Errorf("the notice is still shown after clearing it:\n%s", s.View())
	}
}

func TestStackScreen_SetNotice_StaysAfterAnEdit(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{UserLayers: catalogDirs(t, 1)})
	s.SetNotice(noticeText)

	s.Update(tea.KeyMsg{Type: tea.KeyDown})
	s.Update(tea.KeyMsg{Type: tea.KeySpace})

	if !strings.Contains(s.View(), noticeText) {
		t.Errorf("the notice disappeared after the user edited the stack:\n%s", s.View())
	}
}
