package screens_test

// stack_test.go verifies the catalog-stack screen: building and changing a stack (add, remove,
// reorder catalogs, keep or skip the default layer), the stack description it yields on
// confirmation, starting from a given stack, and the navigation contract shared with the other
// entry screens (Done, Back, Reset, external error).
//
// Key scheme these tests drive (the screen's contract with its callers):
//
//	up / down            move the cursor over the rows: user layers highest first, then the
//	                     default layer as the bottom row
//	a                    start path entry; typed text fills the path field
//	enter (in entry)     add the typed folder as the next user layer, below the layers already
//	                     listed and above the default layer; an invalid folder is refused inline
//	esc (in entry)       cancel the entry; this is not "back"
//	d                    remove the user layer under the cursor (no effect on the default row)
//	space                keep or leave out the default layer when the cursor is on its row
//	shift+up/shift+down  move the user layer under the cursor one step up or down in precedence;
//	                     a layer never moves below the default row
//	enter (not entry)    confirm the stack; refused inline while the stack would be empty
//	esc (not entry)      go back without confirming

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-deploy/internal/app/stackconfig"
	"mosaic-deploy/internal/tui/screens"
)

// stackScreenWidth is wide enough that no row wraps, so a row's text stays on one line.
const stackScreenWidth = 300

func newTestStackScreen(start stackconfig.Description) *screens.StackScreen {
	return screens.NewStackScreen(start, stackScreenWidth, 40, plainStyles())
}

// catalogDirs returns n distinct existing directories (cleaned, absolute).
func catalogDirs(t *testing.T, n int) []string {
	t.Helper()
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = filepath.Clean(t.TempDir())
	}
	return dirs
}

func press(s *screens.StackScreen, keys ...tea.KeyType) {
	for _, k := range keys {
		s.Update(tea.KeyMsg{Type: k})
	}
}

func pressRune(s *screens.StackScreen, r rune) {
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// addFolder drives the path entry: start entry, type path, Enter.
func addFolder(s *screens.StackScreen, path string) {
	pressRune(s, 'a')
	for _, r := range path {
		pressRune(s, r)
	}
	press(s, tea.KeyEnter)
}

// confirmed presses Enter and returns the yielded description; it fails when the screen is not Done.
func confirmed(t *testing.T, s *screens.StackScreen) stackconfig.Description {
	t.Helper()
	press(s, tea.KeyEnter)
	if !s.Done() {
		t.Fatalf("Done() = false after confirming; view:\n%s", s.View())
	}
	return s.Description()
}

func requireDescription(t *testing.T, got stackconfig.Description, layers []string, includeDefault bool) {
	t.Helper()
	if got.IncludeDefault != includeDefault || len(got.UserLayers) != len(layers) ||
		(len(layers) > 0 && !reflect.DeepEqual(got.UserLayers, layers)) {
		t.Errorf("Description = %+v, want UserLayers %v, IncludeDefault %v", got, layers, includeDefault)
	}
}

func TestStackScreen_ConfirmWithoutChanges_YieldsTheStartingStack(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	got := confirmed(t, s)

	requireDescription(t, got, d, true)
	if s.Changed() {
		t.Error("Changed() = true for a confirmed unchanged stack, want false")
	}
}

func TestStackScreen_AddFolder_AppendsALayerBelowTheListedOnes(t *testing.T) {
	d := catalogDirs(t, 3)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d[:1], IncludeDefault: true})

	addFolder(s, d[1])
	addFolder(s, d[2])

	requireDescription(t, confirmed(t, s), d, true)
	if !s.Changed() {
		t.Error("Changed() = false after adding layers, want true")
	}
}

func TestStackScreen_AddFolder_StoresTheCleanedAbsolutePath(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})

	addFolder(s, filepath.Join(d[0], "sub", ".."))

	requireDescription(t, confirmed(t, s), d, true)
}

