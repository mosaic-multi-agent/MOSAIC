package main

// codexsetup_helpers_test.go holds the shared fixtures of the Codex setup
// preflight tests. MockInteraction, mlSession, mlRunFolder and errScriptedStart
// come from the other preflight tests of this package.

import (
	"strings"
	"testing"

	commonharness "mosaic-common/harness"
	"mosaic-common/interaction"

	"mosaic-run/internal/codexsetup"
	"mosaic-run/internal/domain"
)

const (
	codexTrustReason   = "trust-reason-9931"
	codexContactReason = "contact-reason-7742"
)

// MockCodexSetupChecker returns a scripted result and records every checked directory.
type MockCodexSetupChecker struct {
	Result  codexsetup.Result
	Checked []string
}

func (m *MockCodexSetupChecker) Check(workDir string) codexsetup.Result {
	m.Checked = append(m.Checked, workDir)
	return m.Result
}

// codexSetupResult builds a result with the given statuses; reasons are set
// for indeterminate statuses.
func codexSetupResult(trust codexsetup.TrustStatus, contact codexsetup.ServerStatus) codexsetup.Result {
	r := codexsetup.Result{ConfigPath: "codex-home/config.toml", Trust: trust, ContactUser: contact}
	if trust == codexsetup.TrustIndeterminate {
		r.TrustReason = codexTrustReason
	}
	if contact == codexsetup.ServerIndeterminate {
		r.ContactUserReason = codexContactReason
	}
	return r
}

var codexAllGood = codexSetupResult(codexsetup.TrustTrusted, codexsetup.ServerConfigured)

// newCodexSetupConfig builds a preflight config for the Codex harness.
func newCodexSetupConfig(runFolder string, checker codexSetupChecker, in domain.Interaction, interactive bool) codexSetupConfig {
	return codexSetupConfig{
		RunFolder:   runFolder,
		HarnessID:   commonharness.HarnessIDCodex,
		Checker:     checker,
		Interact:    in,
		Interactive: interactive,
		Debug:       domain.NopDebugLogger{},
	}
}

// noticesMentioning returns the notices whose title or message contains substr
// (case-insensitive).
func noticesMentioning(notices []interaction.Notice, substr string) []interaction.Notice {
	var out []interaction.Notice
	for _, n := range notices {
		if strings.Contains(strings.ToLower(n.Title+" "+n.Message), strings.ToLower(substr)) {
			out = append(out, n)
		}
	}
	return out
}

// assertAllWarnings fails unless every notice is a titled warning.
func assertAllWarnings(t *testing.T, notices []interaction.Notice) {
	t.Helper()
	for _, n := range notices {
		if n.Level != interaction.NoticeWarning || n.Title == "" || n.Message == "" {
			t.Errorf("notice %+v must be a warning with a title and a message", n)
		}
	}
}
