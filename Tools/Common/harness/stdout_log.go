package harness

import (
	"bytes"
	"io"
	"strconv"
	"sync"

	"mosaic-common/fatal"
)

// stdoutLogger reports stdout lines to a Sink as they arrive. Write never
// blocks on the sink: lines wait in an unbounded queue and a dedicated
// goroutine passes them on, so a slow sink cannot stall the pipe or the
// no-output watch. Unlike lineObserver it never drops a line: close flushes
// the unterminated last line and waits until everything reached the sink.
//
// All sink calls for the invocation made through the logger happen on its
// one goroutine, so the sink is never called concurrently with itself.
type stdoutLogger struct {
	sink Sink

	mu      sync.Mutex
	partial []byte
	queue   []string
	total   int  // every stdout byte received
	closed  bool // no more input will arrive

	next int // ordinal of the next logged line (goroutine-owned)

	wake chan struct{}
	done chan struct{}
}

// startStdoutLogger returns a logger already delivering to sink, or nil when
// there is no sink (all methods are nil-safe).
func startStdoutLogger(sink Sink) *stdoutLogger {
	if sink == nil {
		return nil
	}
	l := &stdoutLogger{
		sink: sink,
		next: 1,
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	go func() {
		defer fatal.Recover()
		l.deliver()
	}()
	return l
}

// withStdoutLogger adds the logger to the stdout writer chain.
func withStdoutLogger(w io.Writer, l *stdoutLogger) io.Writer {
	if l == nil {
		return w
	}
	return io.MultiWriter(w, l)
}

// Write implements io.Writer. It always accepts all bytes.
func (l *stdoutLogger) Write(p []byte) (int, error) {
	l.mu.Lock()
	l.total += len(p)
	l.partial = append(l.partial, p...)
	for {
		i := bytes.IndexByte(l.partial, '\n')
		if i < 0 {
			break
		}
		l.enqueueLocked(l.partial[:i])
		l.partial = l.partial[i+1:]
	}
	l.mu.Unlock()
	l.signal()
	return len(p), nil
}

// enqueueLocked queues line with its terminator stripped; empty lines are
// skipped. The caller holds l.mu.
func (l *stdoutLogger) enqueueLocked(line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if len(line) == 0 {
		return
	}
	l.queue = append(l.queue, string(line))
}

func (l *stdoutLogger) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// close flushes the unterminated last line, waits until every queued line
// reached the sink, and returns the number of stdout bytes received.
func (l *stdoutLogger) close() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	if len(l.partial) > 0 {
		l.enqueueLocked(l.partial)
		l.partial = nil
	}
	l.closed = true
	total := l.total
	l.mu.Unlock()
	l.signal()
	<-l.done
	return total
}

// deliver passes queued lines to the sink, one line per event so the event
// sequence for a given output does not depend on timing, until the input is
// closed and the queue is empty.
func (l *stdoutLogger) deliver() {
	defer close(l.done)
	for {
		l.mu.Lock()
		var line string
		have := len(l.queue) > 0
		if have {
			line = l.queue[0]
			l.queue = l.queue[1:]
		}
		closed := l.closed
		l.mu.Unlock()

		if have {
			logEvent(l.sink, "spawn.stdout.lines", line, []Field{
				{Key: "first", Value: strconv.Itoa(l.next)},
				{Key: "lines", Value: "1"},
			})
			l.next++
			continue
		}
		if closed {
			return
		}
		<-l.wake
	}
}

// logEnd logs the invocation's closing events on every end path: the
// captured stderr (the normal-exit path logs stderr itself, after its own
// events) and the final spawn.end with the stdout byte count.
func logEnd(sink Sink, reason string, stdoutBytes int, exitCode *int) {
	fields := []Field{
		{Key: "reason", Value: reason},
		{Key: "stdout_bytes", Value: strconv.Itoa(stdoutBytes)},
	}
	if exitCode != nil {
		fields = append(fields, Field{Key: "exit_code", Value: strconv.Itoa(*exitCode)})
	}
	logEvent(sink, "spawn.end", "process ended: "+reason, fields)
}

// logStderr logs the captured stderr as a spawn.stderr event.
func logStderr(sink Sink, stderr *bytes.Buffer) {
	logEvent(sink, "spawn.stderr", stderr.String(), []Field{{Key: "bytes", Value: strconv.Itoa(stderr.Len())}})
}
