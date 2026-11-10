package tui

// ProgramRef as the activity observer: the feeds it hands out reach the
// progress screen through the running program, in order, tied to their own
// invocation, without ever blocking the adapter that reports them.
//
// The program runs a probe model wrapping a root model on the progress screen;
// after every message the probe stores the progress screen's rendering, so the
// tests observe what the user would see without touching the model from
// another goroutine.

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mosaic-run/internal/domain"
)

// Compile-time proof that the adapters can be handed a ProgramRef.
var _ domain.ActivityObserver = (*ProgramRef)(nil)

var activityInv = domain.ActivityInvocation{
	Source: domain.ActivitySourceDispatch, Agent: "agent-a#3", Harness: "claude-code",
}

// activityProbe is a tea.Model wrapping a root model and recording the
// rendering of its progress screen after every message. When gate is non-nil,
// every message waits until the gate is closed.
type activityProbe struct {
	inner *rootModel
	gate  chan struct{}

	mu     sync.Mutex
	latest string
}

func (p *activityProbe) Init() tea.Cmd { return nil }

func (p *activityProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if p.gate != nil {
		<-p.gate
	}
	p.inner.Update(msg)
	view := ""
	if p.inner.progressScreen != nil {
		view = p.inner.progressScreen.View()
	}
	p.mu.Lock()
	p.latest = view
	p.mu.Unlock()
	return p, nil
}

func (p *activityProbe) View() string { return "" }

func (p *activityProbe) snapshot() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.latest
}

// startActivityProgram runs a probe around a progress-screen root model and
// returns a ProgramRef wired to it.
func startActivityProgram(t *testing.T, gate chan struct{}) (*ProgramRef, *activityProbe) {
	t.Helper()
	probe := &activityProbe{inner: newParityModel(), gate: gate}
	prog, done := runHeadless(probe)
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
	return ref, probe
}

// waitView polls the probe until pred accepts its rendering, or fails after 3s.
func waitView(t *testing.T, probe *activityProbe, what string, pred func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if v := probe.snapshot(); pred(v) {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("progress screen never showed %s; last rendering:\n%s", what, probe.snapshot())
	return ""
}

func contains(sub string) func(string) bool {
	return func(v string) bool { return strings.Contains(v, sub) }
}

func TestProgramRef_InvocationStarted_BeforeTheProgramIsWired_ReturnsAUsableFeed(t *testing.T) {
	ref := NewProgramRef()

	feed := ref.InvocationStarted(activityInv)
	if feed == nil {
		t.Fatal("InvocationStarted returned a nil feed")
	}
	feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "x"})
	feed.Ended()
}

func TestProgramRef_Activity_ReachesTheProgressScreen(t *testing.T) {
	ref, probe := startActivityProgram(t, nil)

	feed := ref.InvocationStarted(activityInv)
	waitView(t, probe, "the placeholder of the started invocation", contains("no live output available"))
	feed.Activity(domain.ActivityItem{Kind: domain.ActivityToolCall, Name: "Read", Summary: "notes.txt"})
	v := waitView(t, probe, "the tool call", contains("notes.txt"))

	if strings.Contains(v, "no live output available") {
		t.Errorf("placeholder still shown with an item present:\n%s", v)
	}
	feed.Ended()
	waitView(t, probe, "the ended mark", func(v string) bool { return strings.Contains(strings.ToLower(v), "ended") })
}

func TestProgramRef_Activity_KeepsTheCallOrder(t *testing.T) {
	ref, probe := startActivityProgram(t, nil)
	feed := ref.InvocationStarted(activityInv)

	for i := 1; i <= 5; i++ {
		feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: fmt.Sprintf("step-%d", i)})
	}
	v := waitView(t, probe, "the last item", contains("step-5"))

	last := -1
	for i := 1; i <= 5; i++ {
		pos := strings.Index(v, fmt.Sprintf("step-%d", i))
		if pos < 0 || pos < last {
			t.Fatalf("item %d missing or out of order in:\n%s", i, v)
		}
		last = pos
	}
}

func TestProgramRef_NewInvocation_ReplacesTheEarlierOnesActivity(t *testing.T) {
	ref, probe := startActivityProgram(t, nil)
	first := ref.InvocationStarted(activityInv)
	first.Activity(domain.ActivityItem{Kind: domain.ActivityToolCall, Name: "FirstTool", Summary: "one"})
	waitView(t, probe, "the first invocation's item", contains("FirstTool"))
	first.Ended()

	second := ref.InvocationStarted(domain.ActivityInvocation{Source: domain.ActivitySourceDispatch, Agent: "agent-b#4"})
	v := waitView(t, probe, "the second invocation's placeholder", contains("no live output available"))

	if strings.Contains(v, "FirstTool") {
		t.Errorf("earlier invocation's activity shown for the new one:\n%s", v)
	}
	second.Ended()
}

func TestProgramRef_LateItemOfASupersededInvocation_IsNotShownAsTheCurrentOnes(t *testing.T) {
	ref, probe := startActivityProgram(t, nil)
	first := ref.InvocationStarted(activityInv)
	second := ref.InvocationStarted(domain.ActivityInvocation{Source: domain.ActivitySourceConsultation, Agent: "orch"})
	waitView(t, probe, "the second invocation's placeholder", contains("orch"))

	first.Activity(domain.ActivityItem{Kind: domain.ActivityToolCall, Name: "StaleTool", Summary: "late"})
	second.Activity(domain.ActivityItem{Kind: domain.ActivityToolCall, Name: "FreshTool", Summary: "now"})
	waitView(t, probe, "the current invocation's item", contains("FreshTool"))
	time.Sleep(100 * time.Millisecond)

	if v := probe.snapshot(); strings.Contains(v, "StaleTool") {
		t.Errorf("a superseded invocation's item was shown:\n%s", v)
	}
}

func TestProgramRef_Activity_WithoutAProgressScreen_IsHarmless(t *testing.T) {
	probe := &activityProbe{inner: newFlowModel(domain.RunOutcome{Status: domain.RunCompleted})}
	prog, done := runHeadless(probe)
	defer func() { prog.Quit(); <-done }()
	ref := NewProgramRef()
	ref.set(prog)

	feed := ref.InvocationStarted(activityInv)
	feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: "nobody listens"})
	feed.Ended()
	time.Sleep(50 * time.Millisecond)
}

func TestProgramRef_Activity_WhileTheProgramIsStalled_NeverBlocksTheCaller(t *testing.T) {
	gate := make(chan struct{})
	ref, probe := startActivityProgram(t, gate)
	returned := make(chan struct{})

	go func() {
		defer close(returned)
		feed := ref.InvocationStarted(activityInv)
		for i := 0; i < 5000; i++ {
			feed.Activity(domain.ActivityItem{Kind: domain.ActivityText, Summary: fmt.Sprintf("flood-%d", i)})
		}
		feed.Ended()
	}()

	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("reporting activity blocked while the program was not consuming messages")
	}
	close(gate)
	v := waitView(t, probe, "the end of the invocation after the program resumed",
		func(v string) bool { return strings.Contains(strings.ToLower(v), "ended") })
	if !strings.Contains(v, "agent-a#3") {
		t.Errorf("the start of the invocation was lost under backlog:\n%s", v)
	}
}
