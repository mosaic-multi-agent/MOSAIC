package deploy

// record.go is a test fixture: the deploy package importing the format-id package, which
// lives under internal/agentformat. Deploy is format-blind, so the guard must reject this
// import. The fixture is never compiled against the real module.

import (
	"mosaic-deploy/internal/agentformat/formatid"
)

var _ = formatid.All
