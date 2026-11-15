package main

// Tests for the Codex setup preflight wrapper: one warning per problem, the CLI
// proceeds, the TUI asks once and refuses only on an explicit No or cancel.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/interaction"

	"mosaic-run/internal/codexsetup"
	"mosaic-run/internal/domain"
)

var (
	codexUntrusted     = codexSetupResult(codexsetup.TrustUntrusted, codexsetup.ServerConfigured)
	codexNoContact     = codexSetupResult(codexsetup.TrustTrusted, codexsetup.ServerMissing)
	codexBothProblems  = codexSetupResult(codexsetup.TrustUntrusted, codexsetup.ServerMissing)
	codexTrustUnsure   = codexSetupResult(codexsetup.TrustIndeterminate, codexsetup.ServerConfigured)
	codexContactUnsure = codexSetupResult(codexsetup.TrustTrusted, codexsetup.ServerIndeterminate)
	codexBothUnsure    = codexSetupResult(codexsetup.TrustIndeterminate, codexsetup.ServerIndeterminate)
)

func TestCodexSetup_CLIUntrusted_WarnsAndProceeds(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	in := &MockInteraction{}
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		return domain.RunOutcome{Status: domain.RunCompleted, Message: "inner done"}, nil
	}}

	out, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexUntrusted}, in, false)).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 || out.Status != domain.RunCompleted || out.Message != "inner done" {
		t.Fatalf("got (%+v, %v) after %d call(s), want the wrapped outcome unchanged", out, err, inner.calls)
	}
	if len(in.Notices) != 1 {
		t.Fatalf("notices = %d, want 1 warning", len(in.Notices))
	}
	assertAllWarnings(t, in.Notices)
	if len(noticesMentioning(in.Notices, "trust")) != 1 {
		t.Errorf("the warning %+v must be about workspace trust", in.Notices[0])
	}
	if len(noticesMentioning(in.Notices, commonharness.CodexContactUserServer)) != 0 {
		t.Errorf("a trust-only problem must not produce a contact-user warning: %+v", in.Notices)
	}
	if len(in.Questions) != 0 {
		t.Errorf("CLI must never ask, got %d question(s)", len(in.Questions))
	}
}

func TestCodexSetup_CLIMissingContactUser_WarnsAndProceeds(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	in := &MockInteraction{}
	inner := &mlSession{}

	_, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexNoContact}, in, false)).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 {
		t.Fatalf("err=%v calls=%d, want the run to proceed", err, inner.calls)
	}
	if len(in.Notices) != 1 {
		t.Fatalf("notices = %d, want 1 warning", len(in.Notices))
	}
	assertAllWarnings(t, in.Notices)
	if len(noticesMentioning(in.Notices, commonharness.CodexContactUserServer)) != 1 {
		t.Errorf("the warning %+v must name the contact-user server", in.Notices[0])
	}
	if len(in.Questions) != 0 {
		t.Errorf("CLI must never ask, got %d question(s)", len(in.Questions))
	}
}

func TestCodexSetup_CLIBothProblems_OneWarningEach(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	in := &MockInteraction{}
	inner := &mlSession{}

	_, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexBothProblems}, in, false)).
		Start(context.Background(), domain.RunConfig{})

	if err != nil || inner.calls != 1 {
		t.Fatalf("err=%v calls=%d, want the run to proceed", err, inner.calls)
	}
	if len(in.Notices) != 2 {
		t.Fatalf("notices = %d, want one warning per problem (2)", len(in.Notices))
	}
	assertAllWarnings(t, in.Notices)
	if len(noticesMentioning(in.Notices, commonharness.CodexContactUserServer)) != 1 ||
		len(noticesMentioning(in.Notices, "trust")) < 1 {
		t.Errorf("notices %+v must cover trust and contact-user separately", in.Notices)
	}
}

