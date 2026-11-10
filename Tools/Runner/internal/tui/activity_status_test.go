package tui

// Activity statuses through the program feed: they reach the program for
// dispatch and consultation invocations, and they are not lost, reordered or
// overtaken by the invocation's end when the pending-item queue drops old
// items under backlog.
//
// The program runs a recorder that keeps every message it receives. Status
// messages are recognised by their printed content (the status values the
// tests send), so the tests do not depend on how the transport names or
// shapes them. The printed form (%+v) is used, so the transport message must
// hold the status by value (a pointer would print an address).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// msgRecorder is a tea.Model keeping every message it receives. When gate is
// non-nil, every message waits until the gate is closed.
type msgRecorder struct {
	gate chan struct{}

	mu   sync.Mutex
	msgs []tea.Msg
}

func (r *msgRecorder) Init() tea.Cmd { return nil }

func (r *msgRecorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if r.gate != nil {
		<-r.gate
	}
	r.mu.Lock()
	r.msgs = append(r.msgs, msg)
	r.mu.Unlock()
	return r, nil
}

func (r *msgRecorder) View() string { return "" }

func (r *msgRecorder) snapshot() []tea.Msg {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]tea.Msg(nil), r.msgs...)
}

func startRecorderProgram(t *testing.T, gate chan struct{}) (*ProgramRef, *msgRecorder) {
	t.Helper()
	rec := &msgRecorder{gate: gate}
	prog, done := runHeadless(rec)
	t.Cleanup(func() {
		if gate != nil {
			select {
			case <-gate:
			default:
				close(gate)
			}
		}
		prog.Quit()
		<-done
	})
	ref := NewProgramRef()
	ref.set(prog)
	return ref, rec
}

var contextTokensRe = regexp.MustCompile(`ContextTokens:(\d+)`)

// statusMsgs returns the printed form of every recorded message that carries
// one of the statuses the tests send, together with its position.
func statusMsgs(msgs []tea.Msg) (printed []string, positions []int) {
	for i, m := range msgs {
		switch m.(type) {
		case activityStartedMsg, activityItemMsg, activityEndedMsg:
			continue
		}
		s := fmt.Sprintf("%+v", m)
		if strings.Contains(s, "status-model") {
			printed = append(printed, s)
			positions = append(positions, i)
		}
	}
	return printed, positions
}

func waitRecorded(t *testing.T, rec *msgRecorder, what string, pred func([]tea.Msg) bool) []tea.Msg {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m := rec.snapshot(); pred(m) {
			return m
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("program never received %s; got %d messages", what, len(rec.snapshot()))
	return nil
}

func hasEnded(msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(activityEndedMsg); ok {
			return true
		}
	}
	return false
}

func TestProgramRef_Status_ReachesTheProgram_ForDispatchAndConsultationInvocations(t *testing.T) {
	for _, src := range []domain.ActivitySource{domain.ActivitySourceDispatch, domain.ActivitySourceConsultation} {
		t.Run(string(src), func(t *testing.T) {
			ref, rec := startRecorderProgram(t, nil)
			feed := ref.InvocationStarted(domain.ActivityInvocation{Source: src, Agent: "agent-a#3", Harness: "claude-code"})

			feed.Status(domain.ActivityStatus{Model: "status-model-x", ContextTokens: 4242, HasContext: true,
				RunningTool: "Read", RunningToolID: "tool-1"})

			msgs := waitRecorded(t, rec, "the status", func(m []tea.Msg) bool {
				p, _ := statusMsgs(m)
				return len(p) > 0
			})
			p, _ := statusMsgs(msgs)
			for _, want := range []string{"status-model-x", "4242", "Read", "tool-1"} {
				if !strings.Contains(p[0], want) {
					t.Errorf("status message %q lacks %q", p[0], want)
				}
			}
			feed.Ended()
		})
	}
}

func TestProgramRef_Status_AfterEnded_IsNotDelivered(t *testing.T) {
	ref, rec := startRecorderProgram(t, nil)
	feed := ref.InvocationStarted(activityInv)
	feed.Ended()
	waitRecorded(t, rec, "the ended message", hasEnded)

	feed.Status(domain.ActivityStatus{Model: "status-model-late"})
	time.Sleep(100 * time.Millisecond)

	if p, _ := statusMsgs(rec.snapshot()); len(p) != 0 {
		t.Errorf("a status after Ended must be ignored, got %v", p)
	}
}

func TestProgramRef_Status_BeforeTheProgramIsWired_IsHarmless(t *testing.T) {
	feed := NewProgramRef().InvocationStarted(activityInv)

	feed.Status(domain.ActivityStatus{Model: "status-model-x"})
	feed.Ended()
}

func TestProgramRef_Status_UnderBacklog_NewestSurvivesInOrderBeforeTheEnd(t *testing.T) {
	gate := make(chan struct{})
	ref, rec := startRecorderProgram(t, gate)
	const lines = 3 * maxPendingActivityItems
	returned := make(chan struct{})

	go func() {
		defer close(returned)
		feed := ref.InvocationStarted(activityInv)
		for i := 1; i <= lines; i++ {
			feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: fmt.Sprintf("flood-%d", i)})
			feed.Status(domain.ActivityStatus{Model: "status-model-x", ContextTokens: i, HasContext: true})
		}
		feed.Ended()
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("reporting statuses blocked while the program was not consuming messages")
	}
	close(gate)
	msgs := waitRecorded(t, rec, "the ended message", hasEnded)

	printed, positions := statusMsgs(msgs)
	if len(printed) == 0 {
		t.Fatal("every status was lost under backlog")
	}
	last := -1
	for _, s := range printed {
		m := contextTokensRe.FindStringSubmatch(s)
		if m == nil {
			t.Fatalf("status message without context tokens: %q", s)
		}
		n, _ := strconv.Atoi(m[1])
		if n <= last {
			t.Errorf("statuses delivered out of order: %d after %d", n, last)
		}
		last = n
	}
	if last != lines {
		t.Errorf("the newest status (%d) must survive the queue bound, newest delivered was %d", lines, last)
	}
	endedAt := -1
	for i, m := range msgs {
		if _, ok := m.(activityEndedMsg); ok {
			endedAt = i
		}
	}
	if positions[len(positions)-1] > endedAt {
		t.Errorf("the newest status must be delivered before the ended message")
	}
}

func TestProgramRef_Status_SurvivesDroppingForEachInvocationOfAConsultationAndADispatch(t *testing.T) {
	gate := make(chan struct{})
	ref, rec := startRecorderProgram(t, gate)

	dispatch := ref.InvocationStarted(activityInv)
	consult := ref.InvocationStarted(domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orch"})
	for i := 0; i < 2*maxPendingActivityItems; i++ {
		dispatch.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "d"})
		consult.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "c"})
	}
	dispatch.Status(domain.ActivityStatus{Model: "status-model-dispatch", ContextTokens: 111, HasContext: true})
	consult.Status(domain.ActivityStatus{Model: "status-model-consult", ContextTokens: 222, HasContext: true})
	dispatch.Ended()
	consult.Ended()
	close(gate)
	msgs := waitRecorded(t, rec, "both ended messages", func(m []tea.Msg) bool {
		n := 0
		for _, x := range m {
			if _, ok := x.(activityEndedMsg); ok {
				n++
			}
		}
		return n == 2
	})

	printed, _ := statusMsgs(msgs)
	joined := strings.Join(printed, "\n")
	for _, want := range []string{"status-model-dispatch", "status-model-consult"} {
		if !strings.Contains(joined, want) {
			t.Errorf("status %q was lost under backlog; delivered: %v", want, printed)
		}
	}
}
