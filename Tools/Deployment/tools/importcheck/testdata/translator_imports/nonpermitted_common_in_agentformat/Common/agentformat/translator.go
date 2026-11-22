package agentformat

// translator.go is a test fixture: the translator layer imports a mosaic-common
// package outside its permitted set (agentfields, docformat, mosaic, agentformat).
// It uses no I/O and no mosaic-deploy package, so only the mosaic-common
// restriction can reject it.

import (
	_ "mosaic-common/agentload"
)