func TestStackScreen_AddFolder_MissingFolderIsRefusedAndNotAdded(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})
	missing := filepath.Join(d[0], "does-not-exist")

	addFolder(s, missing)
	press(s, tea.KeyEsc) // leave the entry
	got := confirmed(t, s)

	requireDescription(t, got, d, true)
}

func TestStackScreen_CancelledEntry_IsNotBackNavigation(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})

	pressRune(s, 'a')
	press(s, tea.KeyEsc)

	if s.Back() {
		t.Error("Back() = true after cancelling a path entry, want false")
	}
	if s.Done() {
		t.Error("Done() = true after cancelling a path entry, want false")
	}
	press(s, tea.KeyEsc)
	if !s.Back() {
		t.Error("Back() = false after a second Esc outside the entry, want true")
	}
}

func TestStackScreen_RemoveLayer_DropsTheLayerUnderTheCursor(t *testing.T) {
	d := catalogDirs(t, 3)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown)
	pressRune(s, 'd')

	requireDescription(t, confirmed(t, s), []string{d[0], d[2]}, true)
}

func TestStackScreen_RemoveOnTheDefaultRow_LeavesTheStackUnchanged(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown) // the default row
	pressRune(s, 'd')

	requireDescription(t, confirmed(t, s), d, true)
}

func TestStackScreen_ReorderDown_SwapsWithTheNextLayer(t *testing.T) {
	d := catalogDirs(t, 3)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyShiftDown)

	requireDescription(t, confirmed(t, s), []string{d[1], d[0], d[2]}, true)
}

func TestStackScreen_ReorderUp_SwapsWithThePreviousLayer(t *testing.T) {
	d := catalogDirs(t, 3)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown, tea.KeyDown, tea.KeyShiftUp)

	requireDescription(t, confirmed(t, s), []string{d[0], d[2], d[1]}, true)
}

func TestStackScreen_ReorderUpAtTheTop_LeavesTheOrder(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyShiftUp)

	requireDescription(t, confirmed(t, s), d, true)
}

func TestStackScreen_ReorderDownOnTheLastUserLayer_NeverMovesBelowTheDefault(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown, tea.KeyShiftDown)

	got := confirmed(t, s)
	requireDescription(t, got, d, true)
	view := s.View()
	if strings.Index(view, d[1]) > strings.Index(view, "(default)") {
		t.Errorf("the default layer must stay the bottom row; view:\n%s", view)
	}
}

func TestStackScreen_ToggleDefault_LeavesTheDefaultLayerOutAndBack(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown)
	pressRune(s, ' ')
	if !s.Changed() {
		t.Error("Changed() = false after leaving the default layer out, want true")
	}
	left := confirmed(t, s)

	requireDescription(t, left, d, false)

	s.Reset(stackconfig.Description{UserLayers: d, IncludeDefault: false})
	press(s, tea.KeyDown)
	pressRune(s, ' ')
	requireDescription(t, confirmed(t, s), d, true)
}

func TestStackScreen_ToggleTwice_RestoresTheStartingStack(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	press(s, tea.KeyDown)
	pressRune(s, ' ')
	pressRune(s, ' ')

	requireDescription(t, confirmed(t, s), d, true)
	if s.Changed() {
		t.Error("Changed() = true after toggling back to the start, want false")
	}
}

func TestStackScreen_StartWithDefaultKept_ShowsTheDefaultAsBottomLayer(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	view := s.View()

	if !strings.Contains(view, "(default)") {
		t.Errorf("view must show the default layer; got:\n%s", view)
	}
	if strings.Index(view, d[0]) > strings.Index(view, "(default)") {
		t.Errorf("the default layer must be listed below the user layers; got:\n%s", view)
	}
}

func TestStackScreen_StartWithDefaultLeftOut_StillShowsTheDefaultRowAndYieldsNoDefault(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: false})

	view := s.View()

	if !strings.Contains(view, "(default)") {
		t.Errorf("the default layer is offered every time, left out or not; got:\n%s", view)
	}
	requireDescription(t, confirmed(t, s), d, false)
}

