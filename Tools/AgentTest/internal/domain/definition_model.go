package domain

import "errors"

// DefinitionModelReader is an optional HarnessAdapter capability: an adapter that
// can read its harness's model field from a deployed agent definition implements it.
// Callers type-assert for it. The field's location and syntax are harness knowledge
// and stay inside internal/harness/.
type DefinitionModelReader interface {
	// DefinitionModel reads the model field of the definition at path (absolute).
	// Value is the parsed scalar (quotes removed, no other normalisation).
	// err is non-nil only when the file cannot be read or its frontmatter cannot be parsed.
	DefinitionModel(path string) (DefinitionModelField, error)
}

// DefinitionModelField is the model field of one deployed agent definition.
type DefinitionModelField struct {
	Present bool   // the model key exists in the definition
	Value   string // its parsed value; "" when absent or empty
}

// ErrNoSubjectModel is wrapped by a real adapter's SpawnPlan when the subject
// carries no model to launch with.
var ErrNoSubjectModel = errors.New("subject has no model to launch with")
