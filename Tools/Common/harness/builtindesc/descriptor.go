package builtindesc

// Loading-mechanism wire values (YAML cli.loading_mechanism).
const (
	LoadingMechanismName         = "name"
	LoadingMechanismRunnerLoaded = "runner-loaded"
)

// Descriptor is Common's validated view of one descriptor: only the fields
// more than one tool needs.
type Descriptor struct {
	ID             string
	DisplayName    string
	AgentsDir      string  // paths.agents.project
	SkillsDir      string  // paths.skills.project; "" when skills unsupported
	HooksDir       string  // paths.hooks.project; "" when hooks unsupported
	AgentExtension string  // extensions.agent, leading dot
	AgentFormatID  string  // resolved: "markdown" when agent_format_id absent
	Models         *Models // nil when no models block
	CLI            *CLI    // nil when not CLI-backed
}

// Models is the descriptor's model data.
type Models struct {
	IDs        []string // declared order
	FormatHint string
}

// CLI is the descriptor's CLI-only facts; its presence means CLI-backed.
type CLI struct {
	Label            string
	LoadingMechanism string // LoadingMechanismName or LoadingMechanismRunnerLoaded
}

// ValidationError reports a descriptor rule violation.
type ValidationError struct {
	Origin  string // e.g. "builtin:codex" or a testdata path
	Field   string // dotted YAML path, e.g. "cli.loading_mechanism"
	Message string
}

// Error returns "<Origin>: <Field>: <Message>".
func (e *ValidationError) Error() string {
	return e.Origin + ": " + e.Field + ": " + e.Message
}
