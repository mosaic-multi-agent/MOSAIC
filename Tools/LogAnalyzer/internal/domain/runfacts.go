package domain

import "time"

// OptionalBool is a logged boolean that may be absent. Absent never reads as false.
type OptionalBool uint8

const (
	BoolAbsent OptionalBool = iota
	BoolFalse
	BoolTrue
)

// BoolOf returns BoolTrue or BoolFalse for b.
func BoolOf(b bool) OptionalBool {
	if b {
		return BoolTrue
	}
	return BoolFalse
}

// Value returns the boolean and whether it is present; (false, false) when absent.
func (o OptionalBool) Value() (value, present bool) {
	return o == BoolTrue, o != BoolAbsent
}

// IsPresent reports whether the boolean was logged.
func (o OptionalBool) IsPresent() bool {
	return o != BoolAbsent
}

// RunFacts are the run-level facts. A fact the logs did not carry is "" / zero / nil.
type RunFacts struct {
	Harnesses            []Harness  // distinct non-empty envelope harness values on any event of the run, sorted; verbatim
	SchemaVersions       []string   // distinct non-empty schema_version values on any event of the run, sorted; verbatim
	AdapterVersion       string     // last run_start that carried one
	OrchestratorModel    ModelID    // last run_start that carried one
	OrchestratorProvider ProviderID // last run_start that carried one
	StartedAt            time.Time  // earliest parseable run_start timestamp; zero when none
	EndedAt              time.Time  // latest parseable run_end timestamp; zero when none
	Outcome              string     // last run_end that carried one
}

// RunSession is one harness session seen in the run, one entry per distinct session id.
type RunSession struct {
	ID        string       // envelope session_id
	StartedAt time.Time    // first session_start; zero when none or unparseable
	EndedAt   time.Time    // last session_end; zero when none or unparseable
	Resumed   OptionalBool // last session_start that carried it
	Reason    string       // last session_end that carried one
	Model     ModelID      // last session_start that carried one
	Provider  ProviderID   // last session_start that carried one
}

// Turn is one turn event; content is never kept.
type Turn struct {
	Actor ActorRef  // Orchestrator(), or AgentInstance(stream instance hint) with Type ""
	Role  TurnRole  // verbatim; "" when absent
	At    time.Time // zero when unparseable
}

// Notification is one notification event; the message is never kept.
type Notification struct {
	Actor            ActorRef
	At               time.Time
	Type             string // notification_type
	RequiresResponse OptionalBool
	Resolution       string
	ID               string // notification_id
}
