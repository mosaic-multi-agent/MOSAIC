// Package opencode is the OpenCode harness adapter: identity, declared
// capabilities, spawn planning and envelope decoding. The interception facet
// (ConfigScopes, InspectScopes, Provision, Deprovision, TranslateCall,
// TranslateOutcome) is present so the type satisfies domain.HarnessAdapter at
// compile time, but is explicitly not implemented yet — a later pass
// replaces the stubs in stub.go with real behaviour.
//
// This file declares the harness's project-scoped layout. The harness identity
// and the agents and plugins directories come from the shared builtin
// descriptor via mosaic-common/harness; only facts no descriptor carries are
// local.
package opencode

import (
	"path/filepath"

	commonharness "mosaic-common/harness"
	"mosaic-agent-test/internal/domain"
)

// HarnessID is this harness's stable identity, from the shared catalog.
const HarnessID = commonharness.HarnessIDOpenCode

// ConfigFileName is the project config file; no descriptor carries it.
const ConfigFileName = "opencode.json"

// This harness's project-scoped directories, read from the shared descriptor.
// The descriptor's hooks directory is this harness's plugins directory.
var (
	entry         = mustEntry()
	AgentsRelDir  = entry.AgentsDir
	PluginsRelDir = entry.HooksDir
)

// mustEntry returns this harness's catalog entry. The builtin descriptor for
// this harness always exists with its directories set; a missing entry or an
// empty directory is a build defect.
func mustEntry() commonharness.CLIHarness {
	e, ok := commonharness.LookupCLIHarness(HarnessID)
	if !ok {
		panic("opencode: harness " + HarnessID + " missing from the shared catalog")
	}
	if e.AgentsDir == "" {
		panic("opencode: harness " + HarnessID + " has no agents directory in the shared catalog")
	}
	if e.HooksDir == "" {
		panic("opencode: harness " + HarnessID + " has no plugins directory in the shared catalog")
	}
	return e
}

// OpenCodeCLIExecutable names the product CLI this adapter spawns as the
// subject-under-test's own process. Resolving it, including the Windows
// .cmd/.bat shim case, is mosaic-common/harness's job at spawn time, not
// this adapter's.
const OpenCodeCLIExecutable = "opencode"

// ConfigHomeEnvVar is the environment variable this harness resolves its
// user-scope configuration directory from. Setting it in the spawn plan
// is what relocates both non-sandbox scopes into the run's sandbox,
// following the pattern the sibling adapter already establishes for its
// own configuration home.
const ConfigHomeEnvVar = "XDG_CONFIG_HOME"

// ConfigHomeRelDir is where the relocated configuration home lives,
// relative to the sandbox's control directory. It is under ControlDir and
// not under SubjectDir so the subject cannot discover it.
const ConfigHomeRelDir = "opencode-home"

// UserConfigDir returns this harness's user-scope configuration directory for
// sandbox s: the opencode subdirectory inside the relocated XDG_CONFIG_HOME
// the spawn plan sets. Setting ConfigHomeEnvVar in the spawn plan to
// filepath.Join(s.ControlDir, ConfigHomeRelDir) causes the spawned opencode
// process to read its user configuration from this directory rather than from
// the real user home.
//
// Pure path derivation; reads no environment and touches no filesystem.
func UserConfigDir(s domain.Sandbox) string {
	return filepath.Join(s.ControlDir, ConfigHomeRelDir, "opencode")
}
