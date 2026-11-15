package all

// all.go is a test fixture: the wiring package blank-imports translator
// sub-packages of the layer only, which the rule must accept.

import (
	_ "mosaic-common/agentformat/codextoml"
	_ "mosaic-common/agentformat/testjson"
)
