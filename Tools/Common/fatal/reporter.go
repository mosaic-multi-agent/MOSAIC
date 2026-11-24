package fatal

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"sync"
)

// Reporter is the process reporter created by Install.
type Reporter struct {
	cfg Config

	claimMu sync.Mutex
	claimed bool
	done    chan struct{} // closed when the claimed report has completed

	stateMu  sync.Mutex
	recorded *panicRecord
	programs []programHandle

	capture *stderrCapture
}

type panicRecord struct {
	value any
	stack []byte
}

// request describes one terminating call.
type request struct {
	code        int
	report      bool
	message     string // Fail: message to print and record
	fromCapture bool   // ExitCLI(1): message is the captured stderr text
	panicRec    *panicRecord
}

func newReporter(cfg Config) *Reporter {
	r := &Reporter{cfg: cfg, done: make(chan struct{})}
	r.capture = &stderrCapture{out: cfg.Stderr}
	return r
}

// Fail reports an unexpected failure and exits with code 1. message is printed to
// stderr followed by a newline (not doubled if it already ends with one).
func (r *Reporter) Fail(message string) {
	r.run(request{code: 1, report: true, message: message})
}

// Exit ends the process with code and reports nothing.
func (r *Reporter) Exit(code int) {
	r.run(request{code: code})
}

// ExitCLI ends the process with the result code of a CLI frontend. Code 1 is an
// unexpected failure reported with the text written through Stderr(); any other
// code behaves as Exit.
func (r *Reporter) ExitCLI(code int) {
	if code == 1 {
		r.run(request{code: 1, report: true, fromCapture: true})
		return
	}
	r.run(request{code: code})
}

// Stderr returns the writer the tool uses for its own stderr output in CLI mode.
func (r *Reporter) Stderr() io.Writer { return r.capture }

// run executes one terminating call: a recorded TUI panic replaces the request,
// the first report claims the process, every later call waits and exits 1.
func (r *Reporter) run(req request) {
	if rec := r.recordedPanic(); rec != nil {
		req = request{code: 1, report: true, panicRec: rec}
	}
	if !r.claim(req.report) {
		<-r.done
		r.cfg.Exit(1)
		return
	}
	if !req.report {
		r.cfg.Exit(req.code)
		return
	}
	r.report(req)
	close(r.done)
	r.cfg.Exit(1)
}

// claim returns false when another report already owns the process. When take is
// set a successful claim makes the caller the reporter.
func (r *Reporter) claim(take bool) bool {
	r.claimMu.Lock()
	defer r.claimMu.Unlock()
	if r.claimed {
		return false
	}
	if take {
		r.claimed = true
	}
	return true
}

// Recover must be deferred directly: defer fatal.Recover(). With no reporter
// installed the panic continues untouched; otherwise it is reported through the
// installed reporter and the process exits with code 1.
func Recover() {
	r := currentReporter()
	if r == nil {
		return
	}
	v := recover()
	if v == nil {
		return
	}
	r.handlePanic(&panicRecord{value: v, stack: debug.Stack()})
}

func (r *Reporter) handlePanic(rec *panicRecord) {
	if !r.claim(true) {
		<-r.done
		r.cfg.Exit(1)
		return
	}
	r.stopPrograms()
	r.report(request{code: 1, report: true, panicRec: rec})
	close(r.done)
	r.cfg.Exit(1)
}

func (r *Reporter) recordedPanic() *panicRecord {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.recorded
}

// recordPanic keeps the first guarded panic only.
func (r *Reporter) recordPanic(v any, stack []byte) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.recorded == nil {
		r.recorded = &panicRecord{value: v, stack: stack}
	}
}

// report prints the screen report, writes the failure file, prints the path line
// and pauses on a Windows TUI start.
func (r *Reporter) report(req request) {
	kind, message, stack := "failure", req.message, ""
	switch {
	case req.panicRec != nil:
		kind = "panic"
		message = fmt.Sprintf("panic: %v", req.panicRec.value)
		stack = string(req.panicRec.stack)
		r.print(message + "\n\n" + stack)
		if !strings.HasSuffix(stack, "\n") {
			r.print("\n")
		}
	case req.fromCapture:
		message = r.capture.text()
	default:
		r.print(message)
		if !strings.HasSuffix(message, "\n") {
			r.print("\n")
		}
	}
	if path, ok := r.writeFailureFile(kind, message, stack); ok {
		r.print("failure report written to " + path + "\n")
	}
	if r.cfg.Mode == ModeTUI && r.cfg.GOOS == "windows" {
		r.pause()
	}
}

func (r *Reporter) print(s string) {
	_, _ = io.WriteString(r.cfg.Stderr, s)
}
