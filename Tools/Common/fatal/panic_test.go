package fatal

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

//go:noinline
func panicInNamedWorker() {
	panic("worker boom")
}

// runPanickingGoroutine runs body in a goroutine that defers Recover last-registered
// (so it runs first), with a catch-all outside it so a missing Recover cannot crash
// the test binary.
func runPanickingGoroutine(t *testing.T, body func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		defer Recover()
		body()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("goroutine did not finish")
	}
}

func TestRecover_ReportsPanicWithValueAndStackAndWritesFile(t *testing.T) {
	g := newRig(t, nil)

	runPanickingGoroutine(t, panicInNamedWorker)

	out := g.stderr.String()
	if !strings.Contains(out, "panic: worker boom") {
		t.Fatalf("report lacks the panic value: %q", out)
	}
	if !strings.Contains(out, "panicInNamedWorker") {
		t.Fatalf("report lacks the stack of the panicking goroutine: %q", out)
	}
	path, file := onlyFailureFile(t, g.dir)
	if !strings.Contains(out, "failure report written to "+path) {
		t.Fatalf("path line missing: %q", out)
	}
	for _, want := range []string{"Kind: panic\nExit code: 1\n", "Message:\npanic: worker boom", "\n\nStack:\n", "panicInNamedWorker"} {
		if !strings.Contains(file, want) {
			t.Fatalf("file lacks %q:\n%s", want, file)
		}
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1] (never Go's 2)", codes)
	}
}

func TestRecover_WindowsTUIPanicPausesAfterReport(t *testing.T) {
	g := newRig(t, func(c *Config) { c.Mode = ModeTUI; c.GOOS = "windows" })

	runPanickingGoroutine(t, panicInNamedWorker)

	out := g.stderr.String()
	if strings.Index(out, "panic: worker boom") < 0 || strings.Index(out, PausePrompt) < strings.Index(out, "panic: worker boom") {
		t.Fatalf("want panic report followed by the prompt, got %q", out)
	}
	if g.stdin.Reads() == 0 {
		t.Fatalf("a TUI panic on Windows must wait for Enter")
	}
}

func TestRecover_CLIPanicAddsPathLineAndNeverPauses(t *testing.T) {
	g := newRig(t, func(c *Config) { c.GOOS = "windows" })

	runPanickingGoroutine(t, panicInNamedWorker)

	if g.stdin.Reads() != 0 {
		t.Fatalf("CLI mode must not pause")
	}
	path, _ := onlyFailureFile(t, g.dir)
	if !strings.Contains(g.stderr.String(), "failure report written to "+path+"\n") {
		t.Fatalf("path line missing: %q", g.stderr.String())
	}
}

func TestRecover_PanicWithUnwritableLocationStillReports(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.FailureDir = func() (string, error) { return "", errUnresolved }
	})

	runPanickingGoroutine(t, panicInNamedWorker)

	out := g.stderr.String()
	if !strings.Contains(out, "panic: worker boom") || strings.Contains(out, "failure report written") {
		t.Fatalf("want the panic report only, got %q", out)
	}
}

func TestRecover_ConcurrentPanicsProduceOneReport(t *testing.T) {
	g := newRig(t, nil)

	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			defer func() { _ = recover() }()
			defer Recover()
			panic("racing")
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("goroutine did not finish")
		}
	}

	if n := len(failureFiles(t, g.dir)); n != 1 {
		t.Fatalf("want one failure file, got %d", n)
	}
	if n := strings.Count(g.stderr.String(), "panic: racing"); n != 1 {
		t.Fatalf("want one printed report, got %d in %q", n, g.stderr.String())
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1, 1}) {
		t.Fatalf("exit codes = %v, want [1 1]", codes)
	}
}

func TestRecover_WithoutReporterDoesNotRecover(t *testing.T) {
	if os.Getenv("FATAL_TEST_UNWIRED") == "1" {
		defer Recover()
		panic("unwired boom")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestRecover_WithoutReporterDoesNotRecover$")
	cmd.Env = append(os.Environ(), "FATAL_TEST_UNWIRED=1")
	out, err := cmd.CombinedOutput()

	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("want Go's standard panic exit code 2, got err=%v\n%s", err, out)
	}
	if !strings.Contains(string(out), "panic: unwired boom") || strings.Contains(string(out), "failure report") {
		t.Fatalf("want Go's standard panic output only, got:\n%s", out)
	}
}

func TestRecover_ReportsBeforeOtherDeferredFunctionsOfTheGoroutineRun(t *testing.T) {
	g := newRig(t, nil)
	reportedWhenLaterDeferRan := make(chan bool, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			reportedWhenLaterDeferRan <- strings.Contains(g.stderr.String(), "panic: ordered boom")
		}()
		defer func() { _ = recover() }()
		defer Recover()
		panic("ordered boom")
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("goroutine did not finish")
	}

	if !<-reportedWhenLaterDeferRan {
		t.Fatalf("the panic must be reported before the goroutine's other deferred functions run")
	}
}

func TestRecover_FormatsNonStringPanicValues(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"error value", errors.New("err boom"), "panic: err boom"},
		{"integer value", 42, "panic: 42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newRig(t, nil)

			runPanickingGoroutine(t, func() { panic(tc.value) })

			if !strings.Contains(g.stderr.String(), tc.want) {
				t.Fatalf("want %q in %q", tc.want, g.stderr.String())
			}
			_, file := onlyFailureFile(t, g.dir)
			if !strings.Contains(file, "Message:\n"+tc.want) {
				t.Fatalf("file lacks %q:\n%s", tc.want, file)
			}
		})
	}
}

func TestInstall_LaterReporterReplacesTheEarlierOne(t *testing.T) {
	first := newRig(t, nil)
	second := newRig(t, nil)

	runPanickingGoroutine(t, panicInNamedWorker)

	if !strings.Contains(second.stderr.String(), "panic: worker boom") {
		t.Fatalf("the later reporter must report: %q", second.stderr.String())
	}
	if first.stderr.String() != "" || len(failureFiles(t, first.dir)) != 0 {
		t.Fatalf("the replaced reporter must stay silent: %q", first.stderr.String())
	}
}
