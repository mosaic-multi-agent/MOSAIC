package harness_test

// Silence-shaped modes of the fake-subprocess helper (see helper_test.go) used
// by the no-output watchdog tests. Timings come from environment variables so a
// test states them next to the watchdog duration it checks.
//
//   - wd-drip: GO_HELPER_DRIP_COUNT lines, GO_HELPER_DRIP_MS apart, then exit 0.
//   - wd-drip-then-hang: the same lines, then silence for a long time.
//   - wd-silent-then-line: silent for GO_HELPER_SILENT_MS, then "done", exit 0.
//   - wd-silent-exit: silent for GO_HELPER_SILENT_MS, then exit 0 with no output.
//   - wd-stderr-drip: stderr lines every 200 ms for ~20 s, never any stdout.
//   - wd-v2-start-then-hang: one V2 step_start line, then silence.

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func envMillis(name string) time.Duration {
	n, _ := strconv.Atoi(os.Getenv(name))
	return time.Duration(n) * time.Millisecond
}

func dripLines() {
	count, _ := strconv.Atoi(os.Getenv("GO_HELPER_DRIP_COUNT"))
	gap := envMillis("GO_HELPER_DRIP_MS")
	for i := 0; i < count; i++ {
		writeOut("tick-" + strconv.Itoa(i) + "\n")
		time.Sleep(gap)
	}
}

// runNoOutputHelper handles the watchdog helper modes. It returns normally for
// modes that exit zero.
func runNoOutputHelper(cmd string) {
	switch cmd {
	case "wd-drip":
		dripLines()
	case "wd-drip-then-hang":
		dripLines()
		time.Sleep(24 * time.Hour)
	case "wd-silent-then-line":
		time.Sleep(envMillis("GO_HELPER_SILENT_MS"))
		writeOut("done\n")
	case "wd-silent-exit":
		time.Sleep(envMillis("GO_HELPER_SILENT_MS"))
	case "wd-stderr-drip":
		for i := 0; i < 100; i++ {
			os.Stderr.WriteString("noise\n") //nolint:errcheck
			time.Sleep(200 * time.Millisecond)
		}
	case "wd-v2-start-then-hang":
		writeOut(v2HelperStartLine)
		time.Sleep(24 * time.Hour)
	default:
		os.Stderr.WriteString("unknown watchdog helper mode " + cmd + "\n") //nolint:errcheck
		os.Exit(2)
	}
}

// setDrip configures the wd-drip family of modes.
func setDrip(t *testing.T, count int, gap time.Duration) {
	t.Helper()
	t.Setenv("GO_HELPER_DRIP_COUNT", strconv.Itoa(count))
	t.Setenv("GO_HELPER_DRIP_MS", strconv.Itoa(int(gap/time.Millisecond)))
}

// setSilence configures the silent period of the wd-silent-* modes.
func setSilence(t *testing.T, d time.Duration) {
	t.Helper()
	t.Setenv("GO_HELPER_SILENT_MS", strconv.Itoa(int(d/time.Millisecond)))
}
