package logappend_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"mosaic-common/logappend"
)

// pythonCommand returns an interpreter that runs, or skips the test with the reason.
func pythonCommand(t *testing.T) string {
	t.Helper()
	name := "python3"
	if runtime.GOOS == "windows" {
		name = "py"
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("no Python interpreter (%s) on PATH: %v", name, err)
	}
	if out, err := exec.Command(path, "--version").CombinedOutput(); err != nil {
		t.Skipf("Python interpreter %s does not run: %v (%s)", path, err, strings.TrimSpace(string(out)))
	}
	return path
}

func driverPath(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd + string(os.PathSeparator) + "testdata" + string(os.PathSeparator) + "append_driver.py"
}

func pythonCmd(t *testing.T, py string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.Command(py, append([]string{"-I", driverPath(t)}, args...)...)
}

func TestPythonWriterWaitsForLockHeldByGo(t *testing.T) {
	py := pythonCommand(t)
	target := newTarget(t)
	inside := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- logappend.WithLock(target, func() error {
			close(inside)
			<-release
			return os.WriteFile(target, makeLine("g0-0", 20), 0o644)
		})
	}()
	waitInside(t, inside, done)
	cmd := pythonCmd(t, py, "append", target, "p0", "1", "20")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	close(release)

	if err := cmd.Wait(); err != nil {
		t.Fatalf("python append failed: %v (%s)", err, out.String())
	}
	if err := <-done; err != nil {
		t.Fatalf("WithLock: %v", err)
	}

	data := readFile(t, target)
	assertWholeLines(t, data, 20, []string{"g0-0", "p0-0"})
	if !strings.HasPrefix(string(data), "g0-0|") {
		t.Errorf("python wrote before Go released its lock:\n%s", data)
	}
}

func TestGoWriterWaitsForLockHeldByPython(t *testing.T) {
	py := pythonCommand(t)
	target := newTarget(t)
	cmd := pythonCmd(t, py, "hold", target, "0.8", "p0-0|"+"p")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if first, _ := bufio.NewReader(stdout).ReadString('\n'); strings.TrimSpace(first) != "locked" {
		_ = cmd.Wait()
		t.Fatalf("python did not take the lock, said %q", first)
	}
	var aerr error

	elapsed := measure(func() { aerr = logappend.Append(target, makeLine("g0-0", 1)) })

	if aerr != nil {
		t.Fatalf("Append: %v", aerr)
	}
	if elapsed < 400*time.Millisecond {
		t.Errorf("Go appended after %v while Python held the lock", elapsed)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("python hold failed: %v", err)
	}
	if !strings.HasPrefix(string(readFile(t, target)), "p0-0|p\n") {
		t.Errorf("Go wrote before Python released its lock:\n%s", readFile(t, target))
	}
}

func TestGoGivesUpWhilePythonHoldsLockPastWaitLimit(t *testing.T) {
	py := pythonCommand(t)
	target := newTarget(t)
	cmd := pythonCmd(t, py, "hold", target, "3.5", "p0-0|p")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_, _ = bufio.NewReader(stdout).ReadString('\n')

	aerr := logappend.Append(target, makeLine("g0-0", 1))

	if aerr == nil || !strings.Contains(aerr.Error(), logappend.ErrLockTimeout.Error()) {
		t.Errorf("err = %v, want ErrLockTimeout while Python holds the lock", aerr)
	}
	_ = cmd.Wait()
	if strings.Contains(string(readFile(t, target)), "g0-0") {
		t.Error("timed-out Go append wrote its line anyway")
	}
}

func TestGoAndPythonWritersTogetherLoseTearOrBlankNoLine(t *testing.T) {
	py := pythonCommand(t)
	target := newTarget(t)
	const perWriter = 4
	var cmds []*exec.Cmd
	for _, prefix := range []string{"p0", "p1"} {
		c := pythonCmd(t, py, "append", target, prefix, fmt.Sprint(perWriter), fmt.Sprint(longLineSize))
		if err := c.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, c)
	}
	var wg sync.WaitGroup
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := logappend.Append(target, makeLine(fmt.Sprintf("g%d-%d", w, i), longLineSize)); err != nil {
					t.Errorf("Append: %v", err)
				}
			}
		}(w)
	}
	wg.Wait()
	for _, c := range cmds {
		if err := c.Wait(); err != nil {
			t.Errorf("python writer failed: %v", err)
		}
	}

	want := append(tags("g", 3, perWriter), tags("p", 2, perWriter)...)
	assertWholeLines(t, readFile(t, target), longLineSize, want)
}
