package harness

// ModelCatalog is one harness's model data: the identifiers it accepts, and
// how to describe their shape to a person. A harness having a ModelCatalog
// does not make it CLI-backed or spawnable.
type ModelCatalog struct {
	// HarnessID is the harness identity this catalog belongs to: a
	// CLIHarness.ID for CLI-backed harnesses, or another builtin harness ID
	// (e.g. "vscode-ghcp") for non-CLI entries.
	HarnessID string

	// IDs are the valid model identifiers, in stable declared order — the
	// order a selection UI presents and a usage text lists.
	IDs []string

	// FormatHint is human-readable guidance on the identifier shape, e.g.
	// "provider/model-id". Display only: nothing validates against it, and
	// nothing switches on it.
	FormatHint string
}

// copyIDs returns a fresh copy of ids so a caller mutating the returned slice
// cannot corrupt the catalog's own backing array.
func copyIDs(ids []string) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// ModelCatalogs returns every catalog entry as fresh values the caller may not
// corrupt for the next caller. Entries come from the builtin descriptors, in
// builtin harness order: claude-code, opencode, ghcp-cli, vscode-ghcp, codex.
// Membership does not make a harness spawnable;
// callers needing spawnable harnesses filter by IsCLIHarness or their own
// predicate.
func ModelCatalogs() []ModelCatalog {
	out := make([]ModelCatalog, len(modelCatalogs))
	for i, e := range modelCatalogs {
		out[i] = ModelCatalog{
			HarnessID:  e.HarnessID,
			FormatHint: e.FormatHint,
			IDs:        copyIDs(e.IDs),
		}
	}
	return out
}

// LookupModelCatalog returns the model catalog for harnessID, CLI-backed or
// not. The second result distinguishes "no catalog entry" from an entry; it
// never panics. The returned IDs slice is a fresh copy.
func LookupModelCatalog(harnessID string) (ModelCatalog, bool) {
	for _, e := range modelCatalogs {
		if e.HarnessID == harnessID {
			return ModelCatalog{
				HarnessID:  e.HarnessID,
				FormatHint: e.FormatHint,
				IDs:        copyIDs(e.IDs),
			}, true
		}
	}
	return ModelCatalog{}, false
}

// ModelIDs returns harnessID's valid model identifiers as a fresh slice, or
// nil when the catalog has no entry for harnessID. It is the convenience
// form for a caller that only needs the list.
func ModelIDs(harnessID string) []string {
	cat, ok := LookupModelCatalog(harnessID)
	if !ok {
		return nil
	}
	return cat.IDs
}

// IsModelID reports whether modelID is valid for harnessID. It is false for
// an unknown harness, for an unknown model, and for a model that is valid for
// a different harness.
func IsModelID(harnessID, modelID string) bool {
	cat, ok := LookupModelCatalog(harnessID)
	if !ok {
		return false
	}
	for _, id := range cat.IDs {
		if id == modelID {
			return true
		}
	}
	return false
}
