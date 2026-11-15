package all

// all.go is a test fixture: the wiring package imports a mosaic-common package
// outside the agentformat sub-packages. The wiring package may only import
// translator sub-packages of the layer.

import (
	_ "mosaic-common/docformat"
)
