package agentformat

// ArtifactKind classifies the artifact a translator encodes or decodes. The
// values equal those of the deployment tool's own artifact kind, so callers
// convert with a plain type conversion.
type ArtifactKind string

const (
	ArtifactAgent ArtifactKind = "agent"
	ArtifactSkill ArtifactKind = "skill"
	ArtifactHook  ArtifactKind = "hook"
)
