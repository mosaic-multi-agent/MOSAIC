// Package opencodev2 is the OpenCode V2 harness adapter: identity, spawn
// planning, envelope decoding, configuration-scope inspection and
// provisioning. It sits beside the V1 opencode package and imports nothing
// from it.
package opencodev2

import (
	"path/filepath"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/domain"
)

// HarnessID is this harness's stable identity, from the shared catalog.
const HarnessID = commonharness.HarnessIDOpenCodeV2

// PluginFileName is the interceptor plugin's file name in the plugins directory.
const PluginFileName = "mosaic-interceptor.ts"

// CLIExecutable names the product CLI this adapter spawns.
const CLIExecutable = "opencode"

// ConfigHomeEnvVar is the variable V2 resolves its user configuration
// directory from; the spawn plan sets it to relocate user scopes into the run.
const ConfigHomeEnvVar = "XDG_CONFIG_HOME"

// ConfigHomeRelDir is the relocated configuration home, relative to the
// sandbox control directory (not the subject directory, so the subject cannot
// discover it).
const ConfigHomeRelDir = "opencode-v2-home"

// ConfigFileName is the project config file name.
const ConfigFileName = "opencode.json"

// This harness's project-scoped directories, read from the shared catalog.
var (
	entry         = mustEntry()
	AgentsRelDir  = entry.AgentsDir
	PluginsRelDir = entry.HooksDir
)

// mustEntry returns this harness's catalog entry; a missing entry or empty
// directory is a build defect.
func mustEntry() commonharness.CLIHarness {
	e, ok := commonharness.LookupCLIHarness(HarnessID)
	if !ok {
		panic("opencode-v2: harness " + HarnessID + " missing from the shared catalog")
	}
	if e.AgentsDir == "" || e.HooksDir == "" {
		panic("opencode-v2: harness " + HarnessID + " has no agents or plugins directory in the shared catalog")
	}
	return e
}

// relocatedConfigHome is the XDG_CONFIG_HOME value of a run: V2 appends its own
// "opencode" segment to it.
func relocatedConfigHome(s domain.Sandbox) string {
	return filepath.Join(s.ControlDir, ConfigHomeRelDir)
}

// UserConfigDir is the user-scope configuration directory of sandbox s, inside
// the relocated config home. Pure path derivation.
func UserConfigDir(s domain.Sandbox) string {
	return filepath.Join(relocatedConfigHome(s), "opencode")
}
