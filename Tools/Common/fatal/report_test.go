package fatal

import (
	"strings"
	"sync"
	"testing"
)

func TestFail_PrintsMessageThenPathLineAndExitsOne(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() { g.r.Fail("error: boom") })

	path, _ := onlyFailureFile(t, g.dir)
	want := "error: boom\nfailure report written to " + path + "\n"
	if got := g.stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", codes)
	}
}

func TestFail_DoesNotDoubleTrailingNewline(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.FailureDir = func() (string, error) { return "", errUnresolved }
	})

	guard(t, func() { g.r.Fail("error: boom\n") })

	if got := g.stderr.String(); got != "error: boom\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestExit_ReportsNothingForAnyCode(t *testing.T) {
	for _, code := range []int{0, 1, 2, 3} {
		g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "windows" })

		guard(t, func() { g.r.Exit(code) })

		if g.stderr.String() != "" || g.stdin.Reads() != 0 {
			t.Fatalf("Exit(%d) printed %q or paused", code, g.stderr.String())
		}
		if codes := g.exits.Codes(); !equalInts(codes, []int{code}) {
			t.Fatalf("exit codes = %v, want [%d]", codes, code)
		}
	}
}

func TestExitCLI_PassesWrittenStderrThroughAndAddsOnlyPathLine(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() {
		w := g.r.Stderr()
		_, _ = w.Write([]byte("notice: starting\n"))
		_, _ = w.Write([]byte("error: boom\n"))
		g.r.ExitCLI(1)
	})

	path, file := onlyFailureFile(t, g.dir)
	want := "notice: starting\nerror: boom\nfailure report written to " + path + "\n"
	if got := g.stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if !strings.Contains(file, "Message:\nnotice: starting\nerror: boom") {
		t.Fatalf("file must hold the whole captured stderr text:\n%s", file)
	}
	if !strings.Contains(file, "Kind: failure\nExit code: 1\n") {
		t.Fatalf("file kind/code wrong:\n%s", file)
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", codes)
	}
}

func TestExitCLI_OtherCodesReportNothing(t *testing.T) {
	for _, code := range []int{0, 2, 3, 5} {
		g := newRig(t, nil)

		guard(t, func() {
			_, _ = g.r.Stderr().Write([]byte("usage: something\n"))
			g.r.ExitCLI(code)
		})

		if got := g.stderr.String(); got != "usage: something\n" {
			t.Fatalf("ExitCLI(%d) stderr = %q, want pass-through only", code, got)
		}
		if files := failureFiles(t, g.dir); len(files) != 0 {
			t.Fatalf("ExitCLI(%d) wrote %v", code, files)
		}
		if codes := g.exits.Codes(); !equalInts(codes, []int{code}) {
			t.Fatalf("exit codes = %v, want [%d]", codes, code)
		}
	}
}

func TestExitCLI_NoPathLineWhenFileCannotBeWritten(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.FailureDir = func() (string, error) { return "", errUnresolved }
	})

	guard(t, func() {
		_, _ = g.r.Stderr().Write([]byte("error: boom\n"))
		g.r.ExitCLI(1)
	})

	if got := g.stderr.String(); got != "error: boom\n" {
		t.Fatalf("stderr = %q, want unchanged output without a path line", got)
	}
}

func TestExitCLI_OneWithNoOutputRecordsPlaceholder(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() { g.r.ExitCLI(1) })

	_, file := onlyFailureFile(t, g.dir)
	if !strings.Contains(file, "Message:\n(no error output)") {
		t.Fatalf("missing placeholder message:\n%s", file)
	}
}

func TestExitCLI_LongOutputIsTruncatedFromTheFront(t *testing.T) {
	g := newRig(t, nil)
	line := strings.Repeat("x", 99) + "\n"

	guard(t, func() {
		w := g.r.Stderr()
		for i := 0; i < 1500; i++ { // about 150 KB
			_, _ = w.Write([]byte(line))
		}
		_, _ = w.Write([]byte("error: last\n"))
		g.r.ExitCLI(1)
	})

	_, file := onlyFailureFile(t, g.dir)
	if !strings.Contains(file, "Message:\n[earlier output truncated]\n") {
		t.Fatalf("file must start the message with the truncation marker")
	}
	if !strings.Contains(file, "error: last") {
		t.Fatalf("file must keep the most recent output")
	}
	if len(file) > 80*1024 {
		t.Fatalf("file holds %d bytes, want the last 64 KiB only", len(file))
	}
	if strings.Count(g.stderr.String(), line) != 1500 {
		t.Fatalf("stderr must pass through every written line unchanged")
	}
}

func TestReport_SecondTerminatingCallExitsOneSilently(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() {
		g.r.Fail("error: first")
		g.r.Fail("error: second")
		g.r.Exit(0)
	})

	if strings.Contains(g.stderr.String(), "second") {
		t.Fatalf("second call printed: %q", g.stderr.String())
	}
	if n := len(failureFiles(t, g.dir)); n != 1 {
		t.Fatalf("want one failure file, got %d", n)
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1, 1, 1}) {
		t.Fatalf("exit codes = %v, want [1 1 1]", codes)
	}
}

func TestFail_TUIOnLinuxStillWritesFileAndPrintsPathLine(t *testing.T) {
	g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "linux" })

	guard(t, func() { g.r.Fail("error: boom") })

	path, _ := onlyFailureFile(t, g.dir)
	if !strings.Contains(g.stderr.String(), "failure report written to "+path+"\n") {
		t.Fatalf("path line missing: %q", g.stderr.String())
	}
}

func TestFail_ConcurrentFailuresProduceOneReport(t *testing.T) {
	g := newRig(t, nil)

	var wg sync.WaitGroup
	for _, msg := range []string{"error: a", "error: b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { _ = recover() }()
			g.r.Fail(msg)
		}()
	}
	wg.Wait()

	if n := len(failureFiles(t, g.dir)); n != 1 {
		t.Fatalf("want one failure file, got %d", n)
	}
	out := g.stderr.String()
	if strings.Count(out, "error: a")+strings.Count(out, "error: b") != 1 {
		t.Fatalf("want exactly one message printed, got %q", out)
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1, 1}) {
		t.Fatalf("exit codes = %v, want [1 1]", codes)
	}
}
