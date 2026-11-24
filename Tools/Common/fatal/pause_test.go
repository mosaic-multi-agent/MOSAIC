package fatal

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPause_FatalFailureOfWindowsTUIWaitsForEnterAfterMessageAndPath(t *testing.T) {
	g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "windows" })

	guard(t, func() { g.r.Fail("error: boom") })

	path, _ := onlyFailureFile(t, g.dir)
	out := g.stderr.String()
	iMsg := strings.Index(out, "error: boom")
	iPath := strings.Index(out, "failure report written to "+path)
	iPrompt := strings.Index(out, PausePrompt)
	if iMsg < 0 || iPath < 0 || iPrompt < 0 || !(iMsg < iPath && iPath < iPrompt) {
		t.Fatalf("want message, path line, prompt in that order, got %q", out)
	}
	if g.stdin.Reads() == 0 {
		t.Fatalf("the reporter must read stdin to wait for Enter")
	}
	if !g.stdin.PromptSeenAtRead() {
		t.Fatalf("the prompt must be printed before stdin is read")
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1] after the pause", codes)
	}
}

func TestPause_PromptTextIsFixed(t *testing.T) {
	if PausePrompt != "Press Enter to exit..." {
		t.Fatalf("PausePrompt = %q", PausePrompt)
	}
}

func TestPause_FatalFailureOfWindowsTUIStillPausesWithoutFailureFile(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.Mode = ModeTUI
		c.GOOS = "windows"
		c.FailureDir = func() (string, error) { return "", errUnresolved }
	})

	guard(t, func() { g.r.Fail("error: boom") })

	out := g.stderr.String()
	if !strings.Contains(out, "error: boom") || !strings.Contains(out, PausePrompt) {
		t.Fatalf("want message and prompt, got %q", out)
	}
	if strings.Contains(out, "failure report written") {
		t.Fatalf("no path line expected without a file: %q", out)
	}
}

func TestPause_NeverHappensOutsideAFatalFailureOfAWindowsTUI(t *testing.T) {
	cases := []struct {
		name string
		goos string
		mode Mode
		act  func(r *Reporter)
	}{
		{"linux tui failure", "linux", ModeTUI, func(r *Reporter) { r.Fail("error: x") }},
		{"darwin tui failure", "darwin", ModeTUI, func(r *Reporter) { r.Fail("error: x") }},
		{"windows cli failure", "windows", ModeCLI, func(r *Reporter) { r.Fail("error: x") }},
		{"windows cli exit code 1", "windows", ModeCLI, func(r *Reporter) { r.ExitCLI(1) }},
		{"windows tui success", "windows", ModeTUI, func(r *Reporter) { r.Exit(0) }},
		{"windows tui documented outcome", "windows", ModeTUI, func(r *Reporter) { r.Exit(2) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t, func(c *Config) { c.Mode = tc.mode; c.GOOS = tc.goos })

			guard(t, func() { tc.act(g.r) })

			if g.stdin.Reads() != 0 {
				t.Fatalf("stdin was read: the process paused")
			}
			if strings.Contains(g.stderr.String(), PausePrompt) {
				t.Fatalf("prompt printed: %q", g.stderr.String())
			}
		})
	}
}

// failingReader is a stdin whose read always fails.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("stdin broke") }

func TestPause_PromptIsPrintedLastWithoutTrailingNewline(t *testing.T) {
	g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "windows" })

	guard(t, func() { g.r.Fail("error: boom") })

	if out := g.stderr.String(); !strings.HasSuffix(out, PausePrompt) {
		t.Fatalf("output must end with the prompt, no newline after it: %q", out)
	}
}

func TestPause_StdinAtEOFOrReadErrorStillEndsWithExitOneAndNoSecondError(t *testing.T) {
	readers := map[string]io.Reader{
		"eof":        strings.NewReader(""),
		"read error": failingReader{},
	}
	for name, in := range readers {
		t.Run(name, func(t *testing.T) {
			g := newRig(t, func(c *Config) {
				c.Mode = ModeTUI
				c.GOOS = "windows"
				c.Stdin = in
			})

			guard(t, func() { g.r.Fail("error: boom") })

			path, _ := onlyFailureFile(t, g.dir)
			want := "error: boom\nfailure report written to " + path + "\n" + PausePrompt
			if got := g.stderr.String(); got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
			if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
				t.Fatalf("exit codes = %v, want [1]", codes)
			}
		})
	}
}