func TestStackScreen_DefaultToggleStateIsVisibleInTheView(t *testing.T) {
	d := catalogDirs(t, 1)
	kept := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true}).View()
	out := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: false}).View()

	if kept == out {
		t.Error("the view is identical for a kept and a left-out default layer; the state must be visible")
	}
}

func TestStackScreen_DefaultOnlyStart_AllowsAddingYourOwnCatalogAndDroppingTheDefault(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})

	addFolder(s, d[0])
	press(s, tea.KeyDown, tea.KeyDown) // past the user layer onto the default row
	pressRune(s, ' ')

	requireDescription(t, confirmed(t, s), d, false)
}

func TestStackScreen_EmptyStack_IsRefusedUntilALayerIsKept(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})

	pressRune(s, ' ') // the cursor starts on the only row, the default
	press(s, tea.KeyEnter)
	if s.Done() {
		t.Fatal("Done() = true for a stack with no layers at all, want it refused")
	}
	pressRune(s, ' ')

	requireDescription(t, confirmed(t, s), nil, true)
}

func TestStackScreen_ViewListsLayersHighestFirstAndNamesTheOrder(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})

	view := s.View()

	i0, i1, iDefault := strings.Index(view, d[0]), strings.Index(view, d[1]), strings.Index(view, "(default)")
	if i0 < 0 || !(i0 < i1 && i1 < iDefault) {
		t.Errorf("rows must read highest first, default last; got:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "highest") {
		t.Errorf("the view must say which end of the list wins (\"highest\"); got:\n%s", view)
	}
}

func TestStackScreen_Back_WithoutConfirming(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})

	press(s, tea.KeyEsc)

	if !s.Back() {
		t.Error("Back() = false after Esc, want true")
	}
	if s.Done() {
		t.Error("Done() = true after Esc, want false")
	}
}

func TestStackScreen_Reset_StartsOverFromTheGivenStackAndClearsTheFlags(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d[:1], IncludeDefault: true})
	press(s, tea.KeyEsc)

	s.Reset(stackconfig.Description{UserLayers: d[1:], IncludeDefault: false})

	if s.Back() || s.Done() {
		t.Errorf("Back() = %v, Done() = %v after Reset, want both false", s.Back(), s.Done())
	}
	requireDescription(t, confirmed(t, s), d[1:], false)
	if s.Changed() {
		t.Error("Changed() = true right after Reset and confirm, want false: the new start is the reference")
	}
}

func TestStackScreen_ExternalError_IsShownAndKeepsTheScreenActive(t *testing.T) {
	d := catalogDirs(t, 1)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d, IncludeDefault: true})
	confirmed(t, s)

	s.SetExternalError("catalog layer failed to load: boom")

	if s.Done() || s.Back() {
		t.Errorf("Done() = %v, Back() = %v after SetExternalError, want the screen active (both false)", s.Done(), s.Back())
	}
	if !strings.Contains(s.View(), "catalog layer failed to load: boom") {
		t.Errorf("the external error must be shown; got:\n%s", s.View())
	}
}

func TestStackScreen_ExternalError_ClearsOnTheNextInteraction(t *testing.T) {
	s := newTestStackScreen(stackconfig.Description{IncludeDefault: true})
	s.SetExternalError("unresolvable layer")
	if !strings.Contains(s.View(), "unresolvable layer") {
		t.Fatalf("the external error must be shown first; got:\n%s", s.View())
	}

	press(s, tea.KeyDown)

	if strings.Contains(s.View(), "unresolvable layer") {
		t.Errorf("the external error must clear on the next user interaction; got:\n%s", s.View())
	}
}

func TestStackScreen_ExternalError_KeepsTheEditedStackForAnotherTry(t *testing.T) {
	d := catalogDirs(t, 2)
	s := newTestStackScreen(stackconfig.Description{UserLayers: d[:1], IncludeDefault: true})
	addFolder(s, d[1])
	confirmed(t, s)
	s.SetExternalError("rebuild failed")

	press(s, tea.KeyDown) // clears the message and continues editing

	requireDescription(t, confirmed(t, s), d, true)
}
