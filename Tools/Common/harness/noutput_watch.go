package harness

import (
	"io"
	"sync"
	"time"
)

// outputWatch is Run's opt-in no-output watchdog. It records when stdout data
// last arrived and signals on fired once a full duration passes without any.
// Only stdout data counts; stderr never resets the silence.
type outputWatch struct {
	after time.Duration
	fired chan struct{}
	quit  chan struct{}

	mu   sync.Mutex
	last time.Time
	once sync.Once
}

// startOutputWatch returns a watch measuring silence from now, or nil when
// after is zero or negative (watchdog disabled).
func startOutputWatch(after time.Duration) *outputWatch {
	if after <= 0 {
		return nil
	}
	w := &outputWatch{
		after: after,
		fired: make(chan struct{}),
		quit:  make(chan struct{}),
		last:  time.Now(),
	}
	go w.run()
	return w
}

// Write implements io.Writer: any stdout data resets the silence.
func (w *outputWatch) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.mu.Lock()
		w.last = time.Now()
		w.mu.Unlock()
	}
	return len(p), nil
}

func (w *outputWatch) run() {
	timer := time.NewTimer(w.after)
	defer timer.Stop()
	for {
		select {
		case <-w.quit:
			return
		case <-timer.C:
			w.mu.Lock()
			remaining := w.after - time.Since(w.last)
			w.mu.Unlock()
			if remaining <= 0 {
				close(w.fired)
				return
			}
			timer.Reset(remaining)
		}
	}
}

// firedCh returns the channel closed when the silence limit is reached. A nil
// watch returns a nil channel, which never becomes ready in a select.
func (w *outputWatch) firedCh() <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.fired
}

// stop ends the watch goroutine. Safe on a nil watch and to call repeatedly.
func (w *outputWatch) stop() {
	if w == nil {
		return
	}
	w.once.Do(func() { close(w.quit) })
}

// err is the error Run returns when the watch fired. It satisfies both
// ErrNoOutput and ErrTimeout.
func (w *outputWatch) err() error {
	return &noOutputError{after: w.after}
}

// noOutputError carries the contract message while unwrapping to both
// sentinels.
type noOutputError struct {
	after time.Duration
}

func (e *noOutputError) Error() string {
	return ErrNoOutput.Error() + ": no stdout output for " + e.after.String()
}

func (e *noOutputError) Unwrap() []error {
	return []error{ErrNoOutput, ErrTimeout}
}

// message is the diagnostic event text for a fired watch.
func (w *outputWatch) message() string {
	return "no stdout output for " + w.after.String()
}

// teeWatch adds the watch to the stdout writer; a nil watch returns out as is.
func teeWatch(out io.Writer, w *outputWatch) io.Writer {
	if w == nil {
		return out
	}
	return io.MultiWriter(out, w)
}
