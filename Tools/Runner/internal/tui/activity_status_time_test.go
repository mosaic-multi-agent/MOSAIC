package tui

// The program feed stamps each forwarded status with its receipt time, so the
// screen knows when the last stream event arrived. The message shape is not
// fixed, so the time is located by reflection: the first time.Time field of
// the status message.

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// receiptTime returns the first time.Time field of the message struct.
func receiptTime(m tea.Msg) (time.Time, bool) {
	v := reflect.ValueOf(m)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return time.Time{}, false
	}
	cp := reflect.New(v.Type()).Elem()
	cp.Set(v)
	for i := 0; i < cp.NumField(); i++ {
		f := cp.Field(i)
		if f.Type() == reflect.TypeOf(time.Time{}) {
			f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
			return f.Interface().(time.Time), true
		}
	}
	return time.Time{}, false
}

func TestProgramRef_Status_CarriesTheReceiptTime_NonDecreasing(t *testing.T) {
	ref, rec := startRecorderProgram(t, nil)
	feed := ref.InvocationStarted(activityInv)
	start := time.Now()

	feed.Status(domain.ActivityStatus{Model: "status-model-x", ContextTokens: 1, HasContext: true})
	time.Sleep(20 * time.Millisecond)
	feed.Status(domain.ActivityStatus{Model: "status-model-x", ContextTokens: 2, HasContext: true})
	end := time.Now()

	msgs := waitRecorded(t, rec, "both statuses", func(m []tea.Msg) bool {
		p, _ := statusMsgs(m)
		return len(p) >= 2
	})
	_, positions := statusMsgs(msgs)
	var prev time.Time
	for n, pos := range positions {
		got, ok := receiptTime(msgs[pos])
		if !ok {
			t.Fatalf("status message %d carries no time.Time", n+1)
		}
		if got.Before(start) || got.After(end.Add(time.Second)) {
			t.Errorf("status %d receipt time %v outside [%v, %v]", n+1, got, start, end)
		}
		if got.Before(prev) {
			t.Errorf("receipt time went backwards: %v after %v", got, prev)
		}
		prev = got
	}
	feed.Ended()
}
