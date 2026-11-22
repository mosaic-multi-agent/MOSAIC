package formatid

// formatid.go is a test fixture: the leaf vocabulary package imports a
// mosaic-common package that is otherwise permitted for the translator layer.
// formatid must import nothing from mosaic-common at all.

import (
	_ "mosaic-common/agentfields"
)