func TestCodexSetup_Indeterminate_WarnsWithReasonAndProceeds(t *testing.T) {
	cases := []struct {
		name         string
		result       codexsetup.Result
		wantWarnings int
		wantReasons  []string
	}{
		{"trust undecidable", codexTrustUnsure, 1, []string{codexTrustReason}},
		{"contact-user undecidable", codexContactUnsure, 1, []string{codexContactReason}},
		{"both undecidable", codexBothUnsure, 2, []string{codexTrustReason, codexContactReason}},
	}
	for _, tc := range cases {
		for _, interactive := range []bool{false, true} {
			mode := "cli"
			if interactive {
				mode = "tui"
			}
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				runFolder, _ := mlRunFolder(t)
				in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Cancelled}}
				inner := &mlSession{}

				out, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: tc.result}, in, interactive)).
					Start(context.Background(), domain.RunConfig{})

				if err != nil || out.Status != domain.RunCompleted || inner.calls != 1 {
					t.Fatalf("interactive=%v: got (%+v, %v) after %d call(s), want proceed", interactive, out, err, inner.calls)
				}
				if len(in.Notices) != tc.wantWarnings {
					t.Fatalf("interactive=%v: notices = %d, want %d", interactive, len(in.Notices), tc.wantWarnings)
				}
				assertAllWarnings(t, in.Notices)
				for _, reason := range tc.wantReasons {
					if len(noticesMentioning(in.Notices, reason)) != 1 {
						t.Errorf("interactive=%v: no single warning carries the reason %q: %+v", interactive, reason, in.Notices)
					}
				}
				if len(in.Questions) != 0 {
					t.Errorf("interactive=%v: undecidable-only results must not ask, got %d question(s)", interactive, len(in.Questions))
				}
			})
		}
	}
}

func TestCodexSetup_AllGood_NoWarningNoQuestion(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		runFolder, _ := mlRunFolder(t)
		in := &MockInteraction{}
		inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
			return domain.RunOutcome{Status: domain.RunStopped}, errScriptedStart
		}}

		out, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexAllGood}, in, interactive)).
			Start(context.Background(), domain.RunConfig{})

		if inner.calls != 1 || !errors.Is(err, errScriptedStart) || out.Status != domain.RunStopped {
			t.Errorf("interactive=%v: got (%+v, %v) after %d call(s), want the wrapped result unchanged", interactive, out, err, inner.calls)
		}
		if len(in.Notices) != 0 || len(in.Questions) != 0 {
			t.Errorf("interactive=%v: notices=%d questions=%d, want none", interactive, len(in.Notices), len(in.Questions))
		}
	}
}

func TestCodexSetup_TUIAnswerRows(t *testing.T) {
	problems := []struct {
		name         string
		result       codexsetup.Result
		wantWarnings int
	}{
		{"untrusted", codexUntrusted, 1},
		{"contact-user missing", codexNoContact, 1},
		{"both problems", codexBothProblems, 2},
		{"problem plus undecidable check", codexSetupResult(codexsetup.TrustIndeterminate, codexsetup.ServerMissing), 2},
	}
	answers := []struct {
		name      string
		reply     interaction.ConfirmAnswer
		replyErr  error
		wantStart bool
	}{
		{"answered yes proceeds", interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: true}, nil, true},
		{"answered no refuses", interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: false}, nil, false},
		{"cancelled refuses", interaction.ConfirmAnswer{Status: interaction.Cancelled}, nil, false},
		{"skipped one proceeds", interaction.ConfirmAnswer{Status: interaction.SkippedOne}, nil, true},
		{"skipped all proceeds", interaction.ConfirmAnswer{Status: interaction.SkippedAll}, nil, true},
		{"confirm error proceeds", interaction.ConfirmAnswer{Status: interaction.Cancelled}, errors.New("prompt failed"), true},
	}
	for _, p := range problems {
		for _, a := range answers {
			t.Run(p.name+"/"+a.name, func(t *testing.T) {
				runFolder, _ := mlRunFolder(t)
				in := &MockInteraction{ConfirmReply: a.reply, ConfirmErr: a.replyErr}
				inner := &mlSession{}

				out, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: p.result}, in, true)).
					Start(context.Background(), domain.RunConfig{})

				if err != nil {
					t.Fatalf("err = %v, want nil (a refusal is an outcome, not an error)", err)
				}
				if (inner.calls == 1) != a.wantStart || inner.calls > 1 {
					t.Fatalf("wrapped Start calls = %d, want called=%v", inner.calls, a.wantStart)
				}
				if a.wantStart && out.Status != domain.RunCompleted {
					t.Errorf("outcome = %+v, want the wrapped outcome", out)
				}
				if !a.wantStart && (out.Status != domain.RunRefused || strings.TrimSpace(out.Message) == "") {
					t.Errorf("outcome = %+v, want RunRefused with a message", out)
				}
				if len(in.Notices) != p.wantWarnings {
					t.Errorf("notices = %d, want %d warning(s) shown before the question", len(in.Notices), p.wantWarnings)
				}
				assertAllWarnings(t, in.Notices)
				if len(in.Questions) != 1 {
					t.Fatalf("questions = %d, want exactly 1 even with several problems", len(in.Questions))
				}
			})
		}
	}
}

