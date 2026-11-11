package domain

// SubjectPlacementReader is an optional HarnessAdapter capability: an adapter
// that launches the subject by its identity, and so resolves the subject
// definition by its `name:` among the project-level agents of the sandbox,
// reports that directory. An adapter that does not implement it (or reports an
// empty directory) resolves the subject some other way and is not checked.
type SubjectPlacementReader interface {
	// SubjectAgentsDir is the project-level agents directory, relative to the
	// sandbox subject directory and slash-separated.
	SubjectAgentsDir() string
}
