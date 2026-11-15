package main

// Tests for the CLI experimental-harness warning wrapper: one warning per
// start or resume on a marked harness, none otherwise, and the run proceeds.

import (
	"context"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/interaction"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness"
)

const cliExperimentalReason = "reason-token-4417"

func TestExperimentalNotice_MarkedHarnessWarnsOnceAndProceeds(t *testing.T) {
	restore := harness.MarkExperimentalForTest(commonharness.HarnessIDOpenCode, cliExperimentalReason)
	defer restore()
	in := &MockInteraction{}
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		return domain.RunOutcome{Status: domain.RunCompleted, Message: "inner done"}, nil
	}}

	out, err := withExperimentalNotice(inner, commonharness.HarnessIDOpenCode, in).
		Start(context.Background(), domain.RunConfig{})

	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 || out.Status != domain.RunCompleted || out.Message != "inner done" {
		t.Errorf("inner calls=%d outcome=%+v, want one delegated call with the outcome unchanged", inner.calls, out)
	}
	if len(in.Notices) != 1 {
		t.Fatalf("notices = %d, want exactly 1", len(in.Notices))
	}
	n := in.Notices[0]
	if n.Level != interaction.NoticeWarning {
		t.Errorf("notice level = %q, want warning", n.Level)
	}
	if !strings.Contains(n.Message, cliExperimentalReason) {
		t.Errorf("notice message %q does not carry the reason", n.Message)
	}
}

func TestExperimentalNotice_WarnsOncePerStartIncludingResume(t *testing.T) {
	restore := harness.MarkExperimentalForTest(commonharness.HarnessIDOpenCode, cliExperimentalReason)
	defer restore()
	in := &MockInteraction{}
	sess := withExperimentalNotice(&mlSession{}, commonharness.HarnessIDOpenCode, in)

	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: true})
	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if len(in.Notices) != 2 {
		t.Errorf("notices after a start and a resume = %d, want 2 (one each)", len(in.Notices))
	}
}

func TestExperimentalNotice_UnmarkedHarnessEmitsNothingAndProceeds(t *testing.T) {
	restore := harness.MarkExperimentalForTest(commonharness.HarnessIDOpenCode, cliExperimentalReason)
	defer restore()
	in := &MockInteraction{}
	inner := &mlSession{}

	_, err := withExperimentalNotice(inner, commonharness.HarnessIDClaudeCode, in).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 {
		t.Fatalf("err=%v inner calls=%d, want the run to proceed", err, inner.calls)
	}
	if len(in.Notices) != 0 {
		t.Errorf("notices = %d for an unmarked harness, want none", len(in.Notices))
	}
}

func TestExperimentalNotice_InnerErrorPassesThrough(t *testing.T) {
	restore := harness.MarkExperimentalForTest(commonharness.HarnessIDOpenCode, cliExperimentalReason)
	defer restore()
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		return domain.RunOutcome{}, errScriptedStart
	}}

	_, err := withExperimentalNotice(inner, commonharness.HarnessIDOpenCode, &MockInteraction{}).
		Start(context.Background(), domain.RunConfig{})

	if err != errScriptedStart {
		t.Errorf("err = %v, want the inner error unchanged", err)
	}
}
