package fatal

import (
	"bytes"
	"io"
	"sync"
)

const captureLimit = 64 * 1024

const truncationMarker = "[earlier output truncated]\n"

// stderrCapture passes writes through unchanged and keeps the last captureLimit
// bytes of everything written.
type stderrCapture struct {
	out io.Writer

	mu        sync.Mutex
	tail      []byte
	truncated bool
}

func (c *stderrCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tail = append(c.tail, p...)
	if len(c.tail) > captureLimit {
		c.tail = append([]byte(nil), c.tail[len(c.tail)-captureLimit:]...)
		c.truncated = true
	}
	return c.out.Write(p)
}

// text returns the captured output as the failure message.
func (c *stderrCapture) text() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.tail) == 0 {
		return "(no error output)"
	}
	tail := c.tail
	if c.truncated {
		if i := bytes.IndexByte(tail, '\n'); i >= 0 && i+1 < len(tail) {
			tail = tail[i+1:]
		}
		return truncationMarker + string(tail)
	}
	return string(tail)
}
