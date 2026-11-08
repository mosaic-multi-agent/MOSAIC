package tui

// A status message delivered to the root model reaches the progress screen's
// current invocation. The screen keeps the status privately, so it is read by
// reflection.

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	"mosaic-run/internal/domain"
)

// screenActivityField returns the named field of the progress screen's
// activity panel.
func screenActivityField(m *rootModel, name string) reflect.Value {
	panel := reflect.ValueOf(m.progressScreen).Elem().FieldByName("activity")
	f := panel.FieldByName(name)
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

func TestRootModel_ActivityStatusMsg_ReachesTheProgressScreen(t *testing.T) {
	m := newTestModel()
	m.progressScreen = newProgressScreen(m)
	m.screen = screenProgress
	const key = 7
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	want := domain.ActivityStatus{Model: "routed-model", ContextTokens: 1234, HasContext: true}

	m.Update(activityStartedMsg{key: key, inv: activityInv})
	m.Update(activityStatusMsg{key: key, status: want, at: at})

	got := screenActivityField(m, "status").Interface().(domain.ActivityStatus)
	if got != want {
		t.Errorf("screen status = %+v, want %+v", got, want)
	}
	gotAt := screenActivityField(m, "statusAt").Interface().(time.Time)
	if !gotAt.Equal(at) {
		t.Errorf("screen status time = %v, want %v", gotAt, at)
	}
}

func TestRootModel_ActivityStatusMsg_WithoutProgressScreen_IsIgnored(t *testing.T) {
	m := newTestModel()
	m.progressScreen = nil

	m.Update(activityStatusMsg{key: 1, status: domain.ActivityStatus{Model: "x"}, at: time.Now()})
}
