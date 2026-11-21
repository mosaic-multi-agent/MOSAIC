package cli_test

// interaction_notice_routing_test.go verifies where the non-interactive Interaction writes
// notice and progress lines: onto the single diagnostics writer it was built with, never onto
// the writer that carries a subcommand's result document.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

const noticeProbeText = "source harness does not record tool information"

// failingWriter rejects every write.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// noticeRaisingService raises a notice and a progress line through the Interaction while a
// deploy runs, the way the application layer does.
type noticeRaisingService struct {
	*spyService
	ix domain.Interaction
}

func (s *noticeRaisingService) DeployNew(ctx context.Context, req app.DeployRequest) (domain.RunSummary, error) {
	s.ix.Notify(ctx, domain.Notice{Level: domain.NoticeWarning, Message: noticeProbeText})
	s.ix.Progress(ctx, domain.ProgressEvent{Phase: "writing", Current: 1, Total: 2, Subject: "probe-subject"})
	return s.spyService.DeployNew(ctx, req)
}

func newDiagnosticsInteraction(diag io.Writer) domain.Interaction {
	return cli.NewInteraction(cli.PreAnswers{}, &stubTodo{}, diag)
}

func TestNewInteraction_Notify_WritesNoticeTextToDiagnosticsWriter(t *testing.T) {
	var diag bytes.Buffer
	ix := newDiagnosticsInteraction(&diag)

	ix.Notify(context.Background(), domain.Notice{Level: domain.NoticeWarning, Title: "probe-title", Message: noticeProbeText})

	got := diag.String()
	if !strings.Contains(got, noticeProbeText) || !strings.Contains(got, "probe-title") {
		t.Errorf("diagnostics writer holds %q, want the notice title and message", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("diagnostics writer holds %q, want exactly one line per notice", got)
	}
}

func TestNewInteraction_Progress_WritesProgressTextToDiagnosticsWriter(t *testing.T) {
	var diag bytes.Buffer
	ix := newDiagnosticsInteraction(&diag)

	ix.Progress(context.Background(), domain.ProgressEvent{Phase: "writing", Current: 3, Total: 7, Subject: "probe-subject"})

	got := diag.String()
	for _, want := range []string{"writing", "3", "7", "probe-subject"} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnostics writer holds %q, want it to contain %q", got, want)
		}
	}
}

func TestNewInteraction_NoticesAndProgress_KeepCallOrder(t *testing.T) {
	var diag bytes.Buffer
	ix := newDiagnosticsInteraction(&diag)

	ix.Notify(context.Background(), domain.Notice{Level: domain.NoticeInfo, Message: "first-notice"})
	ix.Progress(context.Background(), domain.ProgressEvent{Phase: "planning", Subject: "second-progress"})
	ix.Notify(context.Background(), domain.Notice{Level: domain.NoticeInfo, Message: "third-notice"})

	got := diag.String()
	a, b, c := strings.Index(got, "first-notice"), strings.Index(got, "second-progress"), strings.Index(got, "third-notice")
	if a < 0 || b < 0 || c < 0 || a >= b || b >= c {
		t.Errorf("diagnostics writer holds %q, want first-notice, second-progress, third-notice in that order", got)
	}
}

func TestNewInteraction_NoticeAndProgress_WithFailingWriter_ReturnWithoutPanic(t *testing.T) {
	ix := newDiagnosticsInteraction(failingWriter{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		ix.Notify(context.Background(), domain.Notice{Level: domain.NoticeWarning, Message: "x"})
		ix.Progress(context.Background(), domain.ProgressEvent{Phase: "writing"})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Notify or Progress did not return when the writer failed")
	}
}

// runDeployWithNotice runs `deploy` with the given output args; the service raises a notice and
// a progress line through an Interaction wired to the diagnostics buffer, as main.go wires it.
func runDeployWithNotice(t *testing.T, outputArgs ...string) (stdout, stderr string) {
	t.Helper()
	workspace := t.TempDir()
	var out, errOut bytes.Buffer
	svc := &noticeRaisingService{
		spyService: &spyService{deployResp: successSummary(workspace)},
		ix:         newDiagnosticsInteraction(&errOut),
	}
	args := append([]string{"deploy"}, outputArgs...)
	args = append(args, "--harness", "stub-harness", "--workspace", workspace, "--auto-confirm")

	cli.Run(context.Background(), args, svc, &out, &errOut)

	return out.String(), errOut.String()
}

func TestRun_Deploy_JsonOutput_WithRaisedNotice_StdoutIsExactlyOneJSONDocument(t *testing.T) {
	for _, spelling := range [][]string{{"--output", "json"}, {"--output=json"}, {"--output", "JSON"}} {
		t.Run(strings.Join(spelling, " "), func(t *testing.T) {
			stdout, stderr := runDeployWithNotice(t, spelling...)

			dec := json.NewDecoder(strings.NewReader(stdout))
			var doc map[string]any
			if err := dec.Decode(&doc); err != nil {
				t.Fatalf("stdout does not start with a JSON document: %v\nstdout: %q", err, stdout)
			}
			if dec.More() || strings.TrimSpace(stdout[dec.InputOffset():]) != "" {
				t.Errorf("stdout carries more than one JSON document: %q", stdout)
			}
			if !strings.Contains(stderr, noticeProbeText) || !strings.Contains(stderr, "probe-subject") {
				t.Errorf("stderr %q lost the notice or progress text", stderr)
			}
		})
	}
}

func TestRun_Deploy_HumanOutput_WithRaisedNotice_NoticeVisibleOnDiagnosticsStream(t *testing.T) {
	stdout, stderr := runDeployWithNotice(t)

	if !strings.Contains(stderr, noticeProbeText) {
		t.Errorf("stderr %q does not show the notice in a human run", stderr)
	}
	if strings.Contains(stdout, noticeProbeText) {
		t.Errorf("stdout %q carries the notice text; stdout holds only the human summary", stdout)
	}
}
