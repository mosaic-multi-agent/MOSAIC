package deploy

// record.go is a test fixture: the deploy package blank-importing a translator
// sub-package of internal/agentformat. The guard must reject every package under
// internal/agentformat, not only the parent. The fixture is never compiled against the
// real module.

import (
	_ "mosaic-deploy/internal/agentformat/codextoml"
)
