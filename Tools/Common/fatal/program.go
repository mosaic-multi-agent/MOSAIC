package fatal

import (
	"runtime/debug"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const stopWait = 5 * time.Second

// programHandle is a program built by NewProgram that has started running.
type programHandle struct{ p *tea.Program }

type guardLink struct{ prog *tea.Program }

// guardModel runs the tool's model and records panics of Init, Update, View and
// of the commands they return.
type guardModel struct {
	inner tea.Model
	r     *Reporter
	link  *guardLink
}

// NewProgram is a drop-in for tea.NewProgram. The program runs a guarded copy of m
// and is tracked by r while it runs.
func (r *Reporter) NewProgram(m tea.Model, opts ...tea.ProgramOption) *tea.Program {
	link := &guardLink{}
	link.prog = tea.NewProgram(guardModel{inner: m, r: r, link: link}, opts...)
	return link.prog
}

// UnwrapModel returns the tool's own model from a final model returned by Run of a
// program built by NewProgram; any other model is returned unchanged.
func UnwrapModel(m tea.Model) tea.Model {
	if g, ok := m.(guardModel); ok {
		return g.inner
	}
	return m
}

func (g guardModel) Init() (cmd tea.Cmd) {
	g.r.track(g.link.prog)
	if g.r.recordedPanic() != nil {
		return nil
	}
	defer g.r.guardRecover(g.link.prog)
	return g.r.wrapCmd(g.inner.Init(), g.link.prog)
}

func (g guardModel) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	model = g
	if g.r.recordedPanic() != nil {
		return model, nil
	}
	defer g.r.guardRecover(g.link.prog)
	next, c := g.inner.Update(msg)
	g.inner = next
	return g, g.r.wrapCmd(c, g.link.prog)
}

func (g guardModel) View() (view string) {
	if g.r.recordedPanic() != nil {
		return ""
	}
	defer g.r.guardRecover(g.link.prog)
	return g.inner.View()
}

// wrapCmd guards cmd, and recursively the members of a batch it returns.
func (r *Reporter) wrapCmd(cmd tea.Cmd, p *tea.Program) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() (msg tea.Msg) {
		defer r.guardRecover(p)
		msg = cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			wrapped := make(tea.BatchMsg, len(batch))
			for i, c := range batch {
				wrapped[i] = r.wrapCmd(c, p)
			}
			return wrapped
		}
		return msg
	}
}

// guardRecover must be deferred directly. It records a panic and kills the
// program so Run returns with the terminal restored.
func (r *Reporter) guardRecover(p *tea.Program) {
	v := recover()
	if v == nil {
		return
	}
	r.recordPanic(v, debug.Stack())
	p.Kill()
}

func (r *Reporter) track(p *tea.Program) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.programs = append(r.programs, programHandle{p: p})
}

// stopPrograms kills every running program and waits for each Run to return, so
// the terminal is restored before anything is printed.
func (r *Reporter) stopPrograms() {
	r.stateMu.Lock()
	programs := append([]programHandle(nil), r.programs...)
	r.stateMu.Unlock()
	for _, h := range programs {
		h.p.Kill()
		finished := make(chan struct{})
		go func() {
			h.p.Wait()
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(stopWait):
		}
	}
}
