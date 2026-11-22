package launch_test

// Tests that a subject's output reaches the per-run diagnostic file written
// by diaglog, both when the subject exits normally and when it is killed by
// its timeout.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/diaglog"
	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/launch"
)

// helperStdout is what the lines_exit and lines_hang helper commands print:
// two lines, 32 bytes in total including both newlines.
const helperStdout = "line-one-marker\nline-two-marker\n"

const helperStdoutBytes = "stdout_bytes=32"

// launchWithDiagFile launches plan through a Launcher whose sink is a diaglog
// file, returning the diag file content, the Response the decoder received
// and the Launch result.
func launchWithDiagFile(t *testing.T, plan domain.SpawnPlan) (diag string, resp commonharness.Response, result domain.SubjectResult) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diag.log")
	factory := func(domain.SpawnPlan) (commonharness.Sink, io.Closer, error) {
		s, err := diaglog.Open(path, "20260101T120000Z-0001")
		return s, s, err
	}
	dec := func(res commonharness.Response, err error) (domain.SubjectResult, error) {
		resp = res
		return domain.SubjectResult{ProtocolMessage: "decoded", Disposition: domain.DispositionCompleted}, nil
	}

	result, err := launch.New(dec, launch.WithSinkFactory(factory)).Launch(context.Background(), plan)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile diag: %v", err)
	}
	return string(data), resp, result
}

// diagLineWith returns the first diag line starting with prefix, or "".
func diagLineWith(diag, prefix string) string {
	for _, l := range strings.Split(diag, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

func TestLaunch_KilledSubjectLeavesPreKillOutputAndEndReasonInDiagFile(t *testing.T) {
	setHelperEnv(t, "lines_hang")
	plan := testPlan(t)
	plan.Timeout = 1500 * time.Millisecond

	diag, _, _ := launchWithDiagFile(t, plan)

	for _, want := range []string{"line-one-marker", "line-two-marker", "helper-stderr-marker"} {
		if !strings.Contains(diag, want) {
			t.Errorf("diag file is missing %q; content:\n%s", want, diag)
		}
	}
	end := diagLineWith(diag, "spawn.end")
	if !strings.Contains(end, "reason=timeout") {
		t.Errorf("spawn.end entry %q lacks the end reason reason=timeout; diag:\n%s", end, diag)
	}
	if !strings.Contains(end, helperStdoutBytes) {
		t.Errorf("spawn.end entry %q lacks the stdout byte count %s; diag:\n%s", end, helperStdoutBytes, diag)
	}
}

func TestLaunch_NormalExitLeavesEachStdoutLineOnceAndByteCountInDiagFile(t *testing.T) {
	setHelperEnv(t, "lines_exit")

	diag, _, _ := launchWithDiagFile(t, testPlan(t))

	one := strings.Count(diag, "line-one-marker")
	two := strings.Count(diag, "line-two-marker")
	if one != 1 || two != 1 {
		t.Errorf("stdout lines appear %d and %d times in diag, want exactly once each (a whole-stdout block would duplicate them); diag:\n%s", one, two, diag)
	}
	if strings.Index(diag, "line-one-marker") > strings.Index(diag, "line-two-marker") {
		t.Errorf("stdout lines are out of arrival order in diag:\n%s", diag)
	}
	end := diagLineWith(diag, "spawn.end")
	if !strings.Contains(end, "reason=exit") || !strings.Contains(end, helperStdoutBytes) {
		t.Errorf("spawn.end entry %q lacks reason=exit or %s; diag:\n%s", end, helperStdoutBytes, diag)
	}
}

func TestLaunch_NormalExitStillPassesRunResponseToDecoder(t *testing.T) {
	setHelperEnv(t, "lines_exit")

	_, resp, result := launchWithDiagFile(t, testPlan(t))

	if string(resp.Stdout) != helperStdout {
		t.Errorf("decoder received Stdout %q, want the subject's whole stdout %q", resp.Stdout, helperStdout)
	}
	if result.ProtocolMessage != "decoded" {
		t.Errorf("Launch result ProtocolMessage = %q, want the decoder's result", result.ProtocolMessage)
	}
}
