package logappend_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"

	"mosaic-common/logappend"
)

const (
	helperEnv   = "LOGAPPEND_TEST_HELPER"
	helperTest  = "TestHelperProcessAppend"
	helperCount = 4
)

// TestHelperProcessAppend is the body of the child processes started by the parallel-process test.
// Run directly it does nothing. Arguments come from the environment: target, tag prefix, line count.
func TestHelperProcessAppend(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		t.Skip("child process body, started by the parallel process test")
	}
	target, prefix := os.Getenv(helperEnv+"_TARGET"), os.Getenv(helperEnv+"_PREFIX")
	count, _ := strconv.Atoi(os.Getenv(helperEnv + "_COUNT"))
	for i := 0; i < count; i++ {
		tag := fmt.Sprintf("%s-%d", prefix, i)
		if err := logappend.Append(target, makeLine(tag, longLineSize)); err != nil {
			t.Fatalf("child %s: Append: %v", prefix, err)
		}
	}
}

// startHelper starts a child test process that appends count long lines tagged prefix-0..prefix-(count-1).
func startHelper(t *testing.T, target, prefix string, count int) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+helperTest+"$", "-test.v")
	cmd.Env = append(os.Environ(),
		helperEnv+"=1", helperEnv+"_TARGET="+target, helperEnv+"_PREFIX="+prefix,
		fmt.Sprintf("%s_COUNT=%d", helperEnv, count))
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	return cmd
}

func TestParallelProcessesLoseTearOrBlankNoLine(t *testing.T) {
	target := newTarget(t)
	prefixes := []string{"a", "b", "c"}
	var cmds []*exec.Cmd
	for _, p := range prefixes {
		cmds = append(cmds, startHelper(t, target, p, helperCount))
	}

	var want []string
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Errorf("child %s failed: %v", prefixes[i], err)
		}
		for n := 0; n < helperCount; n++ {
			want = append(want, fmt.Sprintf("%s-%d", prefixes[i], n))
		}
	}

	assertWholeLines(t, readFile(t, target), longLineSize, want)
}

func TestProcessesAndGoroutinesTogetherLoseTearOrBlankNoLine(t *testing.T) {
	target := newTarget(t)
	cmds := []*exec.Cmd{startHelper(t, target, "a0", helperCount), startHelper(t, target, "b0", helperCount)}
	var wg sync.WaitGroup
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < helperCount; i++ {
				if err := logappend.Append(target, makeLine(fmt.Sprintf("g%d-%d", w, i), longLineSize)); err != nil {
					t.Errorf("Append: %v", err)
				}
			}
		}(w)
	}
	wg.Wait()
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Errorf("child failed: %v", err)
		}
	}

	want := append(tags("g", 3, helperCount), tags("a", 1, helperCount)...)
	want = append(want, tags("b", 1, helperCount)...)
	assertWholeLines(t, readFile(t, target), longLineSize, want)
}
