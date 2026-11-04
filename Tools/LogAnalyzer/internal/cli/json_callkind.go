package cli

import "mosaic-log-analyzer/internal/domain"

// jsonCallsByKind counts an actor's calls by kind; all three keys are always
// present.
type jsonCallsByKind struct {
	Main       int `json:"main"`
	Compaction int `json:"compaction"`
	Auxiliary  int `json:"auxiliary"`
}

// countCallsByKind counts every call; a call without a kind is a main call.
func countCallsByKind(calls []domain.ContextCall) jsonCallsByKind {
	var out jsonCallsByKind
	for _, c := range calls {
		switch c.Kind {
		case domain.CallCompaction:
			out.Compaction++
		case domain.CallAuxiliary:
			out.Auxiliary++
		default:
			out.Main++
		}
	}
	return out
}
