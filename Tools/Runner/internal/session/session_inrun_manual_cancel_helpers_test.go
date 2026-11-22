package session_test

// Fixtures for the in-run manual-dispatch cancel tests: a recorder of the
// notices the session sends, and a scripted cancel of the dialogue's row
// question that records the detail text it was shown.

import (
	"context"
	"strings"
	"sync"

	"mosaic-common/interaction"
	"mosaic-run/internal/domain"
)

const (
	manualNoticeTitle    = "Manual dispatch"
	manualCancelFragment = "cancelled"
	manualCancelHint     = "Esc cancels"
)

// noticeRecorder is a no-op interaction that records every notice.
type noticeRecorder struct {
	noopInteraction
	mu      sync.Mutex
	notices []interaction.Notice
}

func (n *noticeRecorder) Notify(_ context.Context, no interaction.Notice) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notices = append(n.notices, no)
}

// manualNotices returns the notices titled as the manual dispatch cancel notice.
func (n *noticeRecorder) manualNotices() []interaction.Notice {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []interaction.Notice
	for _, no := range n.notices {
		if no.Title == manualNoticeTitle {
			out = append(out, no)
		}
	}
	return out
}

// cancelRowRecordingDetail makes every row question of u end with Esc and
// returns the recorder of the detail text each row question was shown with.
func cancelRowRecordingDetail(u *manualUser) *rowDetails {
	rec := &rowDetails{}
	u.one[domain.QuestionManualRow] = []func(interaction.ChoiceQuestion) interaction.ChoiceAnswer{
		func(q interaction.ChoiceQuestion) interaction.ChoiceAnswer {
			rec.details = append(rec.details, q.Detail)
			return interaction.ChoiceAnswer{Status: interaction.Cancelled}
		},
	}
	return rec
}

// rowDetails holds the detail of each row question asked, in order.
type rowDetails struct {
	details []string
}

// hinted reports whether the i-th row question carried the Esc hint.
func (r *rowDetails) hinted(i int) bool {
	return i < len(r.details) && strings.Contains(r.details[i], manualCancelHint)
}

// cancelledEntries returns the debug entries of the request cancelled event.
func (r *inRunRig) cancelledEntries() []sessionLogEntry {
	return r.logger.entriesOf(domain.EventSessionRequestCancelled)
}
