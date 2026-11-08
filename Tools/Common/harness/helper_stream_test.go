package harness_test

// Streaming modes of the fake-subprocess helper (see helper_test.go). They
// produce stdout in separate writes so tests can observe it while the process
// is still running.
//
// Synchronisation uses a gate file rather than sleeps: a gated mode writes its
// first line, then waits until the file named by GO_HELPER_GATE_FILE exists
// before writing the rest. A test creates the file from inside its stdout
// observer, so the remainder of the output can only appear after the first line
// was delivered to the observer while the process was still running. The wait
// is capped so a missing delivery fails the test instead of hanging it.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const helperGateWait = 5 * time.Second

// waitGate blocks until the gate file exists. It returns true immediately when
// no gate is configured, and false when the wait cap elapsed first.
func waitGate() bool {
	path := os.Getenv("GO_HELPER_GATE_FILE")
	if path == "" {
		return true
	}
	deadline := time.Now().Add(helperGateWait)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func writeOut(s string) { os.Stdout.WriteString(s) } //nolint:errcheck

// writeGated writes first, waits for the gate, then writes rest. When the gate
// never opens it writes a marker line instead of rest.
func writeGated(first, rest string) {
	writeOut(first)
	if !waitGate() {
		writeOut("gate-timeout\n")
		return
	}
	writeOut(rest)
}

// runStreamHelper handles the streaming helper modes. It returns normally for
// a known mode that exits zero, and exits the process itself otherwise.
func runStreamHelper(cmd string) {
	protocolResp := `{"agent_instance_id":"test-agent#1","status_code":"SUCCESS","status_message":"ok"}`
	switch cmd {
	case "stream-gated":
		writeGated("first\n", "second\n")

	case "stream-partial":
		writeOut("alpha\nbeta")

	case "stream-crlf":
		writeOut("a\r\n\r\n\nb\r\n")

	case "stream-long":
		n, _ := strconv.Atoi(os.Getenv("GO_HELPER_LONG_BYTES"))
		chunk := strings.Repeat("x", 64*1024)
		for n > 0 {
			size := len(chunk)
			if n < size {
				size = n
			}
			writeOut(chunk[:size])
			n -= size
		}
		writeOut("\ntail\n")

	case "stream-many":
		count, _ := strconv.Atoi(os.Getenv("GO_HELPER_LINE_COUNT"))
		pad, _ := strconv.Atoi(os.Getenv("GO_HELPER_LINE_PAD"))
		padding := strings.Repeat("p", pad)
		for i := 0; i < count; i++ {
			writeOut("line-" + strconv.Itoa(i) + padding + "\n")
		}

	case "stream-three":
		writeOut("l1\nl2\nl3\n")
		os.Stderr.WriteString("warn\n") //nolint:errcheck

	case "stream-exit1":
		writeOut("partial\n")
		os.Stderr.WriteString("boom") //nolint:errcheck
		os.Exit(1)

	case "stream-then-hang":
		writeOut("first\n")
		time.Sleep(24 * time.Hour)

	case "claude-stream-gated":
		// A pretty-printed JSON array envelope: valid as a whole, but its
		// first line ("[") is complete long before the result line.
		resultLine := `{"type":"result","result":` + jsonQuote(protocolResp) + `}` + "\n"
		writeGated("[\n", resultLine+"]\n")

	case "opencode-stream-gated":
		text := `{"type":"text","part":{"type":"text","text":` + jsonQuote(protocolResp) + `}}` + "\n"
		finish := `{"type":"step_finish","part":{"reason":"stop"}}` + "\n"
		writeGated(`{"type":"step_start"}`+"\n", text+finish)

	case "ghcpcli-stream-gated":
		msg := `{"type":"assistant.message","data":{"content":` + jsonQuote(protocolResp) + `},"id":"id1","timestamp":"2026-08-12T21:00:00Z","parentId":"pid1"}` + "\n"
		result := `{"type":"result","timestamp":"2026-08-12T21:00:01Z","sessionId":"s1","exitCode":0,"usage":{}}` + "\n"
		writeGated(`{"type":"session.mcp_servers_loaded","data":{},"id":"id0","timestamp":"2026-08-12T21:00:00Z","parentId":"pid1"}`+"\n", msg+result)

	default:
		os.Stderr.WriteString("unknown GO_HELPER_CMD: " + cmd) //nolint:errcheck
		os.Exit(2)
	}
}

// newGate configures the gate file for helper processes spawned by the current
// test and returns the function that opens it.
func newGate(t *testing.T) (open func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gate")
	t.Setenv("GO_HELPER_GATE_FILE", path)
	return func() { os.WriteFile(path, []byte("open"), 0644) } //nolint:errcheck
}
