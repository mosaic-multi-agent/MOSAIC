package main

// codexsetup_wiring.go holds the Codex start-of-run setup preflight session
// wrapper: it warns about an untrusted workspace and a missing contact-user
// server and, in the TUI, lets the user proceed or abort.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	commonharness "mosaic-common/harness"
	"mosaic-common/interaction"

	"mosaic-run/internal/codexsetup"
	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

// codexSetupChecker is the narrow port the preflight uses
// (*codexsetup.Checker satisfies it).
type codexSetupChecker interface {
	Check(workDir string) codexsetup.Result
}

// codexSetupConfig is the per-session input of the preflight.
type codexSetupConfig struct {
	RunFolder   string             // workDir = filepath.Dir(RunFolder)
	HarnessID   string             // checks run only for "codex"
	Checker     codexSetupChecker  // production: codexsetup.NewChecker()
	Interact    domain.Interaction // Notify for warnings; Confirm only when Interactive
	Interactive bool               // CLI: false; TUI: true
	Debug       domain.DebugLogger // diagnostic entries
}

type codexSetupPreflight struct {
	inner session.Session
	cfg   codexSetupConfig
}

// newCodexSetupPreflight returns a session wrapper placed inside the lifecycle decorator.
func newCodexSetupPreflight(inner session.Session, cfg codexSetupConfig) session.Session {
	return &codexSetupPreflight{inner: inner, cfg: cfg}
}

// withCodexSetupPreflight wraps inner with a production preflight for the harness.
func withCodexSetupPreflight(inner session.Session, runFolder, harnessID string, in domain.Interaction, interactive bool, debug domain.DebugLogger) session.Session {
	return newCodexSetupPreflight(inner, codexSetupConfig{
		RunFolder:   runFolder,
		HarnessID:   harnessID,
		Checker:     codexsetup.NewChecker(),
		Interact:    in,
		Interactive: interactive,
		Debug:       debug,
	})
}

func (p *codexSetupPreflight) Start(ctx context.Context, rc domain.RunConfig) (domain.RunOutcome, error) {
	if p.cfg.HarnessID != commonharness.HarnessIDCodex {
		return p.inner.Start(ctx, rc)
	}
	workDir := filepath.Dir(p.cfg.RunFolder)
	res := p.cfg.Checker.Check(workDir)
	p.cfg.Debug.Log("codexsetup.check", string(res.Trust)+"/"+string(res.ContactUser),
		domain.F("work_dir", workDir), domain.F("config_path", res.ConfigPath),
		domain.F("trust_reason", res.TrustReason), domain.F("contact_user_reason", res.ContactUserReason))

	var problems []string // definite problems, named in a refusal message
	switch res.Trust {
	case codexsetup.TrustUntrusted:
		problems = append(problems, "the workspace is not trusted by Codex")
		p.warn(ctx, "Workspace is not trusted by Codex",
			fmt.Sprintf("%s is not a trusted Codex project, so Codex skips the project .codex configuration "+
				"(agents and MCP servers). Trust the project in Codex "+
				"(Runner never changes your Codex configuration).", workDir))
	case codexsetup.TrustIndeterminate:
		p.warn(ctx, "Codex workspace trust could not be determined",
			fmt.Sprintf("Could not determine whether %s is trusted by Codex (%s). "+
				"If it is not trusted, Codex skips the project .codex configuration. Continuing.", workDir, res.TrustReason))
	}
	switch res.ContactUser {
	case codexsetup.ServerMissing:
		problems = append(problems, "the "+commonharness.CodexContactUserServer+" MCP server is not configured for Codex")
		p.warn(ctx, "Codex has no "+commonharness.CodexContactUserServer+" MCP server",
			"No enabled "+commonharness.CodexContactUserServer+" MCP server is configured for Codex, so agents "+
				"cannot contact the user. Configure it in your Codex home config.toml or the project .codex/config.toml "+
				"(Runner never changes your Codex configuration).")
	case codexsetup.ServerIndeterminate:
		p.warn(ctx, "Codex "+commonharness.CodexContactUserServer+" server could not be determined",
			fmt.Sprintf("Could not determine whether the %s MCP server is configured for Codex (%s). "+
				"If it is missing, agents cannot contact the user. Continuing.",
				commonharness.CodexContactUserServer, res.ContactUserReason))
	}

	if p.cfg.Interactive && len(problems) > 0 && !p.userChoosesProceed(ctx, workDir) {
		return domain.RunOutcome{
			Status:  domain.RunRefused,
			Message: "Aborted by user because Codex setup is incomplete: " + strings.Join(problems, "; "),
		}, nil
	}
	return p.inner.Start(ctx, rc)
}

func (p *codexSetupPreflight) warn(ctx context.Context, title, message string) {
	p.cfg.Interact.Notify(ctx, interaction.Notice{Level: interaction.NoticeWarning, Title: title, Message: message})
}

// userChoosesProceed asks whether to continue despite the setup problems. Only
// an explicit "no" or a cancel aborts; a failed prompt proceeds.
func (p *codexSetupPreflight) userChoosesProceed(ctx context.Context, workDir string) bool {
	ans, err := p.cfg.Interact.Confirm(ctx, interaction.Question{
		Subject: workDir,
		Title:   "Proceed despite the Codex setup problems?",
		Prompt:  "The warnings above describe Codex setup problems that can silently break this run. Proceed anyway?",
	})
	if err != nil {
		p.cfg.Debug.Log("codexsetup.confirm_error", err.Error())
		return true
	}
	switch ans.Status {
	case interaction.Answered:
		return ans.Confirm
	case interaction.Cancelled:
		return false
	default:
		return true
	}
}
