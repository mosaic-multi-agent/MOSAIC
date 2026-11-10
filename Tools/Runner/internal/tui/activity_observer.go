package tui

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/tui/screens/runflow"
)

// maxPendingActivityItems bounds the item messages queued for a program that
// is not consuming; when exceeded the oldest queued items are dropped. Start
// and end messages are never dropped.
const maxPendingActivityItems = 512

// activityStartedMsg announces a new invocation whose activity follows.
type activityStartedMsg struct {
	key runflow.ActivityKey
	inv domain.ActivityInvocation
}

// activityItemMsg carries one activity item of the invocation key.
type activityItemMsg struct {
	key  runflow.ActivityKey
	item domain.ActivityItem
}

// activityStatusMsg carries the latest stream status of the invocation key.
// Statuses are latest-wins: the sender keeps at most one queued per invocation
// and never drops it as an item.
type activityStatusMsg struct {
	key    runflow.ActivityKey
	status domain.ActivityStatus
	at     time.Time
}

// activityEndedMsg announces that the invocation key finished.
type activityEndedMsg struct {
	key runflow.ActivityKey
}

// activitySender hands activity messages to the program from a background
// goroutine, so reporting never waits for the program's message loop.
type activitySender struct {
	mu      sync.Mutex
	nextKey uint64
	queue   []tea.Msg
	items   int // item messages in queue
	running bool
}

// newKey returns a key no other invocation of this sender has.
func (a *activitySender) newKey() runflow.ActivityKey {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextKey++
	return runflow.ActivityKey(a.nextKey)
}

// post queues msg for p and returns at once.
func (a *activitySender) post(p *tea.Program, msg tea.Msg) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if st, isStatus := msg.(activityStatusMsg); isStatus && a.replaceQueuedStatus(st) {
		return
	}
	if _, isItem := msg.(activityItemMsg); isItem {
		if a.items >= maxPendingActivityItems {
			a.dropOldestItem()
		}
		a.items++
	}
	a.queue = append(a.queue, msg)
	if !a.running {
		a.running = true
		go a.drain(p)
	}
}

// replaceQueuedStatus overwrites the still-queued status of the same
// invocation with st and reports whether there was one. Caller holds mu.
func (a *activitySender) replaceQueuedStatus(st activityStatusMsg) bool {
	for i := len(a.queue) - 1; i >= 0; i-- {
		if q, ok := a.queue[i].(activityStatusMsg); ok && q.key == st.key {
			a.queue[i] = st
			return true
		}
	}
	return false
}

// dropOldestItem removes the oldest queued item message. Caller holds mu.
func (a *activitySender) dropOldestItem() {
	for i, m := range a.queue {
		if _, isItem := m.(activityItemMsg); isItem {
			a.queue = append(a.queue[:i], a.queue[i+1:]...)
			a.items--
			return
		}
	}
}

// drain sends the queued messages in order until the queue is empty.
func (a *activitySender) drain(p *tea.Program) {
	for {
		a.mu.Lock()
		if len(a.queue) == 0 {
			a.running = false
			a.mu.Unlock()
			return
		}
		msg := a.queue[0]
		a.queue = a.queue[1:]
		if _, isItem := msg.(activityItemMsg); isItem {
			a.items--
		}
		a.mu.Unlock()
		p.Send(msg)
	}
}

// InvocationStarted implements domain.ActivityObserver. The returned feed
// delivers to the program without ever blocking its caller and is harmless
// before the program is wired.
func (r *ProgramRef) InvocationStarted(inv domain.ActivityInvocation) domain.ActivityFeed {
	f := &programFeed{ref: r, key: r.act.newKey()}
	f.send(activityStartedMsg{key: f.key, inv: inv})
	return f
}

// programFeed is the activity channel of one invocation of a ProgramRef.
type programFeed struct {
	ref *ProgramRef
	key runflow.ActivityKey

	mu    sync.Mutex
	ended bool
}

func (f *programFeed) send(msg tea.Msg) {
	if p := f.ref.program(); p != nil {
		f.ref.act.post(p, msg)
	}
}

// Activity implements domain.ActivityFeed.
func (f *programFeed) Activity(item domain.ActivityItem) {
	f.mu.Lock()
	ended := f.ended
	f.mu.Unlock()
	if !ended {
		f.send(activityItemMsg{key: f.key, item: item})
	}
}

// Status implements domain.ActivityFeed. The status is stamped with its
// receipt time, which the screen shows as the last stream event.
func (f *programFeed) Status(st domain.ActivityStatus) {
	f.mu.Lock()
	ended := f.ended
	f.mu.Unlock()
	if !ended {
		f.send(activityStatusMsg{key: f.key, status: st, at: time.Now()})
	}
}

// Ended implements domain.ActivityFeed.
func (f *programFeed) Ended() {
	f.mu.Lock()
	already := f.ended
	f.ended = true
	f.mu.Unlock()
	if !already {
		f.send(activityEndedMsg{key: f.key})
	}
}

// handleActivityMsg applies an activity message to the progress screen and
// reports whether msg was one. Messages are ignored without a progress screen.
func (m *rootModel) handleActivityMsg(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case activityStartedMsg:
		if m.progressScreen != nil {
			m.progressScreen.BeginActivity(msg.key, msg.inv)
		}
	case activityItemMsg:
		if m.progressScreen != nil {
			m.progressScreen.AppendActivity(msg.key, msg.item)
		}
	case activityStatusMsg:
		if m.progressScreen != nil {
			m.progressScreen.UpdateActivityStatus(msg.key, msg.status, msg.at)
		}
	case activityEndedMsg:
		if m.progressScreen != nil {
			m.progressScreen.EndActivity(msg.key)
		}
	default:
		return false
	}
	return true
}
