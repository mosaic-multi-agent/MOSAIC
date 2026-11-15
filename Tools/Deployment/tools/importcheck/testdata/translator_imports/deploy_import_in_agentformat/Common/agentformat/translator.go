package agentformat

// translator.go is a test fixture representing a forbidden import of a
// mosaic-deploy package from the shared translator layer. The layer lives in
// mosaic-common and must not depend on any Deploy package, so the guard must
// reject this file.

import (
	"mosaic-deploy/internal/domain"
)

// This file is a fixture and is never compiled against the real module.
var _ = domain.ArtifactKindAgent
