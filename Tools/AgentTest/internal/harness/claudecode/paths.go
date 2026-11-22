// Package claudecode is the Claude Code harness adapter: the first and, in
// this effort, only real implementation of domain.HarnessAdapter. Everything
// Claude-Code-specific in the module lives here; no package outside
// internal/harness/ and cmd/ may import it.
//
// This file declares the harness's project-scoped layout. The harness identity
// and the agents and hooks directories come from the shared builtin descriptor
// via mosaic-common/harness; only facts no descriptor carries are local.
package claudecode

import commonharness "mosaic-common/harness"

// HarnessID is this harness's stable identity, from the shared catalog.
const HarnessID = commonharness.HarnessIDClaudeCode

// SettingsRelPath is the project settings file; no descriptor carries it.
const SettingsRelPath = ".claude/settings.json"

// This harness's project-scoped directories, read from the shared descriptor.
var (
	entry        = mustEntry()
	HooksRelDir  = entry.HooksDir
	AgentsRelDir = entry.AgentsDir
)

// mustEntry returns this harness's catalog entry. The builtin descriptor for
// this harness always exists with its directories set; a missing entry or an
// empty directory is a build defect.
func mustEntry() commonharness.CLIHarness {
	e, ok := commonharness.LookupCLIHarness(HarnessID)
	if !ok {
		panic("claudecode: harness " + HarnessID + " missing from the shared catalog")
	}
	if e.AgentsDir == "" {
		panic("claudecode: harness " + HarnessID + " has no agents directory in the shared catalog")
	}
	if e.HooksDir == "" {
		panic("claudecode: harness " + HarnessID + " has no hooks directory in the shared catalog")
	}
	return e
}

// ConfigHomeEnvVar is the environment variable this harness honours to
// relocate its user-scope configuration. Setting it into the spawn plan's
// environment is what lets the adapter report the user scope as neutralized
// rather than merely inspected.
const ConfigHomeEnvVar = "CLAUDE_CONFIG_DIR"

// RunIDEnvVar is the environment variable injected into the subject's process
// so the MOSAIC logger bundle can attribute the session's events to the
// correct run bucket without waiting for a protocol-compliant dispatch to
// extract a run id from the opening prompt.
const RunIDEnvVar = "MOSAIC_RUN_ID"
