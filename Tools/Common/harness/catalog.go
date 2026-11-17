package harness

import "fmt"

// Harness identities. These constants are the identity half of the catalog
// entries below; they exist as named constants so a composition root's
// switch cases and a test's expectations name the same thing.
const (
	HarnessIDClaudeCode = "claude-code"
	HarnessIDOpenCode   = "opencode"
	HarnessIDGHCPCLI    = "ghcp-cli"
	HarnessIDCodex      = "codex"
)

// LoadingMechanism describes how a CLI harness locates agent definition files.
type LoadingMechanism int

const (
	// LoadingMechanismUnset is the zero value, indicating the field was never
	// assigned. A catalog entry with this value is misconfigured; the Runner
	// refuses the run rather than silently misbehaving.
	LoadingMechanismUnset LoadingMechanism = 0

	// LoadingMechanismName means the harness resolves agents by name from the
	// agents directory. Runner uses the backup-and-transform strategy, which
	// does nothing for a harness with no transformation rules.
	LoadingMechanismName LoadingMechanism = 2

	// LoadingMechanismRunnerLoaded means the Runner loads the agent file
	// itself on every invocation; no snapshot is taken for the harness.
	LoadingMechanismRunnerLoaded LoadingMechanism = 3
)

// String returns a human-readable label for the loading mechanism.
// Returns "unset", "name" or "runner-loaded" for defined constants, or "unknown(<N>)" for
// unrecognized values.
func (m LoadingMechanism) String() string {
	switch m {
	case LoadingMechanismUnset:
		return "unset"
	case LoadingMechanismName:
		return "name"
	case LoadingMechanismRunnerLoaded:
		return "runner-loaded"
	default:
		return fmt.Sprintf("unknown(%d)", int(m))
	}
}

// CLIHarness is one CLI-backed harness this module can spawn through: its
// stable identity and how to name it to a person.
type CLIHarness struct {
	// ID is the stable identity a --harness flag accepts and a composition
	// root switches on. It never changes once shipped: it appears in scripts
	// and in stored run configuration.
	ID string

	// Label is the human-readable name a selection UI or a help text shows.
	// It is presentation only; nothing ever switches on it.
	Label string

	// AgentsDir is the harness-convention relative path (from the project
	// root) to the directory that holds agent definition files. For example,
	// ".claude/agents" for claude-code or ".opencode/agents" for opencode.
	// Runner locates the deployed agents directory from this value.
	AgentsDir string

	// LoadingMechanism describes how this harness locates agent definition
	// files, from the descriptor's cli.loading_mechanism. Name-based harnesses
	// resolve agents by name; the Runner passes the agent name, not a file path.
	LoadingMechanism LoadingMechanism

	// AgentFormatID is the formatid value of the harness's agent files
	// ("markdown", "codex-toml"), from the descriptor's agent_format_id
	// (absent means "markdown").
	AgentFormatID string

	// AgentExtension is the agent file extension including the leading dot
	// (".md", ".agent.md", ".toml"), from the descriptor's extensions.agent.
	AgentExtension string

	// SkillsDir is the project-relative skills directory (paths.skills.project).
	// Empty means the harness does not support skills.
	SkillsDir string

	// HooksDir is the project-relative hooks or plugins directory
	// (paths.hooks.project). Empty means the harness does not support hooks.
	HooksDir string
}

// cliHarnesses and modelCatalogs are derived once, at package init, from the
// builtin harness descriptors embedded in the builtindesc package. A
// descriptor is the only place a harness's directories, extension, format ID,
// label, loading mechanism and model data are written down.
//
// A descriptor with a "cli" section is CLI-backed and becomes a CLIHarness;
// descriptors without one (e.g. "vscode-ghcp") still contribute model data.
// Adding a CLI harness here therefore needs only its descriptor; each consumer
// additionally maps the identity to a concrete adapter in its own composition
// root, and a coverage test in each consumer fails when that mapping is missed.
//
// The catalog answers "which harnesses exist and what are they called". It
// never answers "how is one constructed": each tool's adapters implement
// that tool's own port, whose types live in that tool's internal packages,
// which this module must not import.
//
// A tool-local test double is not a CLI harness and has no descriptor; a tool
// that accepts one composes it into its own accepted set alongside these.
var cliHarnesses, modelCatalogs = loadCatalogs()

// CLIHarnesses returns every CLI-backed harness this module can spawn
// through, in a stable order, as a fresh slice the caller may not corrupt
// for the next caller.
func CLIHarnesses() []CLIHarness {
	out := make([]CLIHarness, len(cliHarnesses))
	copy(out, cliHarnesses)
	return out
}

// LookupCLIHarness returns the entry for id. The second result distinguishes
// "no such harness" from a zero-valued entry; it never panics.
func LookupCLIHarness(id string) (CLIHarness, bool) {
	for _, e := range cliHarnesses {
		if e.ID == id {
			return e, true
		}
	}
	return CLIHarness{}, false
}

// IsCLIHarness reports whether id names a known CLI-backed harness.
func IsCLIHarness(id string) bool {
	_, ok := LookupCLIHarness(id)
	return ok
}
