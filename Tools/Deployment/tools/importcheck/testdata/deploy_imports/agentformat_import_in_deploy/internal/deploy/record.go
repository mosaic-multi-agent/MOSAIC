package deploy

// record.go is a test fixture: the deploy package importing the agent-format package.
// Deploy is format-blind, so the guard must reject this import. The fixture is never
// compiled against the real module.

import (
	"mosaic-deploy/internal/agentformat"
)

var _ = agentformat.LookupString