func TestCodexSetup_TUIRefusalMessageNamesTheProblems(t *testing.T) {
	cases := []struct {
		name   string
		result codexsetup.Result
		want   []string
	}{
		{"untrusted", codexUntrusted, []string{"trust"}},
		{"contact-user missing", codexNoContact, []string{commonharness.CodexContactUserServer}},
		{"both", codexBothProblems, []string{"trust", commonharness.CodexContactUserServer}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runFolder, _ := mlRunFolder(t)
			in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: false}}

			out, _ := newCodexSetupPreflight(&mlSession{}, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: tc.result}, in, true)).
				Start(context.Background(), domain.RunConfig{})

			for _, w := range tc.want {
				if !strings.Contains(strings.ToLower(out.Message), strings.ToLower(w)) {
					t.Errorf("refusal message %q does not name %q", out.Message, w)
				}
			}
		})
	}
}

func TestCodexSetup_TUIQuestionContract(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	in := &MockInteraction{ConfirmReply: interaction.ConfirmAnswer{Status: interaction.Answered, Confirm: true}}

	_, _ = newCodexSetupPreflight(&mlSession{}, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexBothProblems}, in, true)).
		Start(context.Background(), domain.RunConfig{})

	if len(in.Questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(in.Questions))
	}
	if q := in.Questions[0]; q.Title == "" || q.Prompt == "" {
		t.Errorf("question needs a title and a prompt: %+v", q)
	}
}

func TestCodexSetup_OtherHarnesses_SkipTheCheck(t *testing.T) {
	for _, harnessID := range []string{"claude-code", "opencode", "ghcp-cli", "fake", ""} {
		for _, interactive := range []bool{false, true} {
			runFolder, _ := mlRunFolder(t)
			in := &MockInteraction{}
			inner := &mlSession{}
			checker := &MockCodexSetupChecker{Result: codexBothProblems}
			cfg := newCodexSetupConfig(runFolder, checker, in, interactive)
			cfg.HarnessID = harnessID

			out, err := newCodexSetupPreflight(inner, cfg).Start(context.Background(), domain.RunConfig{})

			if err != nil || out.Status != domain.RunCompleted || inner.calls != 1 {
				t.Errorf("harness %q: got (%+v, %v) after %d call(s), want pass-through", harnessID, out, err, inner.calls)
			}
			if len(checker.Checked) != 0 || len(in.Notices) != 0 || len(in.Questions) != 0 {
				t.Errorf("harness %q: checked=%d notices=%d questions=%d, want none",
					harnessID, len(checker.Checked), len(in.Notices), len(in.Questions))
			}
		}
	}
}

func TestCodexSetup_ChecksTheRunFolderParent(t *testing.T) {
	runFolder, ws := mlRunFolder(t)
	checker := &MockCodexSetupChecker{Result: codexAllGood}

	_, _ = newCodexSetupPreflight(&mlSession{}, newCodexSetupConfig(runFolder, checker, &MockInteraction{}, false)).
		Start(context.Background(), domain.RunConfig{})

	if len(checker.Checked) != 1 || filepath.Clean(checker.Checked[0]) != filepath.Clean(ws) {
		t.Fatalf("checked = %v, want [%q]", checker.Checked, ws)
	}
}

func TestCodexSetup_ChecksOnEveryStartIncludingResume(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	in := &MockInteraction{}
	inner := &mlSession{}
	checker := &MockCodexSetupChecker{Result: codexUntrusted}
	sess := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, checker, in, false))

	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: true})
	_, _ = sess.Start(context.Background(), domain.RunConfig{IsNewRun: false})

	if len(checker.Checked) != 2 || len(in.Notices) != 2 || inner.calls != 2 {
		t.Fatalf("checks=%d notices=%d starts=%d, want 2 each (resume re-checks)",
			len(checker.Checked), len(in.Notices), inner.calls)
	}
}

func TestCodexSetup_ProceedReturnsWrappedErrorUnchanged(t *testing.T) {
	runFolder, _ := mlRunFolder(t)
	inner := &mlSession{start: func(context.Context, domain.RunConfig) (domain.RunOutcome, error) {
		return domain.RunOutcome{Status: domain.RunStopped, Message: "m"}, errScriptedStart
	}}

	out, err := newCodexSetupPreflight(inner, newCodexSetupConfig(runFolder, &MockCodexSetupChecker{Result: codexUntrusted}, &MockInteraction{}, false)).
		Start(context.Background(), domain.RunConfig{})

	if !errors.Is(err, errScriptedStart) || out.Status != domain.RunStopped || out.Message != "m" {
		t.Fatalf("got (%+v, %v), want the wrapped outcome and error unchanged", out, err)
	}
}
