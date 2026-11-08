package harness

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// observerDrainGrace bounds how long Run waits, after the process exited, for
// an observer to receive the lines still queued. A slow observer is cut off
// after this so it can never delay Run's result.
const observerDrainGrace = 2 * time.Second

// lineObserver splits the stdout stream into lines and hands them, in order
// and one at a time, to a callback on its own goroutine. Write never blocks
// on the callback: lines wait in an unbounded queue, so the subprocess pipe
// is always drained at full speed.
type lineObserver struct {
	fn func(line []byte)

	mu      sync.Mutex
	partial []byte
	queue   [][]byte
	closed  bool // no more input will arrive
	stopped bool // delivery abandoned; queued lines are dropped

	wake chan struct{}
	done chan struct{}
}

// startLineObserver returns an observer already delivering to fn.
func startLineObserver(fn func(line []byte)) *lineObserver {
	o := &lineObserver{
		fn:   fn,
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	go o.deliver()
	return o
}

// teeStdout returns the writer that captures stdout into buf and, when obs is
// non-nil, also feeds it to the observer.
func teeStdout(buf *bytes.Buffer, obs *lineObserver) io.Writer {
	if obs == nil {
		return buf
	}
	return io.MultiWriter(buf, obs)
}

// Write implements io.Writer. It always accepts all bytes.
func (o *lineObserver) Write(p []byte) (int, error) {
	o.mu.Lock()
	if !o.stopped {
		o.partial = append(o.partial, p...)
		for {
			i := bytes.IndexByte(o.partial, '\n')
			if i < 0 {
				break
			}
			o.enqueueLocked(o.partial[:i])
			o.partial = o.partial[i+1:]
		}
	}
	o.mu.Unlock()
	o.signal()
	return len(p), nil
}

// enqueueLocked queues a copy of line with its terminator stripped; empty
// lines are skipped. The caller holds o.mu.
func (o *lineObserver) enqueueLocked(line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if len(line) == 0 {
		return
	}
	o.queue = append(o.queue, append([]byte(nil), line...))
}

func (o *lineObserver) signal() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// finish flushes an unterminated last line and waits up to grace for the
// queue to drain, then abandons whatever is left.
func (o *lineObserver) finish(grace time.Duration) {
	o.mu.Lock()
	if !o.stopped && len(o.partial) > 0 {
		o.enqueueLocked(o.partial)
		o.partial = nil
	}
	o.closed = true
	o.mu.Unlock()
	o.signal()

	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-o.done:
	case <-timer.C:
	}
	o.stop()
}

// stop abandons delivery: no further callback starts after it returns.
func (o *lineObserver) stop() {
	o.mu.Lock()
	o.stopped = true
	o.queue = nil
	o.partial = nil
	o.mu.Unlock()
	o.signal()
}

// deliver runs the callback for each queued line until the input is closed
// and drained, delivery is stopped, or the callback panics (after which the
// observer receives nothing further).
func (o *lineObserver) deliver() {
	defer close(o.done)
	for {
		o.mu.Lock()
		if o.stopped {
			o.mu.Unlock()
			return
		}
		if len(o.queue) == 0 {
			closed := o.closed
			o.mu.Unlock()
			if closed {
				return
			}
			<-o.wake
			continue
		}
		line := o.queue[0]
		o.queue = o.queue[1:]
		o.mu.Unlock()

		if !o.call(line) {
			o.stop()
			return
		}
	}
}

// call invokes the callback, reporting false if it panicked.
func (o *lineObserver) call(line []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	o.fn(line)
	return true
}
