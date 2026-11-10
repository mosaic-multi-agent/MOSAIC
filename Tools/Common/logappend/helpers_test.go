package logappend_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// longLineSize is larger than any OS write buffer (pipe and stdio buffers are 4-64 KiB).
const longLineSize = 200 * 1024

// makeLine builds one long, self-describing log line: "<tag>|<filler>\n" where the filler is the
// first letter of the tag repeated size times. A torn or interleaved line breaks that shape.
func makeLine(tag string, size int) []byte {
	return []byte(tag + "|" + strings.Repeat(tag[:1], size) + "\n")
}

// newTarget returns a log file path in a fresh folder; the folder's sub-folder does not exist yet.
func newTarget(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "OrchestrationLogs", "run", "00_orchestrator_events.jsonl")
}

// lockFileOf is the lock file path the log format defines for target.
func lockFileOf(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".lock")
}

// takeLockFile holds the lock the way the log format describes: an exclusively created sidecar file.
func takeLockFile(t *testing.T, target string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(lockFileOf(target), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("take lock file: %v", err)
	}
	f.Close()
}

// dropLockFile releases a lock taken with takeLockFile.
func dropLockFile(t *testing.T, target string) {
	t.Helper()
	if err := os.Remove(lockFileOf(target)); err != nil {
		t.Fatalf("drop lock file: %v", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return data
}

// assertWholeLines checks that data holds exactly the wanted tags, each once, each as an intact
// long line of the given size, with no blank or torn line.
func assertWholeLines(t *testing.T, data []byte, size int, want []string) {
	t.Helper()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("file does not end with a newline (len %d)", len(data))
	}
	lines := bytes.Split(data[:len(data)-1], []byte("\n"))
	seen := map[string]int{}
	for i, l := range lines {
		tag, filler, ok := strings.Cut(string(l), "|")
		if !ok || tag == "" {
			t.Fatalf("line %d is blank or torn (len %d)", i, len(l))
		}
		if len(filler) != size || strings.Trim(filler, tag[:1]) != "" {
			t.Fatalf("line %d (%s) is torn or interleaved: filler len %d, want %d", i, tag, len(filler), size)
		}
		seen[tag]++
	}
	for _, tag := range want {
		if seen[tag] != 1 {
			t.Errorf("line %s appears %d times, want 1", tag, seen[tag])
		}
	}
	if len(lines) != len(want) {
		t.Errorf("file has %d lines, want %d", len(lines), len(want))
	}
}

// measure runs fn and returns how long it took.
func measure(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

func tags(prefix string, writers, perWriter int) []string {
	var out []string
	for w := 0; w < writers; w++ {
		for i := 0; i < perWriter; i++ {
			out = append(out, fmt.Sprintf("%s%d-%d", prefix, w, i))
		}
	}
	return out
}

// waitInside waits until a WithLock callback has started; it fails the test instead of hanging
// when WithLock returned without running the callback.
func waitInside(t *testing.T, inside <-chan struct{}, done <-chan error) {
	t.Helper()
	select {
	case <-inside:
	case err := <-done:
		t.Fatalf("WithLock returned (%v) without running its callback", err)
	case <-time.After(5 * time.Second):
		t.Fatal("WithLock did not run its callback within 5s")
	}
}
