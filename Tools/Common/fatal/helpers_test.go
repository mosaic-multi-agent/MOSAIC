package fatal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer is a goroutine-safe io.Writer used as the injected stderr.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// exitRecorder is an injected Exit seam that returns, so tests continue after a report.
type exitRecorder struct {
	mu    sync.Mutex
	codes []int
}

func (e *exitRecorder) exit(code int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.codes = append(e.codes, code)
}

func (e *exitRecorder) Codes() []int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]int(nil), e.codes...)
}

// stdinProbe is an injected stdin that records whether it was read and whether the
// prompt was already on stderr at that moment.
type stdinProbe struct {
	mu             sync.Mutex
	data           string
	pos            int
	reads          int
	promptAtRead   bool
	stderrForProbe *lockedBuffer
}

func (s *stdinProbe) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if s.stderrForProbe != nil && strings.Contains(s.stderrForProbe.String(), PausePrompt) {
		s.promptAtRead = true
	}
	if s.pos >= len(s.data) {
		return 0, os.ErrClosed
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func (s *stdinProbe) Reads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

func (s *stdinProbe) PromptSeenAtRead() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.promptAtRead
}

var fixedNow = time.Date(2026, 11, 23, 17, 31, 44, 123000000, time.UTC)

const testExe = `C:\tools\mosaic-deploy.exe`

// rig bundles a reporter with all its injected seams.
type rig struct {
	t      *testing.T
	dir    string
	stderr *lockedBuffer
	exits  *exitRecorder
	stdin  *stdinProbe
	cfg    Config
	r      *Reporter
}

// newRig installs a reporter with a CLI-mode, Linux, fixed-clock config writing its
// failure files into a fresh temp folder. mutate may change the config before Install.
func newRig(t *testing.T, mutate func(*Config)) *rig {
	t.Helper()
	g := &rig{
		t:      t,
		dir:    t.TempDir(),
		stderr: &lockedBuffer{},
		exits:  &exitRecorder{},
		stdin:  &stdinProbe{data: "\n"},
	}
	g.stdin.stderrForProbe = g.stderr
	g.cfg = Config{
		Tool:       "mosaic-deploy",
		Version:    "1.14.0",
		Mode:       ModeCLI,
		Args:       []string{testExe, "render", "--output", "json"},
		GOOS:       "linux",
		Stdin:      g.stdin,
		Stderr:     g.stderr,
		Exit:       g.exits.exit,
		Now:        func() time.Time { return fixedNow },
		FailureDir: func() (string, error) { return g.dir, nil },
	}
	if mutate != nil {
		mutate(&g.cfg)
	}
	t.Cleanup(Reset)
	g.r = Install(g.cfg)
	return g
}

// guard fails the test (instead of crashing the test binary) when fn panics.
func guard(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("call panicked instead of completing: %v", p)
		}
	}()
	fn()
}

// failureFiles lists the failure files in dir.
func failureFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "mosaic-*-failure-*.txt"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return m
}

// onlyFailureFile asserts exactly one failure file exists and returns its path and text.
func onlyFailureFile(t *testing.T, dir string) (string, string) {
	t.Helper()
	files := failureFiles(t, dir)
	if len(files) != 1 {
		t.Fatalf("want exactly one failure file in %s, got %v", dir, files)
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("read failure file: %v", err)
	}
	return files[0], strings.ReplaceAll(string(b), "\r\n", "\n")
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var errUnresolved = errors.New("failure folder cannot be resolved")
